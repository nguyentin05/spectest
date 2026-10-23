// Package openapi provides deterministic preprocessing for OpenAPI 3.x and Swagger 2.0
// specifications, including schema pruning, local $ref resolution, authentication topology
// detection, and domain tag chunking.
package openapi

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// OpenAPISpec represents the subset of an OpenAPI 3.x / Swagger 2.0 document required
// for endpoint normalization, schema resolution, and security analysis.
type OpenAPISpec struct {
	OpenAPI string `yaml:"openapi" json:"openapi"`
	Info    struct {
		Title       string `yaml:"title" json:"title"`
		Description string `yaml:"description,omitempty" json:"description,omitempty"`
		Version     string `yaml:"version" json:"version"`
	} `yaml:"info" json:"info"`
	Servers []struct {
		URL string `yaml:"url" json:"url"`
	} `yaml:"servers,omitempty" json:"servers,omitempty"`
	Security   []map[string][]string       `yaml:"security,omitempty" json:"security,omitempty"`
	Paths      map[string]map[string]RawOp `yaml:"paths" json:"paths"`
	Components struct {
		Schemas         map[string]SchemaObject         `yaml:"schemas,omitempty" json:"schemas,omitempty"`
		SecuritySchemes map[string]SecuritySchemeObject `yaml:"securitySchemes,omitempty" json:"securitySchemes,omitempty"`
	} `yaml:"components,omitempty" json:"components,omitempty"`
}

// RawOp represents a raw HTTP method operation defined under an OpenAPI path item.
type RawOp struct {
	OperationID string                    `yaml:"operationId,omitempty" json:"operationId,omitempty"`
	Summary     string                    `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description string                    `yaml:"description,omitempty" json:"description,omitempty"`
	Tags        []string                  `yaml:"tags,omitempty" json:"tags,omitempty"`
	Security    []map[string][]string     `yaml:"security,omitempty" json:"security,omitempty"`
	Parameters  []ParameterObject         `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	RequestBody *RequestBodyObject        `yaml:"requestBody,omitempty" json:"requestBody,omitempty"`
	Responses   map[string]ResponseObject `yaml:"responses,omitempty" json:"responses,omitempty"`
}

// ParameterObject represents a path, query, or header parameter in an OpenAPI operation.
type ParameterObject struct {
	Name     string       `yaml:"name" json:"name"`
	In       string       `yaml:"in" json:"in"`
	Required bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Schema   SchemaObject `yaml:"schema,omitempty" json:"schema,omitempty"`
}

// RequestBodyObject represents the request body definition of an OpenAPI operation.
type RequestBodyObject struct {
	Required bool                       `yaml:"required,omitempty" json:"required,omitempty"`
	Content  map[string]MediaTypeObject `yaml:"content,omitempty" json:"content,omitempty"`
}

// ResponseObject represents a status-code response definition of an OpenAPI operation.
type ResponseObject struct {
	Description string                     `yaml:"description,omitempty" json:"description,omitempty"`
	Content     map[string]MediaTypeObject `yaml:"content,omitempty" json:"content,omitempty"`
}

// MediaTypeObject holds the JSON Schema associated with a MIME type (e.g. `application/json`).
type MediaTypeObject struct {
	Schema SchemaObject `yaml:"schema,omitempty" json:"schema,omitempty"`
}

// SchemaObject represents a JSON Schema definition within an OpenAPI document,
// supporting recursive `$ref` resolution, nested object properties, array items,
// and numeric/enum constraints.
type SchemaObject struct {
	Ref        string                  `yaml:"$ref,omitempty" json:"$ref,omitempty"`
	Type       string                  `yaml:"type,omitempty" json:"type,omitempty"`
	Format     string                  `yaml:"format,omitempty" json:"format,omitempty"`
	Properties map[string]SchemaObject `yaml:"properties,omitempty" json:"properties,omitempty"`
	Items      *SchemaObject           `yaml:"items,omitempty" json:"items,omitempty"`
	Required   []string                `yaml:"required,omitempty" json:"required,omitempty"`
	Enum       []any                   `yaml:"enum,omitempty" json:"enum,omitempty"`
	Minimum    *float64                `yaml:"minimum,omitempty" json:"minimum,omitempty"`
	Maximum    *float64                `yaml:"maximum,omitempty" json:"maximum,omitempty"`
	Example    any                     `yaml:"example,omitempty" json:"example,omitempty"`
}

// SecuritySchemeObject represents an authentication scheme declared under `components.securitySchemes`.
type SecuritySchemeObject struct {
	Type         string `yaml:"type" json:"type"`
	Scheme       string `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	BearerFormat string `yaml:"bearerFormat,omitempty" json:"bearerFormat,omitempty"`
	Name         string `yaml:"name,omitempty" json:"name,omitempty"`
	In           string `yaml:"in,omitempty" json:"in,omitempty"`
}

// NormalizedOperation is a flattened, self-contained representation of a single API endpoint
// with all `$ref` schemas inlined and authentication flags resolved, ready for LLM prompting.
type NormalizedOperation struct {
	Method          string                  `yaml:"method" json:"method"`
	Endpoint        string                  `yaml:"endpoint" json:"endpoint"`
	OperationID     string                  `yaml:"operation_id,omitempty" json:"operation_id,omitempty"`
	Summary         string                  `yaml:"summary,omitempty" json:"summary,omitempty"`
	Tags            []string                `yaml:"tags,omitempty" json:"tags,omitempty"`
	RequiresAuth    bool                    `yaml:"requires_auth" json:"requires_auth"`
	IsTokenProducer bool                    `yaml:"is_token_producer" json:"is_token_producer"`
	RequestSchema   *SchemaObject           `yaml:"request_schema,omitempty" json:"request_schema,omitempty"`
	ResponseSchemas map[string]SchemaObject `yaml:"response_schemas,omitempty" json:"response_schemas,omitempty"`
}

// PruneResult holds the normalized operations extracted from an OpenAPI specification
// alongside byte-size telemetry used to report context compression savings.
type PruneResult struct {
	Title            string
	Version          string
	BaseURL          string
	Operations       []NormalizedOperation
	OriginalBytes    int
	PrunedBytes      int
	CompressionRatio float64
}

// ParseRawSpec unmarshals raw YAML or JSON bytes into an OpenAPISpec structure
// and validates that at least one API path is defined.
func ParseRawSpec(data []byte) (*OpenAPISpec, error) {
	var spec OpenAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	if len(spec.Paths) == 0 {
		return nil, fmt.Errorf("invalid openapi spec: no paths defined")
	}
	return &spec, nil
}

// PruneSpec parses an OpenAPI document, strips verbose descriptions and unused component
// schemas, inlines local `#/components/schemas/` references up to a bounded recursion depth,
// annotates operations with zero-token authentication flags, and computes the compression ratio.
func PruneSpec(data []byte) (*PruneResult, error) {
	spec, err := ParseRawSpec(data)
	if err != nil {
		return nil, err
	}

	baseURL := ""
	if len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		baseURL = spec.Servers[0].URL
	}

	var paths []string
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var methodOrder = [5]string{"post", "get", "put", "patch", "delete"}
	var ops []NormalizedOperation

	for _, path := range paths {
		methodsMap := spec.Paths[path]
		for _, m := range methodOrder {
			rawOp, exists := methodsMap[m]
			if !exists {
				rawOp, exists = methodsMap[strings.ToUpper(m)]
				if !exists {
					continue
				}
			}

			normOp := NormalizedOperation{
				Method:          strings.ToUpper(m),
				Endpoint:        path,
				OperationID:     rawOp.OperationID,
				Summary:         truncateSummary(rawOp.Summary, rawOp.Description),
				Tags:            rawOp.Tags,
				ResponseSchemas: make(map[string]SchemaObject),
			}

			if normOp.OperationID == "" {
				normOp.OperationID = synthesizeOperationID(normOp.Method, path)
			}

			if rawOp.RequestBody != nil {
				if media, ok := rawOp.RequestBody.Content["application/json"]; ok {
					resolved := resolveSchema(media.Schema, spec.Components.Schemas, 0)
					normOp.RequestSchema = &resolved
				}
			}

			for code, resp := range rawOp.Responses {
				if media, ok := resp.Content["application/json"]; ok {
					resolved := resolveSchema(media.Schema, spec.Components.Schemas, 0)
					normOp.ResponseSchemas[code] = resolved
				} else {
					normOp.ResponseSchemas[code] = SchemaObject{Type: "object"}
				}
			}

			ops = append(ops, normOp)
		}
	}

	ApplyAuthDetection(spec, ops)

	prunedData, _ := yaml.Marshal(ops)
	origLen := len(data)
	prunedLen := len(prunedData)
	ratio := 0.0
	if origLen > 0 && prunedLen < origLen {
		ratio = float64(origLen-prunedLen) / float64(origLen) * 100.0
	}

	return &PruneResult{
		Title:            spec.Info.Title,
		Version:          spec.Info.Version,
		BaseURL:          baseURL,
		Operations:       ops,
		OriginalBytes:    origLen,
		PrunedBytes:      prunedLen,
		CompressionRatio: ratio,
	}, nil
}

// SynthesizeSamplePayload constructs a baseline JSON object map from a resolved SchemaObject
// by inspecting property examples, enum values, minimum constraints, or primitive types.
func SynthesizeSamplePayload(schema SchemaObject) map[string]any {
	payload := make(map[string]any)
	for propName, prop := range schema.Properties {
		payload[propName] = synthesizePropertyValue(propName, prop)
	}
	return payload
}

// synthesizePropertyValue derives a representative value for a single schema property.
func synthesizePropertyValue(name string, prop SchemaObject) any {
	if prop.Example != nil {
		return prop.Example
	}
	if len(prop.Enum) > 0 {
		return prop.Enum[0]
	}

	switch prop.Type {
	case "integer", "number":
		if prop.Minimum != nil && *prop.Minimum > 0 {
			return *prop.Minimum
		}
		return 1
	case "boolean":
		return true
	case "array":
		if prop.Items != nil {
			return []any{synthesizePropertyValue(name+"_item", *prop.Items)}
		}
		return []any{}
	case "object":
		if len(prop.Properties) > 0 {
			return SynthesizeSamplePayload(prop)
		}
		return map[string]any{}
	default:
		if prop.Format == "email" {
			return "user@example.com"
		}
		return "sample_" + name
	}
}

// resolveSchema recursively expands local `#/components/schemas/<Name>` references up to
// depth 5 to produce an inline SchemaObject free of external component pointers.
func resolveSchema(s SchemaObject, components map[string]SchemaObject, depth int) SchemaObject {
	if depth > 5 {
		return s
	}
	if s.Ref != "" {
		const prefix = "#/components/schemas/"
		if strings.HasPrefix(s.Ref, prefix) {
			name := strings.TrimPrefix(s.Ref, prefix)
			if target, ok := components[name]; ok {
				return resolveSchema(target, components, depth+1)
			}
		}
	}
	out := SchemaObject{
		Type:     s.Type,
		Format:   s.Format,
		Required: s.Required,
		Enum:     s.Enum,
		Minimum:  s.Minimum,
		Maximum:  s.Maximum,
		Example:  s.Example,
	}
	if len(s.Properties) > 0 {
		out.Properties = make(map[string]SchemaObject, len(s.Properties))
		for k, prop := range s.Properties {
			out.Properties[k] = resolveSchema(prop, components, depth+1)
		}
	}
	if s.Items != nil {
		resolvedItems := resolveSchema(*s.Items, components, depth+1)
		out.Items = &resolvedItems
	}
	if out.Type == "" && len(out.Properties) > 0 {
		out.Type = "object"
	}
	return out
}

// truncateSummary condenses an operation's summary or description into a single line
// capped at 100 characters to minimize prompt token consumption.
func truncateSummary(summary, desc string) string {
	text := strings.TrimSpace(summary)
	if text == "" {
		text = strings.TrimSpace(desc)
	}
	if idx := strings.Index(text, "\n"); idx != -1 {
		text = text[:idx]
	}
	if len(text) > 100 {
		return text[:97] + "..."
	}
	return text
}

// synthesizeOperationID generates a deterministic snake_case operation identifier from
// the HTTP method and URL path when an explicit `operationId` is absent in the spec.
func synthesizeOperationID(method, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var clean []string
	for _, p := range parts {
		p = strings.Trim(p, "{}")
		p = strings.ReplaceAll(p, "-", "_")
		if p != "" && p != "api" && p != "v1" && p != "v2" {
			clean = append(clean, p)
		}
	}
	return strings.ToLower(method) + "_" + strings.Join(clean, "_")
}
