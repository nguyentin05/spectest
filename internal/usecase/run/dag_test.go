package run

import (
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestDAG_LinearDependencies(t *testing.T) {
	setups := map[string]domain.Setup{
		"admin_session": {},
		"product": {
			Use: []string{"admin_session"},
		},
		"order": {
			Use: []string{"product"},
		},
	}

	result, err := ResolveSetupDAG([]string{"order"}, setups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"admin_session", "product", "order"}
	if len(result) != len(expected) {
		t.Fatalf("expected length %d, got %d: %v", len(expected), len(result), result)
	}
	for i, name := range expected {
		if result[i] != name {
			t.Errorf("at index %d: expected %s, got %s", i, name, result[i])
		}
	}
}

func TestDAG_DiamondDependencies(t *testing.T) {
	setups := map[string]domain.Setup{
		"base": {},
		"auth": {
			Use: []string{"base"},
		},
		"catalog": {
			Use: []string{"base"},
		},
		"checkout": {
			Use: []string{"auth", "catalog"},
		},
	}

	result, err := ResolveSetupDAG([]string{"checkout"}, setups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// base must come first, checkout must come last
	if result[0] != "base" {
		t.Errorf("expected base first, got %s", result[0])
	}
	if result[len(result)-1] != "checkout" {
		t.Errorf("expected checkout last, got %s", result[len(result)-1])
	}
	// auth and catalog must both come before checkout and after base
	idxMap := make(map[string]int)
	for i, name := range result {
		idxMap[name] = i
	}
	if idxMap["base"] >= idxMap["auth"] || idxMap["base"] >= idxMap["catalog"] {
		t.Errorf("base must precede auth and catalog")
	}
	if idxMap["auth"] >= idxMap["checkout"] || idxMap["catalog"] >= idxMap["checkout"] {
		t.Errorf("auth and catalog must precede checkout")
	}
}

func TestDAG_CycleDetection(t *testing.T) {
	tests := []struct {
		name    string
		setups  map[string]domain.Setup
		request []string
	}{
		{
			name: "Direct cycle",
			setups: map[string]domain.Setup{
				"A": {Use: []string{"B"}},
				"B": {Use: []string{"A"}},
			},
			request: []string{"A"},
		},
		{
			name: "Self loop",
			setups: map[string]domain.Setup{
				"A": {Use: []string{"A"}},
			},
			request: []string{"A"},
		},
		{
			name: "Three-node cycle",
			setups: map[string]domain.Setup{
				"A": {Use: []string{"B"}},
				"B": {Use: []string{"C"}},
				"C": {Use: []string{"A"}},
			},
			request: []string{"A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveSetupDAG(tt.request, tt.setups)
			if err == nil {
				t.Fatalf("expected cycle error, got nil")
			}
			if !strings.Contains(err.Error(), "cyclic dependency detected") {
				t.Errorf("expected cyclic dependency error message, got: %v", err)
			}
		})
	}
}

func TestDAG_MissingSetup(t *testing.T) {
	setups := map[string]domain.Setup{
		"valid": {
			Use: []string{"missing_dep"},
		},
	}

	// Missing in direct scenario request
	_, err := ResolveSetupDAG([]string{"nonexistent"}, setups)
	if err == nil || !strings.Contains(err.Error(), "not defined in suite.setup") {
		t.Errorf("expected not defined error for direct setup, got: %v", err)
	}

	// Missing in transitive dependency
	_, err = ResolveSetupDAG([]string{"valid"}, setups)
	if err == nil || !strings.Contains(err.Error(), "not defined in suite.setup") {
		t.Errorf("expected not defined error for transitive setup, got: %v", err)
	}
}
