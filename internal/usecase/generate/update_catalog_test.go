package generate

import (
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestReconstructCatalog_Success(t *testing.T) {
	log := &domain.LogFile{
		Base: domain.BaseEntry{
			APICatalog: domain.APICatalog{
				"GET {shop}/api/products": {
					Status: domain.StatusActive,
					Hash:   "hash1",
					Sig:    domain.APISignature{Responses: []int{200}},
				},
				"POST {shop}/api/login": {
					Status: domain.StatusActive,
					Hash:   "hash2",
					Sig:    domain.APISignature{Responses: []int{200}},
				},
			},
		},
		Update: []domain.UpdateEntry{
			{
				N: 1,
				CatalogPatch: domain.APICatalog{
					"POST {shop}/api/orders": {
						Status: domain.StatusActive,
						Hash:   "hash3",
						Sig:    domain.APISignature{Responses: []int{201}},
					},
					"POST {shop}/api/login": {
						Status: domain.StatusRemoved,
						Hash:   "hash2",
					},
				},
			},
			{
				N: 2,
				CatalogPatch: domain.APICatalog{
					"POST {shop}/api/login": {
						Status: domain.StatusActive,
						Hash:   "hash2_v2",
						Sig:    domain.APISignature{Responses: []int{200, 401}},
					},
				},
			},
		},
	}

	catalog, err := ReconstructCatalog(log)
	if err != nil {
		t.Fatalf("ReconstructCatalog failed: %v", err)
	}

	if len(catalog) != 3 {
		t.Fatalf("expected 3 entries in catalog, got %d", len(catalog))
	}

	// GET products should remain active hash1
	if prod, ok := catalog["GET {shop}/api/products"]; !ok || prod.Status != domain.StatusActive || prod.Hash != "hash1" {
		t.Errorf("products mismatch: %+v", prod)
	}
	// POST orders should be active hash3
	if orders, ok := catalog["POST {shop}/api/orders"]; !ok || orders.Status != domain.StatusActive || orders.Hash != "hash3" {
		t.Errorf("orders mismatch: %+v", orders)
	}
	// POST login was removed in update 1 and restored in update 2 with hash2_v2
	if login, ok := catalog["POST {shop}/api/login"]; !ok || login.Status != domain.StatusActive || login.Hash != "hash2_v2" {
		t.Errorf("login mismatch: %+v", login)
	}
}

func TestReconstructCatalog_MissingBaseCatalog(t *testing.T) {
	log := &domain.LogFile{
		Base: domain.BaseEntry{
			APICatalog: nil,
		},
	}

	_, err := ReconstructCatalog(log)
	if err == nil {
		t.Fatal("expected error on missing base catalog, got nil")
	}
	if !strings.Contains(err.Error(), "history log has no API catalog; regenerate once with the current format") {
		t.Errorf("error = %v, want guidance to regenerate", err)
	}
}

func TestReconstructCatalog_InvalidSequence(t *testing.T) {
	cases := []struct {
		name    string
		updates []domain.UpdateEntry
	}{
		{
			name: "gap in N",
			updates: []domain.UpdateEntry{
				{N: 1},
				{N: 3},
			},
		},
		{
			name: "duplicate N",
			updates: []domain.UpdateEntry{
				{N: 1},
				{N: 1},
			},
		},
		{
			name: "start at 0",
			updates: []domain.UpdateEntry{
				{N: 0},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &domain.LogFile{
				Base: domain.BaseEntry{
					APICatalog: domain.APICatalog{},
				},
				Update: tc.updates,
			}
			_, err := ReconstructCatalog(log)
			if err == nil {
				t.Fatalf("expected error on invalid sequence %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "update sequence") {
				t.Errorf("error = %v, want update sequence error", err)
			}
		})
	}
}
