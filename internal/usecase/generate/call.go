package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nguyentin05/spectest/internal/domain"
)

// maxLLMInputChars guards against prompts that exceed the model context. The
// pipeline aborts before calling the LLM.
const maxLLMInputChars = 200_000

// defaultRepairs is the number of repair calls most stages get.
const defaultRepairs = 1

// callValidated is callValidatedN with the default number of repairs.
func callValidated[T any](
	ctx context.Context,
	llm domain.LLMService,
	stage, systemPrompt, userPrompt string,
	validate func(T) error,
) (T, error) {
	return callValidatedN(ctx, llm, stage, systemPrompt, userPrompt, defaultRepairs, validate)
}

// callValidatedN calls the LLM with a strict JSON contract and up to
// maxRepairs repair calls when decoding or validation fails.
func callValidatedN[T any](
	ctx context.Context,
	llm domain.LLMService,
	stage, systemPrompt, userPrompt string,
	maxRepairs int,
	validate func(T) error,
) (T, error) {
	return callValidatedLast(ctx, llm, stage, systemPrompt, userPrompt, maxRepairs,
		func(v T, _ bool) error {
			if validate == nil {
				return nil
			}
			return validate(v)
		})
}

// callValidatedLast is the core loop. validate also learns whether this is the
// last attempt, so a stage can be strict while repairs remain and lenient
// (drop and warn) when they are exhausted.
func callValidatedLast[T any](
	ctx context.Context,
	llm domain.LLMService,
	stage, systemPrompt, userPrompt string,
	maxRepairs int,
	validate func(v T, last bool) error,
) (T, error) {
	var zero T
	system, user := systemPrompt, userPrompt
	var failures []error

	for attempt := 0; attempt <= maxRepairs; attempt++ {
		label := stage
		if attempt > 0 {
			label = fmt.Sprintf("%s: repair %d", stage, attempt)
		}
		if err := checkInputLimit(system, user); err != nil {
			return zero, fmt.Errorf("%s: %w", label, err)
		}
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("%s: %w", label, err)
		}

		raw, err := llm.GenerateStructured(ctx, system, user, nil)
		if err != nil {
			return zero, fmt.Errorf("%s: llm call: %w", label, err)
		}

		value, vErr := decodeAndValidate(raw, func(v T) error {
			if validate == nil {
				return nil
			}
			return validate(v, attempt == maxRepairs)
		})
		if vErr == nil {
			return value, nil
		}
		failures = append(failures, vErr)
		system, user = buildRepairPrompt(systemPrompt, userPrompt, raw, vErr)
	}
	return zero, fmt.Errorf("%s: after %d repair(s): %w", stage, maxRepairs, errors.Join(failures...))
}

func checkInputLimit(system, user string) error {
	total := utf8.RuneCountInString(system) + utf8.RuneCountInString(user)
	if total > maxLLMInputChars {
		return fmt.Errorf("input too large: %d chars exceeds limit %d", total, maxLLMInputChars)
	}
	return nil
}

func decodeAndValidate[T any](raw string, validate func(T) error) (T, error) {
	var value T
	if err := decodeStrictJSON([]byte(raw), &value); err != nil {
		return value, fmt.Errorf("decode json: %w", err)
	}
	// "null" decodes into a nil pointer without error; validators must never
	// see it.
	if rv := reflect.ValueOf(value); rv.IsValid() && rv.Kind() == reflect.Pointer && rv.IsNil() {
		return value, errors.New("decode json: response is null")
	}
	if validate != nil {
		if err := validate(value); err != nil {
			return value, fmt.Errorf("validate: %w", err)
		}
	}
	return value, nil
}

// decodeStrictJSON rejects unknown fields and any trailing data.
func decodeStrictJSON(data []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()

	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := normalizeJSONNumbers(reflect.ValueOf(out)); err != nil {
		return fmt.Errorf("normalize JSON numbers: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return errors.New("multiple JSON values in response")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

// normalizeJSONNumbers restores integer types for numbers decoded into dynamic fields.
func normalizeJSONNumbers(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		dynamic := value.Elem()
		if dynamic.Type() == reflect.TypeOf(json.Number("")) {
			normalized, err := parseJSONNumber(dynamic.Interface().(json.Number))
			if err != nil {
				return err
			}
			value.Set(reflect.ValueOf(normalized))
			return nil
		}
		normalized := reflect.New(dynamic.Type()).Elem()
		normalized.Set(dynamic)
		if err := normalizeJSONNumbers(normalized); err != nil {
			return err
		}
		value.Set(normalized)
	case reflect.Pointer:
		if !value.IsNil() {
			return normalizeJSONNumbers(value.Elem())
		}
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		normalized := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			item := reflect.New(value.Type().Elem()).Elem()
			item.Set(iter.Value())
			if err := normalizeJSONNumbers(item); err != nil {
				return err
			}
			normalized.SetMapIndex(iter.Key(), item)
		}
		value.Set(normalized)
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := normalizeJSONNumbers(value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.CanSet() {
				if err := normalizeJSONNumbers(field); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func parseJSONNumber(number json.Number) (any, error) {
	raw := number.String()
	if !strings.ContainsAny(raw, ".eE") {
		if parsed, err := strconv.ParseInt(raw, 10, strconv.IntSize); err == nil {
			return int(parsed), nil
		}
		if parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize); err == nil {
			return uint(parsed), nil
		}
		return nil, fmt.Errorf("integer %q is outside the supported range", raw)
	}
	parsed, err := number.Float64()
	if err != nil {
		return nil, fmt.Errorf("number %q is outside the supported range: %w", raw, err)
	}
	return parsed, nil
}

func buildRepairPrompt(system, user, previous string, validationErr error) (string, string) {
	var b strings.Builder
	b.WriteString(user)
	b.WriteString("\n\n<previous_response>\n")
	b.WriteString(previous)
	b.WriteString("\n</previous_response>\n\n<validation_errors>\n")
	b.WriteString(validationErr.Error())
	b.WriteString("\n</validation_errors>\n\n")
	b.WriteString("Your previous response was invalid. Return a corrected JSON value " +
		"that satisfies the required schema and the validation errors above. " +
		"Return JSON only.")
	return system, b.String()
}
