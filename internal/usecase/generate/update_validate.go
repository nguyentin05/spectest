package generate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// validateStepContract checks whether an existing step conforms to a target API contract.
// Returns nil if compatible, or an error detailing the incompatibility.
func validateStepContract(step domain.Step, api domain.API) error {
	// 1. Status code check: if Sig.Responses is non-empty, step.Expect must be listed
	if len(api.Sig.Responses) > 0 && step.Expect > 0 {
		if !slices.Contains(api.Sig.Responses, step.Expect) {
			return fmt.Errorf("expected HTTP status %d not in API responses %v", step.Expect, api.Sig.Responses)
		}
	}

	// 2. Auth check: if API requires auth and step headers have no auth
	if len(api.Sig.Auth) > 0 && !slices.Contains(api.Sig.Auth, "none") {
		hasAuth := false
		for k := range step.Headers {
			lk := strings.ToLower(k)
			if lk == "authorization" || strings.Contains(lk, "token") || strings.Contains(lk, "key") || strings.Contains(lk, "auth") {
				hasAuth = true
				break
			}
		}
		if !hasAuth {
			return fmt.Errorf("operation requires authentication (%v) but step headers contain no auth token or key", api.Sig.Auth)
		}
	}

	// 3. Required parameters check (query and header)
	pc, err := parseCall(step.Call)
	if err == nil {
		for _, p := range api.Parameters {
			if !p.Required {
				continue
			}
			if p.In == "query" {
				if !queryHasKey(pc.RawQuery, p.Name) {
					return fmt.Errorf("missing required query parameter %q", p.Name)
				}
			} else if p.In == "header" {
				hasHeader := false
				for hk := range step.Headers {
					if strings.EqualFold(hk, p.Name) {
						hasHeader = true
						break
					}
				}
				if !hasHeader {
					return fmt.Errorf("missing required header parameter %q", p.Name)
				}
			}
		}
	}

	// 4. Request body checks
	if api.Request != nil {
		if api.Request.Required && step.Body == nil {
			return fmt.Errorf("request body is required by api %s", api.Key)
		}
		if step.Body != nil {
			if api.Request.JSON {
				if errs := validateBodyDeep("", 1, step.Body, api.Request); len(errs) > 0 {
					return fmt.Errorf("request body does not conform to schema: %w", errs[0])
				}
			}
		}
	}

	// 5. Response saves checks (e.g. save: { "id": "body.id" } or { "id": "body.items.0.id" })
	if len(step.Save) > 0 && len(api.Responses) > 0 {
		respSchema, hasResp := api.Responses[strconv.Itoa(step.Expect)]
		if !hasResp {
			respSchema, hasResp = api.Responses["default"]
		}
		if hasResp {
			for varName, expr := range step.Save {
				if strings.HasPrefix(expr, "body.") {
					path := strings.TrimPrefix(expr, "body.")
					parts := strings.Split(path, ".")
					if err := validateResponsePath(respSchema, parts, varName, step.Expect); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateResponsePath traverses nested object properties and array item schemas.
func validateResponsePath(schema domain.Schema, parts []string, varName string, status int) error {
	cur := schema
	for _, part := range parts {
		if cur.Unsupported {
			return nil
		}
		// Array index traversal: e.g. "items.0.id"
		if _, err := strconv.Atoi(part); err == nil {
			if cur.Items != nil {
				cur = *cur.Items
				continue
			}
			return nil
		}
		if len(cur.Properties) == 0 {
			return nil
		}
		next, exists := cur.Properties[part]
		if !exists {
			if cur.AdditionalProperties == nil || !*cur.AdditionalProperties {
				return fmt.Errorf("saved body field %q (segment %q for %q) not found in response %d schema", strings.Join(parts, "."), part, varName, status)
			}
			return nil
		}
		cur = next
	}
	return nil
}
