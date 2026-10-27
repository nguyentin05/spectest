package openapi

import (
	"encoding/json"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nguyentin05/spectest/internal/domain"
)

// normalizedOperation is the intermediate representation used before the
// canonical API hash and compact signature are computed. Security is retained
// here because domain.API has no public security field; the hash covers it.
type normalizedOperation struct {
	Method      string
	Path        string
	Description string
	Parameters  []domain.APIParameter
	Request     *domain.RequestSpec
	Responses   map[string]domain.Schema
	Security    []securityRequirement
	RawSnippet  string
}

// Normalize parses an OpenAPI 3.x document and returns its operations sorted by
// path and then method. The supplied server label is used only in the API key,
// never as a base URL; spec `servers` blocks are ignored.
//
// Every operation is assigned its canonical Hash and compact Sig from the
// normalized operation, including security, which is why the internal
// normalizedOperation retains it.
func Normalize(server string, raw []byte) ([]domain.API, error) {
	operations, err := normalizeDocument(raw)
	if err != nil {
		return nil, err
	}
	apis := make([]domain.API, 0, len(operations))
	for _, operation := range operations {
		apis = append(apis, domain.API{
			Key:         apiKey(server, operation.Method, operation.Path),
			Method:      operation.Method,
			Server:      server,
			Path:        operation.Path,
			Description: operation.Description,
			Hash:        apiHash(operation),
			Sig:         buildSignature(operation),
			Parameters:  operation.Parameters,
			Request:     operation.Request,
			Responses:   operation.Responses,
			RawSnippet:  operation.RawSnippet,
		})
	}
	return apis, nil
}

// normalizeDocument is the internal form of Normalize that keeps security so
// the hash and signature can include it.
func normalizeDocument(raw []byte) ([]normalizedOperation, error) {
	parsed, err := parseSpec(raw)
	if err != nil {
		return nil, err
	}

	operations := make([]normalizedOperation, 0)
	for rawPath, item := range parsed.Paths {
		if item == nil {
			continue
		}
		path := cleanAPIPath(rawPath)
		if path == "" {
			continue
		}
		for _, candidate := range pathItemOperations(item) {
			if candidate.operation == nil {
				continue
			}
			operations = append(operations, buildOperation(path, candidate.method, item.Parameters, candidate.operation, parsed.Security))
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Path != operations[j].Path {
			return operations[i].Path < operations[j].Path
		}
		return operations[i].Method < operations[j].Method
	})
	return operations, nil
}

// apiKey is `METHOD {server}PATH`; braces are literal, matching the suite and
// track file format.
func apiKey(server, method, path string) string {
	return method + " {" + server + "}" + path
}

// cleanAPIPath trims a document path and removes any query or fragment. Path
// parameters such as `{id}` are preserved verbatim.
func cleanAPIPath(path string) string {
	path = strings.TrimSpace(path)
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		path = path[:index]
	}
	return path
}

type methodOperation struct {
	method    string
	operation *operation
}

// pathItemOperations returns every supported HTTP method in a deterministic
// order. Final output is sorted by path then method anyway.
func pathItemOperations(item *pathItem) []methodOperation {
	return []methodOperation{
		{method: "GET", operation: item.Get},
		{method: "PUT", operation: item.Put},
		{method: "POST", operation: item.Post},
		{method: "DELETE", operation: item.Delete},
		{method: "OPTIONS", operation: item.Options},
		{method: "HEAD", operation: item.Head},
		{method: "PATCH", operation: item.Patch},
		{method: "TRACE", operation: item.Trace},
	}
}

func buildOperation(path, method string, pathParameters []*parameter, operation *operation, globalSecurity *[]securityRequirement) normalizedOperation {
	return normalizedOperation{
		Method:      method,
		Path:        path,
		Description: operationDescription(operation.Summary, operation.Description),
		Parameters:  mergeParameters(pathParameters, operation.Parameters),
		Request:     buildRequest(operation.RequestBody),
		Responses:   buildResponses(operation.Responses),
		Security:    effectiveSecurity(operation.Security, globalSecurity),
		RawSnippet:  buildRawSnippet(path, method, pathParameters, operation),
	}
}

func buildRawSnippet(path, method string, pathParameters []*parameter, op *operation) string {
	type paramSnippet struct {
		Name     string `json:"name"`
		In       string `json:"in"`
		Required bool   `json:"required,omitempty"`
		Schema   any    `json:"schema,omitempty"`
	}

	mergedParams := make([]paramSnippet, 0)
	paramMap := make(map[string]paramSnippet)

	for _, p := range pathParameters {
		if p == nil {
			continue
		}
		var s any
		_ = p.Schema.Decode(&s)
		paramMap[p.In+":"+p.Name] = paramSnippet{Name: p.Name, In: p.In, Required: p.Required, Schema: s}
	}

	if op != nil {
		for _, p := range op.Parameters {
			if p == nil {
				continue
			}
			var s any
			_ = p.Schema.Decode(&s)
			paramMap[p.In+":"+p.Name] = paramSnippet{Name: p.Name, In: p.In, Required: p.Required, Schema: s}
		}
	}

	for _, p := range paramMap {
		mergedParams = append(mergedParams, p)
	}
	sort.Slice(mergedParams, func(i, j int) bool {
		return mergedParams[i].Name < mergedParams[j].Name
	})

	var reqBody any
	if op != nil && op.RequestBody != nil {
		contentMap := make(map[string]any)
		for mtName, mt := range op.RequestBody.Content {
			if mt == nil {
				continue
			}
			var s any
			_ = mt.Schema.Decode(&s)
			contentMap[mtName] = map[string]any{"schema": s}
		}
		reqBody = map[string]any{
			"required": op.RequestBody.Required,
			"content":  contentMap,
		}
	}

	resps := make(map[string]any)
	if op != nil {
		for status, r := range op.Responses {
			if r == nil {
				continue
			}
			cMap := make(map[string]any)
			for mtName, mt := range r.Content {
				if mt == nil {
					continue
				}
				var s any
				_ = mt.Schema.Decode(&s)
				cMap[mtName] = map[string]any{"schema": s}
			}
			resps[status] = map[string]any{"content": cMap}
		}
	}

	snippet := map[string]any{
		"method":     method,
		"path":       path,
		"parameters": mergedParams,
		"responses":  resps,
	}
	if reqBody != nil {
		snippet["request_body"] = reqBody
	}
	if op != nil && op.Summary != "" {
		snippet["summary"] = op.Summary
	}

	data, err := json.Marshal(snippet)
	if err != nil {
		return ""
	}
	return string(data)
}

// operationDescription prefers summary and falls back to description. Text is
// collapsed onto one line and truncated to 300 runes.
func operationDescription(summary, description string) string {
	const maxLength = 300
	if value := oneLine(summary, maxLength); value != "" {
		return value
	}
	return oneLine(description, maxLength)
}

func oneLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

// mergeParameters merges path-level parameters with operation-level parameters.
// An operation parameter overrides a path parameter with the same (name, in).
func mergeParameters(pathParameters, operationParameters []*parameter) []domain.APIParameter {
	type parameterKey struct {
		in   string
		name string
	}
	merged := make(map[parameterKey]domain.APIParameter)

	add := func(parameters []*parameter) {
		for _, parameter := range parameters {
			if parameter == nil {
				continue
			}
			name := strings.TrimSpace(parameter.Name)
			in := strings.ToLower(strings.TrimSpace(parameter.In))
			if name == "" || !supportedParameterIn(in) {
				continue
			}
			merged[parameterKey{in: in, name: name}] = domain.APIParameter{
				Name:     name,
				In:       in,
				Required: parameter.Required,
				Schema:   schemaToDomain(&parameter.Schema, make(map[*yaml.Node]bool)),
			}
		}
	}
	add(pathParameters)
	add(operationParameters)

	parameters := make([]domain.APIParameter, 0, len(merged))
	for _, parameter := range merged {
		parameters = append(parameters, parameter)
	}
	sort.Slice(parameters, func(i, j int) bool {
		if parameters[i].In != parameters[j].In {
			return parameters[i].In < parameters[j].In
		}
		return parameters[i].Name < parameters[j].Name
	})
	return parameters
}

func supportedParameterIn(in string) bool {
	switch in {
	case "path", "query", "header", "cookie":
		return true
	default:
		return false
	}
}

// buildRequest returns nil when the operation has no request body. A body with
// no usable media type still yields a non-nil RequestSpec with JSON=false.
func buildRequest(body *requestBody) *domain.RequestSpec {
	if body == nil {
		return nil
	}
	request := &domain.RequestSpec{Required: body.Required}
	if media, ok := selectMedia(body.Content); ok {
		request.MediaType = media.name
		request.JSON = media.name == "application/json"
		request.Schema = schemaToDomain(media.schema, make(map[*yaml.Node]bool))
	}
	return request
}

func buildResponses(responses map[string]*response) map[string]domain.Schema {
	if len(responses) == 0 {
		return nil
	}
	normalized := make(map[string]domain.Schema, len(responses))
	for code, item := range responses {
		var schema domain.Schema
		if item != nil {
			if media, ok := selectMedia(item.Content); ok {
				schema = schemaToDomain(media.schema, make(map[*yaml.Node]bool))
			}
		}
		normalized[code] = schema
	}
	return normalized
}

type selectedMedia struct {
	name   string
	schema *yaml.Node
}

// selectMedia prefers an exact application/json media type, then `*/*`, then
// the lexically first media type. Parameters such as `; charset=utf-8` are
// removed before comparison and are not stored.
func selectMedia(content map[string]*mediaType) (selectedMedia, bool) {
	if len(content) == 0 {
		return selectedMedia{}, false
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	type candidate struct {
		key  string
		base string
	}
	candidates := make([]candidate, 0, len(keys))
	for _, key := range keys {
		base := normalizeMediaType(key)
		if base == "" {
			continue
		}
		candidates = append(candidates, candidate{key: key, base: base})
	}
	if len(candidates) == 0 {
		return selectedMedia{}, false
	}

	chosen := candidates[0]
	for _, candidate := range candidates {
		if candidate.base == "application/json" {
			chosen = candidate
			break
		}
	}
	if chosen.base != "application/json" {
		for _, candidate := range candidates {
			if candidate.base == "*/*" {
				chosen = candidate
				break
			}
		}
	}

	media := content[chosen.key]
	if media == nil {
		return selectedMedia{name: chosen.base}, true
	}
	return selectedMedia{name: chosen.base, schema: &media.Schema}, true
}

func normalizeMediaType(value string) string {
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = value[:index]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// effectiveSecurity applies the OpenAPI override rule: an operation-level
// `security` key, including an explicit empty list, replaces the document-level
// list. The full requirement list is retained so the hash can cover all scopes.
func effectiveSecurity(operationSecurity, globalSecurity *[]securityRequirement) []securityRequirement {
	source := globalSecurity
	if operationSecurity != nil {
		source = operationSecurity
	}
	if source == nil {
		return nil
	}
	requirements := make([]securityRequirement, len(*source))
	copy(requirements, *source)
	return requirements
}

// schemaToDomain converts the representable subset of a JSON Schema node. Any
// construct the shallow domain model cannot express is marked Unsupported
// instead of being guessed. seen guards alias/reference cycles in the node tree.
func schemaToDomain(node *yaml.Node, seen map[*yaml.Node]bool) domain.Schema {
	if node == nil {
		return domain.Schema{}
	}
	if node.Kind == yaml.AliasNode {
		if node.Alias == nil {
			return domain.Schema{Unsupported: true}
		}
		node = node.Alias
	}
	if node.Kind == 0 {
		return domain.Schema{}
	}
	if seen == nil {
		seen = make(map[*yaml.Node]bool)
	}
	if seen[node] {
		return domain.Schema{Unsupported: true}
	}
	seen[node] = true
	defer delete(seen, node)

	if node.Kind != yaml.MappingNode {
		return domain.Schema{Unsupported: true}
	}
	// A remaining $ref is a circular reference that resolveLocalRefs stopped at.
	if mapValue(node, "$ref") != nil {
		return domain.Schema{Unsupported: true}
	}
	for _, key := range []string{
		"allOf", "oneOf", "anyOf", "not",
		"prefixItems", "contains", "propertyNames", "patternProperties",
		"if", "then", "else", "dependentSchemas",
		"unevaluatedProperties", "unevaluatedItems",
	} {
		if mapValue(node, key) != nil {
			return domain.Schema{Unsupported: true}
		}
	}

	var schema domain.Schema
	if typeNode := mapValue(node, "type"); typeNode != nil {
		if typeNode.Kind != yaml.ScalarNode || typeNode.Tag == "!!null" {
			return domain.Schema{Unsupported: true}
		}
		schema.Type = strings.TrimSpace(typeNode.Value)
	}

	if requiredNode := mapValue(node, "required"); requiredNode != nil {
		if requiredNode.Kind != yaml.SequenceNode {
			return domain.Schema{Unsupported: true}
		}
		seenRequired := make(map[string]struct{}, len(requiredNode.Content))
		for _, item := range requiredNode.Content {
			if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
				return domain.Schema{Unsupported: true}
			}
			name := item.Value
			if name == "" {
				continue
			}
			if _, ok := seenRequired[name]; ok {
				continue
			}
			seenRequired[name] = struct{}{}
			schema.Required = append(schema.Required, name)
		}
		sort.Strings(schema.Required)
	}

	if propertiesNode := mapValue(node, "properties"); propertiesNode != nil {
		if propertiesNode.Kind != yaml.MappingNode {
			return domain.Schema{Unsupported: true}
		}
		schema.Properties = make(map[string]domain.Schema, len(propertiesNode.Content)/2)
		for i := 0; i+1 < len(propertiesNode.Content); i += 2 {
			name := propertiesNode.Content[i].Value
			schema.Properties[name] = schemaToDomain(propertiesNode.Content[i+1], seen)
		}
	}

	if itemsNode := mapValue(node, "items"); itemsNode != nil {
		if itemsNode.Kind != yaml.MappingNode {
			return domain.Schema{Unsupported: true}
		}
		items := schemaToDomain(itemsNode, seen)
		schema.Items = &items
	}

	if additionalNode := mapValue(node, "additionalProperties"); additionalNode != nil {
		if additionalNode.Kind != yaml.ScalarNode || additionalNode.Tag != "!!bool" {
			return domain.Schema{Unsupported: true}
		}
		allowed := additionalNode.Value == "true"
		schema.AdditionalProperties = &allowed
	}

	return schema
}
