package generate

import (
	"strings"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

func TestBuildUpdatePlan_CompatibleChange(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID: "get-health",
				Steps: []domain.Step{
					{Call: "GET {shop}/health", Expect: 200},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"GET {shop}/health": {
			Key:    "GET {shop}/health",
			Method: "GET",
			Server: "shop",
			Path:   "/health",
			Sig: domain.APISignature{
				Responses: []int{200, 204}, // 200 is still present!
			},
		},
	}
	index, _ := BuildImpactIndex(suite, apis)

	diff := APIDiff{
		Changed: []APIChange{
			{
				Key: "GET {shop}/health",
				Before: domain.APIState{
					Sig: domain.APISignature{Responses: []int{200}},
				},
				After: apis["GET {shop}/health"],
			},
		},
	}

	impact, _ := ComputeImpact(suite, diff, index)
	plan, err := BuildUpdatePlan(suite, diff, impact, index, apis)
	if err != nil {
		t.Fatalf("BuildUpdatePlan failed: %v", err)
	}

	for _, act := range plan {
		if act.RequiresLLM {
			t.Errorf("compatible change should not require LLM: %+v", act)
		}
	}
}

func TestBuildUpdatePlan_IncompatibleResponseStatus(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID: "create-order",
				Steps: []domain.Step{
					{Call: "POST {shop}/api/orders", Expect: 201},
				},
			},
		},
	}
	// New API only returns 200, no 201
	apis := map[string]domain.API{
		"POST {shop}/api/orders": {
			Key:    "POST {shop}/api/orders",
			Method: "POST",
			Server: "shop",
			Path:   "/api/orders",
			Sig: domain.APISignature{
				Responses: []int{200},
			},
		},
	}
	index, _ := BuildImpactIndex(suite, apis)

	diff := APIDiff{
		Changed: []APIChange{
			{
				Key: "POST {shop}/api/orders",
				Before: domain.APIState{
					Sig: domain.APISignature{Responses: []int{201}},
				},
				After: apis["POST {shop}/api/orders"],
			},
		},
	}

	impact, _ := ComputeImpact(suite, diff, index)
	plan, err := BuildUpdatePlan(suite, diff, impact, index, apis)
	if err != nil {
		t.Fatalf("BuildUpdatePlan failed: %v", err)
	}

	foundRepair := false
	for _, act := range plan {
		if act.Kind == ActionRepairScenario && act.ScenarioID == "create-order" && act.RequiresLLM {
			foundRepair = true
		}
	}
	if !foundRepair {
		t.Fatalf("expected ActionRepairScenario for create-order with RequiresLLM=true, got plan: %+v", plan)
	}
}

func TestBuildUpdatePlan_RemovedAPI_AddsToolSkip(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID: "get-profile",
				Steps: []domain.Step{
					{Call: "GET {shop}/api/profile", Expect: 200},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"GET {shop}/api/profile": {
			Key:    "GET {shop}/api/profile",
			Method: "GET",
			Server: "shop",
			Path:   "/api/profile",
		},
	}
	index, _ := BuildImpactIndex(suite, apis)

	diff := APIDiff{
		Removed: []domain.APIStateRef{
			{Key: "GET {shop}/api/profile"},
		},
	}
	impact, _ := ComputeImpact(suite, diff, index)
	plan, err := BuildUpdatePlan(suite, diff, impact, index, apis)
	if err != nil {
		t.Fatalf("BuildUpdatePlan failed: %v", err)
	}

	var skipAction *UpdateAction
	for i := range plan {
		if plan[i].Kind == ActionAddToolSkip && plan[i].ScenarioID == "get-profile" {
			skipAction = &plan[i]
		}
	}
	if skipAction == nil {
		t.Fatalf("expected ActionAddToolSkip for get-profile, got: %+v", plan)
	}
	if !strings.HasPrefix(skipAction.Reason, "spectest: API") {
		t.Errorf("expected skip prefix 'spectest: API', got %q", skipAction.Reason)
	}
}

func TestBuildUpdatePlan_LockedScenarioRequiresUpdate_Fails(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID:     "locked-order",
				Locked: true,
				Steps: []domain.Step{
					{Call: "DELETE {shop}/api/orders", Expect: 204},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"DELETE {shop}/api/orders": {
			Key:    "DELETE {shop}/api/orders",
			Method: "DELETE",
			Server: "shop",
			Path:   "/api/orders",
		},
	}
	index, _ := BuildImpactIndex(suite, apis)

	diff := APIDiff{
		Removed: []domain.APIStateRef{
			{Key: "DELETE {shop}/api/orders"},
		},
	}
	impact, _ := ComputeImpact(suite, diff, index)
	_, err := BuildUpdatePlan(suite, diff, impact, index, apis)
	if err == nil {
		t.Fatal("expected error on locked scenario requiring update/skip, got nil")
	}
	if !strings.Contains(err.Error(), "locked scenario") {
		t.Errorf("error = %v, want 'locked scenario'", err)
	}
}

func TestBuildUpdatePlan_RestoredAPI_RemovesToolSkip(t *testing.T) {
	suite := &domain.Suite{
		Scenarios: []domain.Scenario{
			{
				ID:   "restored-user",
				Skip: "spectest: API GET {shop}/api/users removed",
				Steps: []domain.Step{
					{Call: "GET {shop}/api/users", Expect: 200},
				},
			},
			{
				ID:   "user-skipped",
				Skip: "skip for maintenance by team",
				Steps: []domain.Step{
					{Call: "GET {shop}/api/users", Expect: 200},
				},
			},
		},
	}
	apis := map[string]domain.API{
		"GET {shop}/api/users": {
			Key:    "GET {shop}/api/users",
			Method: "GET",
			Server: "shop",
			Path:   "/api/users",
			Sig:    domain.APISignature{Responses: []int{200}},
		},
	}
	index, _ := BuildImpactIndex(suite, apis)

	diff := APIDiff{
		Restored: []domain.API{apis["GET {shop}/api/users"]},
	}
	impact, _ := ComputeImpact(suite, diff, index)
	plan, err := BuildUpdatePlan(suite, diff, impact, index, apis)
	if err != nil {
		t.Fatalf("BuildUpdatePlan failed: %v", err)
	}

	foundRemove := false
	for _, act := range plan {
		if act.Kind == ActionRemoveToolSkip {
			if act.ScenarioID == "restored-user" {
				foundRemove = true
			}
			if act.ScenarioID == "user-skipped" {
				t.Errorf("must not remove user-authored skip from user-skipped: %+v", act)
			}
		}
	}
	if !foundRemove {
		t.Fatalf("expected ActionRemoveToolSkip for restored-user, got plan: %+v", plan)
	}
}

func TestValidateStepContract_NestedResponsePath(t *testing.T) {
	api := domain.API{
		Key:    "GET {shop}/api/products",
		Method: "GET",
		Server: "shop",
		Path:   "/api/products",
		Sig: domain.APISignature{
			Responses: []int{200},
		},
		Responses: map[string]domain.Schema{
			"200": {
				Type: "object",
				Properties: map[string]domain.Schema{
					"data": {
						Type: "object",
						Properties: map[string]domain.Schema{
							"token": {Type: "string"},
						},
					},
					"items": {
						Type: "array",
						Items: &domain.Schema{
							Type: "object",
							Properties: map[string]domain.Schema{
								"id": {Type: "string"},
							},
						},
					},
				},
			},
		},
	}

	// 1. Valid nested object: body.data.token
	step1 := domain.Step{
		Call:   "GET {shop}/api/products",
		Expect: 200,
		Save:   map[string]string{"tok": "body.data.token"},
	}
	if err := validateStepContract(step1, api); err != nil {
		t.Errorf("expected valid for body.data.token, got: %v", err)
	}

	// 2. Valid array element item: body.items.0.id
	step2 := domain.Step{
		Call:   "GET {shop}/api/products",
		Expect: 200,
		Save:   map[string]string{"pid": "body.items.0.id"},
	}
	if err := validateStepContract(step2, api); err != nil {
		t.Errorf("expected valid for body.items.0.id, got: %v", err)
	}

	// 3. Invalid nested property: body.data.nonexistent
	step3 := domain.Step{
		Call:   "GET {shop}/api/products",
		Expect: 200,
		Save:   map[string]string{"bad": "body.data.nonexistent"},
	}
	if err := validateStepContract(step3, api); err == nil {
		t.Errorf("expected error for body.data.nonexistent, got nil")
	}

	// 4. Invalid array item property: body.items.0.unknown
	step4 := domain.Step{
		Call:   "GET {shop}/api/products",
		Expect: 200,
		Save:   map[string]string{"bad": "body.items.0.unknown"},
	}
	if err := validateStepContract(step4, api); err == nil {
		t.Errorf("expected error for body.items.0.unknown, got nil")
	}
}
