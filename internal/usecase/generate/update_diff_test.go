package generate

import (
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestDiffCatalog(t *testing.T) {
	oldCatalog := domain.APICatalog{
		"GET {shop}/health": {
			Status: domain.StatusActive,
			Hash:   "hash_health_old",
			Sig:    domain.APISignature{Responses: []int{200}},
		},
		"POST {shop}/api/login": {
			Status: domain.StatusActive,
			Hash:   "hash_login_same",
			Sig:    domain.APISignature{Responses: []int{200}},
		},
		"DELETE {shop}/api/cart": {
			Status: domain.StatusActive,
			Hash:   "hash_cart",
		},
		"GET {shop}/api/profile": {
			Status: domain.StatusRemoved,
			Hash:   "hash_profile_old",
		},
	}

	currentSrc := &domain.SourceSet{
		APIs: []domain.API{
			// Unchanged
			{Key: "POST {shop}/api/login", Hash: "hash_login_same"},
			// Changed
			{Key: "GET {shop}/health", Hash: "hash_health_new"},
			// Restored
			{Key: "GET {shop}/api/profile", Hash: "hash_profile_new"},
			// Added
			{Key: "POST {shop}/api/orders", Hash: "hash_orders_new"},
		},
	}

	diff := DiffCatalog(oldCatalog, currentSrc)

	if diff.IsEmpty() {
		t.Fatal("expected diff to not be empty")
	}

	if len(diff.Added) != 1 || diff.Added[0].Key != "POST {shop}/api/orders" {
		t.Errorf("Added mismatch: %+v", diff.Added)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].Key != "GET {shop}/health" {
		t.Errorf("Changed mismatch: %+v", diff.Changed)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Key != "DELETE {shop}/api/cart" {
		t.Errorf("Removed mismatch: %+v", diff.Removed)
	}
	if len(diff.Restored) != 1 || diff.Restored[0].Key != "GET {shop}/api/profile" {
		t.Errorf("Restored mismatch: %+v", diff.Restored)
	}
}

func TestDiffCatalog_EmptyDiff(t *testing.T) {
	oldCatalog := domain.APICatalog{
		"GET {shop}/health": {
			Status: domain.StatusActive,
			Hash:   "hash1",
		},
	}
	currentSrc := &domain.SourceSet{
		APIs: []domain.API{
			{Key: "GET {shop}/health", Hash: "hash1"},
		},
	}

	diff := DiffCatalog(oldCatalog, currentSrc)
	if !diff.IsEmpty() {
		t.Errorf("expected empty diff, got %+v", diff)
	}
}
