package openapi

import (
	"regexp"
	"strings"
)

var (
	tokenProducerRegex   = regexp.MustCompile(`(?i)/(auth|login|signin|token|oauth|session)`)
	tokenFieldCandidates = []string{
		"access_token",
		"jwt_token",
		"token",
		"id_token",
		"accessToken",
		"jwtToken",
		"idToken",
		"auth_token",
		"authToken",
	}
)

// isPublicEndpoint checks whether an endpoint path segment indicates a public health, docs, or status endpoint.
func isPublicEndpoint(path string) bool {
	lower := strings.ToLower(path)
	for _, seg := range strings.Split(strings.Trim(lower, "/"), "/") {
		switch seg {
		case "health", "healthz", "ping", "ready", "metrics", "status", "docs", "swagger", "openapi", "public", "favicon.ico":
			return true
		}
	}
	return false
}

// AuthTopology summarizes the authentication producer endpoint, extracted token JSONPath,
// and downstream consumer paths discovered deterministically from an OpenAPI specification.
type AuthTopology struct {
	ProducerEndpoint string   `json:"producer_endpoint"`
	ProducerMethod   string   `json:"producer_method"`
	TokenJSONPath    string   `json:"token_jsonpath"`
	ConsumerPaths    []string `json:"consumer_paths"`
}

// ApplyAuthDetection inspects all normalized operations using a two-tier heuristic:
//  1. Explicit OpenAPI `security` declarations and `Authorization` header parameters.
//  2. Path regex patterns identifying token producers (e.g. `/auth/login`, `/oauth/token`)
//     and protected resource consumers.
//
// It mutates `IsTokenProducer` and `RequiresAuth` on the provided operations slice in place
// and returns the discovered AuthTopology.
func ApplyAuthDetection(spec *OpenAPISpec, ops []NormalizedOperation) AuthTopology {
	globalHasSecurity := len(spec.Security) > 0
	topology := AuthTopology{
		TokenJSONPath: "$.data.access_token",
	}

	hasTokenProducer := false
	for i := range ops {
		op := &ops[i]
		if (op.Method == "POST" || op.Method == "GET") && tokenProducerRegex.MatchString(op.Endpoint) {
			op.IsTokenProducer = true
			op.RequiresAuth = false
			hasTokenProducer = true
			if topology.ProducerEndpoint == "" {
				topology.ProducerEndpoint = op.Endpoint
				topology.ProducerMethod = op.Method
				topology.TokenJSONPath = detectTokenField(op.ResponseSchemas)
			}
		}
	}

	for i := range ops {
		op := &ops[i]
		if op.IsTokenProducer {
			continue
		}

		rawOp := lookupRawOp(spec, op.Endpoint, op.Method)
		hasExplicitSecurity := rawOp != nil && len(rawOp.Security) > 0
		hasExplicitAuthHeader := false
		if rawOp != nil {
			for _, p := range rawOp.Parameters {
				if strings.EqualFold(p.In, "header") && strings.EqualFold(p.Name, "Authorization") {
					hasExplicitAuthHeader = true
					break
				}
			}
		}

		isPublic := isPublicEndpoint(op.Endpoint)

		if hasExplicitSecurity || hasExplicitAuthHeader || globalHasSecurity || (hasTokenProducer && !isPublic) {
			op.RequiresAuth = true
			topology.ConsumerPaths = append(topology.ConsumerPaths, op.Endpoint)
		}
	}

	return topology
}

// lookupRawOp retrieves the unpruned RawOp from the parsed OpenAPISpec for a given path and HTTP method.
func lookupRawOp(spec *OpenAPISpec, path, method string) *RawOp {
	if spec == nil {
		return nil
	}
	mMap, ok := spec.Paths[path]
	if !ok {
		return nil
	}
	if op, ok := mMap[strings.ToLower(method)]; ok {
		return &op
	}
	if op, ok := mMap[strings.ToUpper(method)]; ok {
		return &op
	}
	return nil
}

// detectTokenField inspects the 200/201 response schema of a token producer operation
// to infer the JSONPath expression where the issued authentication token resides, prioritizing
// canonical field names (access_token, jwt_token, token, id_token) over arbitrary token fields.
func detectTokenField(respSchemas map[string]SchemaObject) string {
	schema, ok := respSchemas["200"]
	if !ok {
		schema, ok = respSchemas["201"]
	}
	if !ok {
		return "$.data.access_token"
	}

	dataProp, hasData := schema.Properties["data"]

	// Pass 1: Check prioritized candidate names in strict order
	for _, cand := range tokenFieldCandidates {
		if hasData {
			if _, exists := dataProp.Properties[cand]; exists {
				return "$.data." + cand
			}
		}
		if _, exists := schema.Properties[cand]; exists {
			return "$." + cand
		}
	}

	// Pass 2: Fallback to any property containing "token" or "jwt", excluding refresh/type/expiry
	isIgnored := func(name string) bool {
		lower := strings.ToLower(name)
		return strings.Contains(lower, "refresh") || strings.Contains(lower, "expires") || strings.Contains(lower, "type")
	}

	if hasData {
		for k := range dataProp.Properties {
			lk := strings.ToLower(k)
			if (strings.Contains(lk, "token") || strings.Contains(lk, "jwt")) && !isIgnored(lk) {
				return "$.data." + k
			}
		}
	}
	for k := range schema.Properties {
		lk := strings.ToLower(k)
		if (strings.Contains(lk, "token") || strings.Contains(lk, "jwt")) && !isIgnored(lk) {
			return "$." + k
		}
	}

	return "$.data.access_token"
}
