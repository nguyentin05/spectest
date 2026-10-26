package run

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

var varRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_\-]+)\}\}`)

// Scope manages runtime variables, interpolation, and save extractions.
type Scope struct {
	vars map[string]any
}

// NewScope creates an empty runtime variable scope.
func NewScope() *Scope {
	return &Scope{
		vars: make(map[string]any),
	}
}

// NewScopeFromSeed initializes a scope from a Suite's seed definitions.
// It enforces the specification rule: no duplicate variable names across seeds.
func NewScopeFromSeed(seed map[string]domain.Seed) (*Scope, error) {
	s := NewScope()
	if err := s.LoadSeed(seed); err != nil {
		return nil, err
	}
	return s, nil
}

// LoadSeed loads seed variables into the scope, checking for name collisions across different seeds.
func (s *Scope) LoadSeed(seed map[string]domain.Seed) error {
	seenKeys := make(map[string]string) // var_name -> seed_name
	for seedName, seedDef := range seed {
		for k, v := range seedDef.Vars {
			if prevSeed, exists := seenKeys[k]; exists {
				return fmt.Errorf("seed variable collision: %q is defined in both %q and %q", k, prevSeed, seedName)
			}
			seenKeys[k] = seedName
			s.vars[k] = v
		}
	}
	return nil
}

// Clone creates an isolated copy of the scope for per-scenario execution isolation.
func (s *Scope) Clone() *Scope {
	copied := make(map[string]any, len(s.vars))
	for k, v := range s.vars {
		copied[k] = v
	}
	return &Scope{
		vars: copied,
	}
}

// Set stores a variable into the scope.
func (s *Scope) Set(key string, val any) {
	s.vars[key] = val
}

// MergeSetupVars merges local setup variables into the scope, interpolating any {{var}} references.
func (s *Scope) MergeSetupVars(vars map[string]any) {
	for k, v := range vars {
		s.vars[k] = s.InterpolateBody(v)
	}
}

// Get retrieves a variable value from the scope.
func (s *Scope) Get(key string) (any, bool) {
	val, ok := s.vars[key]
	return val, ok
}

// All returns a shallow copy of all variables in the scope.
func (s *Scope) All() map[string]any {
	res := make(map[string]any, len(s.vars))
	for k, v := range s.vars {
		res[k] = v
	}
	return res
}

// InterpolateString replaces all {{var_name}} occurrences in str.
// The dynamic token {{random}} generates a unique random hex string for each occurrence.
func (s *Scope) InterpolateString(str string) string {
	return varRegex.ReplaceAllStringFunc(str, func(match string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}")
		if varName == "random" {
			return generateRandomToken()
		}
		if val, ok := s.vars[varName]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})
}

// InterpolateBody recursively interpolates values in body payload.
// If a string matches strictly "{{var_name}}" (and is not random), the underlying
// type of the variable (int, bool, float, array, map) is preserved directly.
func (s *Scope) InterpolateBody(body any) any {
	if body == nil {
		return nil
	}

	switch v := body.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") && !strings.Contains(trimmed[2:len(trimmed)-2], "{{") {
			varName := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			if varName == "random" {
				return generateRandomToken()
			}
			if rawVal, ok := s.vars[varName]; ok {
				return rawVal
			}
		}
		return s.InterpolateString(v)

	case map[string]any:
		res := make(map[string]any, len(v))
		for k, val := range v {
			res[k] = s.InterpolateBody(val)
		}
		return res

	case []any:
		res := make([]any, len(v))
		for i, val := range v {
			res[i] = s.InterpolateBody(val)
		}
		return res

	default:
		return body
	}
}

// InterpolateHeaders interpolates variables in header values.
func (s *Scope) InterpolateHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return make(map[string]string)
	}
	res := make(map[string]string, len(headers))
	for k, v := range headers {
		res[k] = s.InterpolateString(v)
	}
	return res
}

// ExtractSave extracts variables from the response body and headers according to save mappings,
// storing the extracted variables into the scope and returning the extracted map.
// Supported mapping types:
// - "body.<dot-path>": JSON body extraction (supports nested keys and numeric array indexes).
// - "header.<name>": Case-insensitive HTTP response header lookup.
func (s *Scope) ExtractSave(save map[string]string, respBody []byte, respHeaders http.Header) (map[string]any, error) {
	if len(save) == 0 {
		return nil, nil
	}

	var parsedBody any
	bodyParsed := false

	extracted := make(map[string]any, len(save))
	for varName, sourcePath := range save {
		sourcePath = strings.TrimSpace(sourcePath)
		if strings.HasPrefix(sourcePath, "header.") {
			headerName := strings.TrimPrefix(sourcePath, "header.")
			val := respHeaders.Get(headerName)
			s.vars[varName] = val
			extracted[varName] = val
			continue
		}

		if strings.HasPrefix(sourcePath, "body.") || sourcePath == "body" {
			if !bodyParsed {
				if len(respBody) > 0 {
					if err := json.Unmarshal(respBody, &parsedBody); err != nil {
						return extracted, fmt.Errorf("parse response body as JSON for extraction %q: %w", sourcePath, err)
					}
				}
				bodyParsed = true
			}

			if sourcePath == "body" {
				s.vars[varName] = parsedBody
				extracted[varName] = parsedBody
				continue
			}

			dotPath := strings.TrimPrefix(sourcePath, "body.")
			val, err := extractJSONPath(parsedBody, dotPath)
			if err != nil {
				return extracted, fmt.Errorf("extract %q from response body: %w", sourcePath, err)
			}
			s.vars[varName] = val
			extracted[varName] = val
			continue
		}

		return extracted, fmt.Errorf("unsupported save expression %q (must start with 'body.' or 'header.')", sourcePath)
	}

	return extracted, nil
}

// extractJSONPath traverses nested maps and slices using dot notation (e.g., "data.items.0.id").
func extractJSONPath(data any, path string) (any, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("cannot navigate %q on nil value", part)
		}

		switch node := current.(type) {
		case map[string]any:
			val, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found in map", part)
			}
			current = val

		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("index %q is not a valid slice index: %w", part, err)
			}
			if idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("index %d out of bounds (len %d)", idx, len(node))
			}
			current = node[idx]

		default:
			return nil, fmt.Errorf("cannot navigate key %q on scalar type %T", part, current)
		}
	}

	return current, nil
}

func generateRandomToken() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "rnd1234"
	}
	return hex.EncodeToString(b)
}
