package openapi

import (
	"strings"
	"testing"
)

const sampleSpec = `openapi: 3.0.3
info:
  title: Sample API
  version: "1.0.0"
  description: |
    A very long description that should be stripped from the pruned output
    because it only wastes LLM context tokens.
servers:
  - url: http://localhost:8080
paths:
  /api/v1/auth/login:
    post:
      operationId: loginUser
      tags: [auth]
      summary: Authenticate user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/LoginRequest'
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: object
                    properties:
                      jwt_token:
                        type: string
  /api/v1/orders/checkout:
    post:
      tags: [orders]
      summary: Create order
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                voucher_code:
                  type: string
                total_amount:
                  type: integer
                  minimum: 500000
      responses:
        "201":
          description: Created
  /api/v1/health:
    get:
      summary: Health check
      responses:
        "200":
          description: OK
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
  schemas:
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email:
          type: string
          format: email
        password:
          type: string
    UnusedOrphanSchema:
      type: object
      properties:
        legacy_field:
          type: string
`

func findOp(t *testing.T, ops []NormalizedOperation, method, endpoint string) NormalizedOperation {
	t.Helper()
	for _, op := range ops {
		if op.Method == method && op.Endpoint == endpoint {
			return op
		}
	}
	t.Fatalf("operation %s %s not found", method, endpoint)
	return NormalizedOperation{}
}

func TestParseRawSpec_Errors(t *testing.T) {
	if _, err := ParseRawSpec([]byte("openapi: [broken")); err == nil {
		t.Error("expected YAML parse error")
	}
	if _, err := ParseRawSpec([]byte("openapi: 3.0.0\ninfo:\n  title: x\n")); err == nil || !strings.Contains(err.Error(), "no paths") {
		t.Errorf("expected no paths error, got %v", err)
	}
}

func TestPruneSpec_NormalizesOperations(t *testing.T) {
	res, err := PruneSpec([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("PruneSpec: %v", err)
	}

	if res.Title != "Sample API" || res.Version != "1.0.0" || res.BaseURL != "http://localhost:8080" {
		t.Errorf("unexpected metadata: %+v", res)
	}
	if len(res.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(res.Operations))
	}
	if res.OriginalBytes == 0 || res.PrunedBytes == 0 {
		t.Error("byte telemetry should be populated")
	}
	if res.CompressionRatio < 0 || res.CompressionRatio > 100 {
		t.Errorf("CompressionRatio out of range: %f", res.CompressionRatio)
	}

	login := findOp(t, res.Operations, "POST", "/api/v1/auth/login")
	if login.RequestSchema == nil {
		t.Fatal("login request schema should be resolved")
	}
	if _, ok := login.RequestSchema.Properties["email"]; !ok {
		t.Error("$ref LoginRequest should be inlined with email property")
	}
	if login.RequestSchema.Ref != "" {
		t.Error("resolved schema should not keep $ref")
	}

	checkout := findOp(t, res.Operations, "POST", "/api/v1/orders/checkout")
	if checkout.OperationID != "post_orders_checkout" {
		t.Errorf("synthesized operationId = %q", checkout.OperationID)
	}
	if _, ok := checkout.ResponseSchemas["201"]; !ok {
		t.Error("201 response without content should still be recorded")
	}
}

func TestPruneSpec_InvalidInput(t *testing.T) {
	if _, err := PruneSpec([]byte("not: valid: yaml: [")); err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestApplyAuthDetection(t *testing.T) {
	spec, err := ParseRawSpec([]byte(sampleSpec))
	if err != nil {
		t.Fatal(err)
	}
	res, err := PruneSpec([]byte(sampleSpec))
	if err != nil {
		t.Fatal(err)
	}

	topology := ApplyAuthDetection(spec, res.Operations)
	if topology.ProducerEndpoint != "/api/v1/auth/login" || topology.ProducerMethod != "POST" {
		t.Errorf("unexpected producer: %+v", topology)
	}
	if topology.TokenJSONPath != "$.data.jwt_token" {
		t.Errorf("TokenJSONPath = %q, want $.data.jwt_token", topology.TokenJSONPath)
	}

	login := findOp(t, res.Operations, "POST", "/api/v1/auth/login")
	if !login.IsTokenProducer || login.RequiresAuth {
		t.Errorf("login should be producer without auth: %+v", login)
	}
	checkout := findOp(t, res.Operations, "POST", "/api/v1/orders/checkout")
	if !checkout.RequiresAuth {
		t.Error("checkout with security block should require auth")
	}
	health := findOp(t, res.Operations, "GET", "/api/v1/health")
	if health.RequiresAuth || health.IsTokenProducer {
		t.Error("public health endpoint should not require auth")
	}
}

func TestApplyAuthDetection_AuthorizationHeaderParameter(t *testing.T) {
	spec := &OpenAPISpec{
		Paths: map[string]map[string]RawOp{
			"/api/v1/reports": {
				"get": {Parameters: []ParameterObject{{Name: "Authorization", In: "header"}}},
			},
		},
	}
	ops := []NormalizedOperation{{Method: "GET", Endpoint: "/api/v1/reports"}}
	ApplyAuthDetection(spec, ops)
	if !ops[0].RequiresAuth {
		t.Error("explicit Authorization header parameter should mark endpoint as requiring auth")
	}
}

func TestDetectTokenField(t *testing.T) {
	tests := []struct {
		name    string
		schemas map[string]SchemaObject
		want    string
	}{
		{"no success response", map[string]SchemaObject{}, "$.data.access_token"},
		{"top-level token", map[string]SchemaObject{
			"201": {Properties: map[string]SchemaObject{"token": {Type: "string"}}},
		}, "$.token"},
		{"no token field", map[string]SchemaObject{
			"200": {Properties: map[string]SchemaObject{"id": {Type: "string"}}},
		}, "$.data.access_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectTokenField(tt.schemas); got != tt.want {
				t.Errorf("detectTokenField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLookupRawOp(t *testing.T) {
	spec := &OpenAPISpec{Paths: map[string]map[string]RawOp{
		"/a": {"GET": {OperationID: "upper"}},
	}}
	if op := lookupRawOp(spec, "/a", "get"); op == nil || op.OperationID != "upper" {
		t.Error("should match uppercase method key")
	}
	if lookupRawOp(spec, "/missing", "get") != nil {
		t.Error("missing path should return nil")
	}
	if lookupRawOp(nil, "/a", "get") != nil {
		t.Error("nil spec should return nil")
	}
}

func TestSynthesizeSamplePayload(t *testing.T) {
	minVal := 10.0
	schema := SchemaObject{
		Type: "object",
		Properties: map[string]SchemaObject{
			"email":    {Type: "string", Format: "email"},
			"name":     {Type: "string"},
			"status":   {Type: "string", Enum: []interface{}{"ACTIVE", "INACTIVE"}},
			"note":     {Type: "string", Example: "hello"},
			"qty":      {Type: "integer", Minimum: &minVal},
			"price":    {Type: "number"},
			"enabled":  {Type: "boolean"},
			"tags":     {Type: "array", Items: &SchemaObject{Type: "string"}},
			"empty":    {Type: "array"},
			"address":  {Type: "object", Properties: map[string]SchemaObject{"city": {Type: "string"}}},
			"metadata": {Type: "object"},
		},
	}

	p := SynthesizeSamplePayload(schema)

	checks := map[string]interface{}{
		"email":   "user@example.com",
		"name":    "sample_name",
		"status":  "ACTIVE",
		"note":    "hello",
		"qty":     10.0,
		"price":   1,
		"enabled": true,
	}
	for k, want := range checks {
		if p[k] != want {
			t.Errorf("payload[%q] = %#v, want %#v", k, p[k], want)
		}
	}
	if tags, ok := p["tags"].([]interface{}); !ok || len(tags) != 1 {
		t.Errorf("tags = %#v, want single-item array", p["tags"])
	}
	if empty, ok := p["empty"].([]interface{}); !ok || len(empty) != 0 {
		t.Errorf("empty = %#v, want empty array", p["empty"])
	}
	if addr, ok := p["address"].(map[string]interface{}); !ok || addr["city"] != "sample_city" {
		t.Errorf("address = %#v", p["address"])
	}
	if meta, ok := p["metadata"].(map[string]interface{}); !ok || len(meta) != 0 {
		t.Errorf("metadata = %#v, want empty object", p["metadata"])
	}
}

func TestResolveSchema_DepthLimitAndUnknownRef(t *testing.T) {
	components := map[string]SchemaObject{
		"Node": {Type: "object", Properties: map[string]SchemaObject{
			"child": {Ref: "#/components/schemas/Node"},
		}},
	}
	out := resolveSchema(SchemaObject{Ref: "#/components/schemas/Node"}, components, 0)
	if out.Type != "object" {
		t.Errorf("recursive schema should resolve to object, got %q", out.Type)
	}

	unknown := resolveSchema(SchemaObject{Ref: "#/components/schemas/Missing"}, components, 0)
	if unknown.Ref != "" || unknown.Type != "" {
		t.Errorf("unknown ref should resolve to empty schema, got %+v", unknown)
	}

	inferred := resolveSchema(SchemaObject{Properties: map[string]SchemaObject{"a": {Type: "string"}}}, nil, 0)
	if inferred.Type != "object" {
		t.Errorf("schema with properties should infer object type, got %q", inferred.Type)
	}
}

func TestTruncateSummary(t *testing.T) {
	if got := truncateSummary("", "first line\nsecond line"); got != "first line" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("x", 150)
	if got := truncateSummary(long, ""); len(got) != 100 || !strings.HasSuffix(got, "...") {
		t.Errorf("long summary should be truncated to 100 chars, got len %d", len(got))
	}
}

func TestSynthesizeOperationID(t *testing.T) {
	tests := map[[2]string]string{
		{"GET", "/api/v1/users/{id}"}: "get_users_id",
		{"POST", "/v2/order-items"}:   "post_order_items",
		{"DELETE", "/"}:               "delete_",
	}
	for in, want := range tests {
		if got := synthesizeOperationID(in[0], in[1]); got != want {
			t.Errorf("synthesizeOperationID(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestChunkByDomain(t *testing.T) {
	producer := NormalizedOperation{Method: "POST", Endpoint: "/auth/login", Tags: []string{"auth"}, IsTokenProducer: true}
	ops := []NormalizedOperation{producer}
	for i := 0; i < 7; i++ {
		ops = append(ops, NormalizedOperation{Method: "GET", Endpoint: "/orders", Tags: []string{"orders"}, RequiresAuth: true})
	}
	ops = append(ops, NormalizedOperation{Method: "GET", Endpoint: "/api/v1/products/{id}"})

	chunks := ChunkByDomain(ops, 5)
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks (auth, orders x2, products), got %d", len(chunks))
	}

	if chunks[0].Tag != "auth" || chunks[0].TokenProducer != nil {
		t.Errorf("auth chunk should not attach producer to itself: %+v", chunks[0])
	}
	if chunks[1].Tag != "orders" || len(chunks[1].Operations) != 5 || chunks[1].TokenProducer == nil {
		t.Errorf("first orders chunk should hold 5 ops with producer: %+v", chunks[1])
	}
	if len(chunks[2].Operations) != 2 || chunks[2].TokenProducer == nil {
		t.Errorf("second orders chunk should hold 2 ops with producer: %+v", chunks[2])
	}
	if chunks[3].Tag != "products" || chunks[3].TokenProducer != nil {
		t.Errorf("public products chunk should derive tag from path and skip producer: %+v", chunks[3])
	}
}

func TestChunkByDomain_DefaultsAndFallbackTag(t *testing.T) {
	ops := []NormalizedOperation{{Method: "GET", Endpoint: "/api/v1/{id}"}}
	chunks := ChunkByDomain(ops, 0)
	if len(chunks) != 1 || chunks[0].Tag != "default" {
		t.Errorf("expected single default chunk, got %+v", chunks)
	}
}
