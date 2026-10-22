package openapi

import (
	"testing"
)

const sampleSpecV1 = `openapi: 3.0.3
info:
  title: Sample API
  version: "1.0.0"
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
                      access_token:
                        type: string
  /api/v1/orders/checkout:
    post:
      operationId: createOrderCheckout
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
              required: [voucher_code, total_amount]
              properties:
                voucher_code:
                  type: string
                total_amount:
                  type: integer
                  minimum: 500000
      responses:
        "201":
          description: Created
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

func TestPruneSpecAndAuthDetection(t *testing.T) {
	res, err := PruneSpec([]byte(sampleSpecV1))
	if err != nil {
		t.Fatalf("PruneSpec failed: %v", err)
	}

	if res.Title != "Sample API" {
		t.Errorf("expected title Sample API, got %q", res.Title)
	}
	if len(res.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(res.Operations))
	}

	var foundProducer, foundConsumer bool
	for _, op := range res.Operations {
		if op.Endpoint == "/api/v1/auth/login" && op.IsTokenProducer {
			foundProducer = true
		}
		if op.Endpoint == "/api/v1/orders/checkout" && op.RequiresAuth {
			foundConsumer = true
		}
	}
	if !foundProducer {
		t.Errorf("expected /api/v1/auth/login to be detected as token producer")
	}
	if !foundConsumer {
		t.Errorf("expected /api/v1/orders/checkout to require auth")
	}

	chunks := ChunkByDomain(res.Operations, 5)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 domain chunks (auth, orders), got %d", len(chunks))
	}
}
