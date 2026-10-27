package openapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestNormalize_CapturesRawSnippet(t *testing.T) {
	raw := []byte(`openapi: 3.0.0
info: {title: Test, version: 1.0.0}
paths:
  /login:
    post:
      summary: User login
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [email, password]
              properties:
                email: {type: string, format: email}
                password: {type: string, minLength: 8}
      responses:
        '200':
          description: OK
`)
	apis, err := Normalize("server", raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(apis) == 0 || apis[0].RawSnippet == "" {
		t.Fatal("expected non-empty RawSnippet on normalized API")
	}
	if !strings.Contains(apis[0].RawSnippet, "email") {
		t.Errorf("expected RawSnippet to contain schema property 'email', got: %s", apis[0].RawSnippet)
	}
}

func normalizeOK(t *testing.T, server, raw string) []domain.API {
	t.Helper()
	apis, err := Normalize(server, []byte(raw))
	if err != nil {
		t.Fatalf("Normalize() error = %v, want nil", err)
	}
	return apis
}

func findAPI(t *testing.T, apis []domain.API, key string) domain.API {
	t.Helper()
	for _, api := range apis {
		if api.Key == key {
			return api
		}
	}
	var keys []string
	for _, api := range apis {
		keys = append(keys, api.Key)
	}
	t.Fatalf("API %q not found in %v", key, keys)
	return domain.API{}
}

func TestNormalize_RejectsSwagger2(t *testing.T) {
	raw := `
swagger: "2.0"
info:
  title: Legacy
  version: "1"
paths:
  /users:
    get:
      responses:
        "200":
          description: ok
`
	_, err := Normalize("api", []byte(raw))
	if err == nil {
		t.Fatal("Normalize() error = nil, want Swagger 2 rejection")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "swagger") {
		t.Errorf("Normalize() error = %v, want it to mention swagger", err)
	}
}

func TestNormalize_RejectsMissingOpenAPI(t *testing.T) {
	raw := `
info:
  title: No version
  version: "1"
paths:
  /users:
    get:
      responses:
        "200":
          description: ok
`
	_, err := Normalize("api", []byte(raw))
	if err == nil {
		t.Fatal("Normalize() error = nil, want missing openapi rejection")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "openapi") {
		t.Errorf("Normalize() error = %v, want it to mention openapi", err)
	}
}

func TestNormalize_RejectsNonOpenAPI3Version(t *testing.T) {
	for _, version := range []string{"2.0", "3.x", "4.0.0"} {
		t.Run(version, func(t *testing.T) {
			raw := fmt.Sprintf(`
openapi: %q
paths:
  /users:
    get:
      responses:
        "200":
          description: ok
`, version)
			if _, err := Normalize("api", []byte(raw)); err == nil {
				t.Fatalf("Normalize(openapi: %q) error = nil, want rejection", version)
			}
		})
	}
}

func TestNormalize_KeyMethodServerPath(t *testing.T) {
	raw := `
openapi: 3.0.3
paths:
  /z:
    post:
      responses:
        "201":
          description: created
    get:
      responses:
        "200":
          description: ok
  /a/{id}:
    get:
      responses:
        "200":
          description: ok
  /search?q=1:
    get:
      responses:
        "200":
          description: ok
`
	apis := normalizeOK(t, "api", raw)

	want := []string{
		"GET {api}/a/{id}",
		"GET {api}/search",
		"GET {api}/z",
		"POST {api}/z",
	}
	if len(apis) != len(want) {
		t.Fatalf("len(apis) = %d, want %d: %+v", len(apis), len(want), apis)
	}
	for i, key := range want {
		if apis[i].Key != key {
			t.Errorf("apis[%d].Key = %q, want %q", i, apis[i].Key, key)
		}
		if apis[i].Server != "api" {
			t.Errorf("apis[%d].Server = %q, want %q", i, apis[i].Server, "api")
		}
		if len(apis[i].Hash) != 12 || strings.ToLower(apis[i].Hash) != apis[i].Hash {
			t.Errorf("apis[%d].Hash = %q, want 12 lowercase hex characters", i, apis[i].Hash)
		}
		if len(apis[i].Sig.Responses) != 1 {
			t.Errorf("apis[%d].Sig.Responses = %v, want one status code", i, apis[i].Sig.Responses)
		}
	}
	if got := apis[0].Path; got != "/a/{id}" {
		t.Errorf("path = %q, want %q", got, "/a/{id}")
	}
	if got := apis[0].Method; got != "GET" {
		t.Errorf("method = %q, want GET", got)
	}
	// The path must never leak the query string from the document key.
	if got := apis[1].Path; got != "/search" {
		t.Errorf("path = %q, want query-free %q", got, "/search")
	}
}

func TestNormalize_UsesCLIServerNotSpecServers(t *testing.T) {
	raw := `
openapi: 3.0.3
servers:
  - url: https://spec.example.com/v1
paths:
  /users/{id}:
    get:
      responses:
        "200":
          description: ok
`
	apis := normalizeOK(t, "cli", raw)
	if len(apis) != 1 {
		t.Fatalf("len(apis) = %d, want 1", len(apis))
	}
	api := apis[0]
	if api.Server != "cli" {
		t.Errorf("Server = %q, want cli", api.Server)
	}
	if api.Key != "GET {cli}/users/{id}" {
		t.Errorf("Key = %q, want %q", api.Key, "GET {cli}/users/{id}")
	}
	if api.Path != "/users/{id}" {
		t.Errorf("Path = %q, want %q", api.Path, "/users/{id}")
	}
	if strings.Contains(api.Key, "spec.example.com") {
		t.Errorf("Key = %q, must not use spec servers", api.Key)
	}
}

func TestNormalize_DescriptionFromSummaryThenDescription(t *testing.T) {
	longSummary := strings.Repeat("a", 300) + "TAIL"
	raw := fmt.Sprintf(`
openapi: 3.0.3
paths:
  /summary:
    get:
      summary: "  Short\n summary  "
      description: ignored description
      responses:
        "200":
          description: ok
  /description:
    get:
      description: "  Long\n description  "
      responses:
        "200":
          description: ok
  /long:
    get:
      summary: %q
      responses:
        "200":
          description: ok
  /empty:
    get:
      responses:
        "200":
          description: ok
`, longSummary)
	apis := normalizeOK(t, "api", raw)

	cases := map[string]string{
		"GET {api}/summary":     "Short summary",
		"GET {api}/description": "Long description",
		"GET {api}/long":        strings.Repeat("a", 300),
		"GET {api}/empty":       "",
	}
	for key, want := range cases {
		api := findAPI(t, apis, key)
		if api.Description != want {
			t.Errorf("%s Description = %q, want %q", key, api.Description, want)
		}
		if strings.ContainsAny(api.Description, "\r\n") {
			t.Errorf("%s Description = %q, want a single line", key, api.Description)
		}
		if len([]rune(api.Description)) > 300 {
			t.Errorf("%s Description length = %d, want <= 300", key, len([]rune(api.Description)))
		}
	}
}

func TestNormalize_MergesPathAndOperationParameters(t *testing.T) {
	raw := `
openapi: 3.0.3
paths:
  /users/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
      - name: verbose
        in: query
        schema:
          type: boolean
      - name: X-Trace
        in: header
        schema:
          type: string
      - name: session
        in: cookie
        schema:
          type: string
    get:
      parameters:
        - name: verbose
          in: query
          required: true
          schema:
            type: integer
        - name: limit
          in: query
          schema:
            type: integer
      responses:
        "200":
          description: ok
`
	apis := normalizeOK(t, "api", raw)
	if len(apis) != 1 {
		t.Fatalf("len(apis) = %d, want 1", len(apis))
	}
	params := apis[0].Parameters

	want := []struct {
		in       string
		name     string
		typ      string
		required bool
	}{
		{"cookie", "session", "string", false},
		{"header", "X-Trace", "string", false},
		{"path", "id", "string", true},
		{"query", "limit", "integer", false},
		{"query", "verbose", "integer", true},
	}
	if len(params) != len(want) {
		t.Fatalf("len(Parameters) = %d, want %d: %+v", len(params), len(want), params)
	}
	for i, expected := range want {
		got := params[i]
		if got.In != expected.in || got.Name != expected.name {
			t.Errorf("parameters[%d] = (%q,%q), want (%q,%q)", i, got.In, got.Name, expected.in, expected.name)
		}
		if got.Schema.Type != expected.typ {
			t.Errorf("parameters[%d].Schema.Type = %q, want %q", i, got.Schema.Type, expected.typ)
		}
		if got.Required != expected.required {
			t.Errorf("parameters[%d].Required = %v, want %v", i, got.Required, expected.required)
		}
	}
}

func TestNormalize_SelectsJSONMediaTypeThenWildcardThenLexicalFirst(t *testing.T) {
	raw := `
openapi: 3.0.3
paths:
  /json:
    post:
      requestBody:
        content:
          application/xml:
            schema:
              type: string
          "application/json; charset=utf-8":
            schema:
              type: object
              properties:
                name:
                  type: string
          "*/*":
            schema:
              type: integer
      responses:
        "200":
          description: ok
  /wildcard:
    post:
      requestBody:
        content:
          application/xml:
            schema:
              type: string
          "*/*":
            schema:
              type: boolean
      responses:
        "200":
          description: ok
  /lexical:
    post:
      requestBody:
        content:
          text/plain:
            schema:
              type: string
          application/xml:
            schema:
              type: integer
      responses:
        "200":
          description: ok
  /none:
    post:
      responses:
        "200":
          description: ok
`
	apis := normalizeOK(t, "api", raw)

	jsonAPI := findAPI(t, apis, "POST {api}/json")
	if jsonAPI.Request == nil {
		t.Fatal("json request body = nil, want selected media type")
	}
	if jsonAPI.Request.MediaType != "application/json" || !jsonAPI.Request.JSON {
		t.Errorf("json Request = %+v, want application/json JSON=true", jsonAPI.Request)
	}
	if jsonAPI.Request.Schema.Type != "object" {
		t.Errorf("json Request.Schema.Type = %q, want object", jsonAPI.Request.Schema.Type)
	}
	if got := jsonAPI.Request.Schema.Properties["name"].Type; got != "string" {
		t.Errorf("json Request.Schema.Properties[name].Type = %q, want string", got)
	}

	wildcard := findAPI(t, apis, "POST {api}/wildcard")
	if wildcard.Request == nil {
		t.Fatal("wildcard request body = nil")
	}
	if wildcard.Request.MediaType != "*/*" || wildcard.Request.JSON {
		t.Errorf("wildcard Request = %+v, want */* JSON=false", wildcard.Request)
	}
	if wildcard.Request.Schema.Type != "boolean" {
		t.Errorf("wildcard Request.Schema.Type = %q, want boolean", wildcard.Request.Schema.Type)
	}

	lexical := findAPI(t, apis, "POST {api}/lexical")
	if lexical.Request == nil {
		t.Fatal("lexical request body = nil")
	}
	if lexical.Request.MediaType != "application/xml" || lexical.Request.JSON {
		t.Errorf("lexical Request = %+v, want application/xml JSON=false", lexical.Request)
	}
	if lexical.Request.Schema.Type != "integer" {
		t.Errorf("lexical Request.Schema.Type = %q, want integer", lexical.Request.Schema.Type)
	}

	if none := findAPI(t, apis, "POST {api}/none"); none.Request != nil {
		t.Errorf("request body absent: Request = %+v, want nil", none.Request)
	}
}

func TestNormalize_ResolvesLocalSchemaAndParameterRefs(t *testing.T) {
	raw := `
openapi: 3.0.3
components:
  schemas:
    Id:
      type: integer
    User:
      type: object
      required: [name, id]
      properties:
        id:
          $ref: "#/components/schemas/Id"
        name:
          type: string
    CreateUser:
      type: object
      properties:
        id:
          $ref: "#/components/schemas/Id"
        name:
          type: string
  parameters:
    UserID:
      name: id
      in: path
      required: true
      schema:
        $ref: "#/components/schemas/Id"
paths:
  /users/{id}:
    parameters:
      - $ref: "#/components/parameters/UserID"
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUser"
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
`
	apis := normalizeOK(t, "api", raw)
	if len(apis) != 1 {
		t.Fatalf("len(apis) = %d, want 1", len(apis))
	}
	api := apis[0]

	if len(api.Parameters) != 1 {
		t.Fatalf("len(Parameters) = %d, want 1", len(api.Parameters))
	}
	if got := api.Parameters[0]; got.Name != "id" || got.In != "path" || !got.Required || got.Schema.Type != "integer" {
		t.Errorf("parameter = %+v, want resolved path id integer", got)
	}

	if api.Request == nil {
		t.Fatal("Request = nil, want resolved request body")
	}
	if got := api.Request.Schema.Properties["id"].Type; got != "integer" {
		t.Errorf("request id type = %q, want integer", got)
	}
	if got := api.Request.Schema.Properties["name"].Type; got != "string" {
		t.Errorf("request name type = %q, want string", got)
	}

	response, ok := api.Responses["200"]
	if !ok {
		t.Fatalf("Responses = %+v, want 200", api.Responses)
	}
	if got := response.Properties["id"].Type; got != "integer" {
		t.Errorf("response id type = %q, want integer", got)
	}
	if !containsString(response.Required, "id") || !containsString(response.Required, "name") {
		t.Errorf("response Required = %v, want id and name", response.Required)
	}
}

func TestNormalize_CircularRefDoesNotHang(t *testing.T) {
	raw := `
openapi: 3.0.3
components:
  schemas:
    Node:
      type: object
      properties:
        value:
          type: string
        next:
          $ref: "#/components/schemas/Node"
paths:
  /nodes:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Node"
`
	apis := normalizeOK(t, "api", raw)
	if len(apis) != 1 {
		t.Fatalf("len(apis) = %d, want 1", len(apis))
	}
	schema := apis[0].Responses["200"]
	if schema.Type != "object" {
		t.Fatalf("response schema Type = %q, want object", schema.Type)
	}
	if got := schema.Properties["value"].Type; got != "string" {
		t.Errorf("response value type = %q, want string", got)
	}
	next, ok := schema.Properties["next"]
	if !ok {
		t.Fatalf("response properties = %+v, want next", schema.Properties)
	}
	if !next.Unsupported {
		t.Errorf("circular property Unsupported = false, want true: %+v", next)
	}
}

func TestNormalize_RejectsExternalRef(t *testing.T) {
	refs := []string{
		"./other.yaml#/components/schemas/User",
		"https://example.com/openapi.yaml#/components/schemas/User",
		"file.yaml",
	}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			raw := fmt.Sprintf(`
openapi: 3.0.3
paths:
  /users:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: %q
`, ref)
			_, err := Normalize("api", []byte(raw))
			if err == nil {
				t.Fatalf("Normalize() error = nil, want external ref rejection")
			}
			if !strings.Contains(err.Error(), ref) {
				t.Errorf("Normalize() error = %v, want it to name ref %q", err, ref)
			}
		})
	}
}

func TestNormalize_EmptyPathsIsError(t *testing.T) {
	for name, raw := range map[string]string{
		"explicit empty": `
openapi: 3.0.3
paths: {}
`,
		"missing": `
openapi: 3.0.3
info:
  title: no paths
  version: "1"
`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Normalize("api", []byte(raw)); err == nil {
				t.Fatal("Normalize() error = nil, want empty paths error")
			}
		})
	}
}

func TestNormalize_ResponsesKeepStringCodes(t *testing.T) {
	raw := `
openapi: 3.0.3
paths:
  /users:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: string
        "2XX":
          description: broad
          content:
            application/json:
              schema:
                type: integer
        default:
          description: fallback
`
	apis := normalizeOK(t, "api", raw)
	responses := apis[0].Responses
	if len(responses) != 3 {
		t.Fatalf("len(Responses) = %d, want 3: %+v", len(responses), responses)
	}
	if responses["200"].Type != "string" {
		t.Errorf("200 type = %q, want string", responses["200"].Type)
	}
	if responses["2XX"].Type != "integer" {
		t.Errorf("2XX type = %q, want integer", responses["2XX"].Type)
	}
	if _, ok := responses["default"]; !ok {
		t.Errorf("Responses = %+v, want default", responses)
	}
}

func TestNormalize_UnsupportedSchemaConstructsAreFlagged(t *testing.T) {
	raw := `
openapi: 3.0.3
paths:
  /all-of:
    post:
      requestBody:
        content:
          application/json:
            schema:
              allOf:
                - type: object
      responses:
        "200":
          description: ok
  /additional-props:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              additionalProperties:
                type: string
      responses:
        "200":
          description: ok
  /boolean-additional-props:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              additionalProperties: false
      responses:
        "200":
          description: ok
`
	apis := normalizeOK(t, "api", raw)

	allOf := findAPI(t, apis, "POST {api}/all-of")
	if allOf.Request == nil || !allOf.Request.Schema.Unsupported {
		t.Errorf("allOf schema = %+v, want Unsupported=true", allOf.Request)
	}

	additional := findAPI(t, apis, "POST {api}/additional-props")
	if additional.Request == nil || !additional.Request.Schema.Unsupported {
		t.Errorf("schema-valued additionalProperties = %+v, want Unsupported=true", additional.Request)
	}

	boolean := findAPI(t, apis, "POST {api}/boolean-additional-props")
	if boolean.Request == nil {
		t.Fatal("boolean additionalProperties Request = nil")
	}
	if boolean.Request.Schema.Unsupported {
		t.Errorf("boolean additionalProperties should be representable: %+v", boolean.Request.Schema)
	}
	if boolean.Request.Schema.AdditionalProperties == nil || *boolean.Request.Schema.AdditionalProperties {
		t.Errorf("AdditionalProperties = %v, want pointer to false", boolean.Request.Schema.AdditionalProperties)
	}
}

func TestNormalize_SecurityOverrideAndUnion(t *testing.T) {
	raw := `
openapi: 3.0.3
security:
  - apiKey: []
components:
  securitySchemes:
    apiKey:
      type: apiKey
      in: header
      name: X-API-Key
    bearerAuth:
      type: http
      scheme: bearer
    oauth:
      type: oauth2
paths:
  /global:
    get:
      responses:
        "200":
          description: ok
  /public:
    get:
      security: []
      responses:
        "200":
          description: ok
  /override:
    get:
      security:
        - bearerAuth: []
      responses:
        "200":
          description: ok
  /alternatives:
    get:
      security:
        - oauth: [read]
        - apiKey: []
          bearerAuth: []
      responses:
        "200":
          description: ok
`
	ops, err := normalizeDocument([]byte(raw))
	if err != nil {
		t.Fatalf("normalizeDocument() error = %v, want nil", err)
	}
	byPath := make(map[string]normalizedOperation, len(ops))
	for _, op := range ops {
		byPath[op.Path] = op
	}

	requireAuth := func(path string, want []string) {
		t.Helper()
		op, ok := byPath[path]
		if !ok {
			t.Fatalf("operation %q not found", path)
		}
		got := securitySchemeNames(op.Security)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s auth = %v, want %v", path, got, want)
		}
	}

	requireAuth("/global", []string{"apiKey"})
	requireAuth("/public", nil)
	requireAuth("/override", []string{"bearerAuth"})
	requireAuth("/alternatives", []string{"apiKey", "bearerAuth", "oauth"})

	alternatives := byPath["/alternatives"]
	if len(alternatives.Security) != 2 {
		t.Fatalf("alternatives security = %+v, want 2 requirements", alternatives.Security)
	}
	for _, requirement := range alternatives.Security {
		for _, scheme := range requirement.Schemes {
			if scheme.Name == "oauth" && (len(scheme.Scopes) != 1 || scheme.Scopes[0] != "read") {
				t.Errorf("oauth scopes = %v, want [read]", scheme.Scopes)
			}
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
