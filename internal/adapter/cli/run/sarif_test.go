package run

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

func TestBuildSARIF_ValidStructureAndFailures(t *testing.T) {
	summary := &ucrun.Summary{
		SuiteName:      "SpecTest Suite",
		TotalScenarios: 2,
		Records: []ucrun.ExecutionRecord{
			{
				ScenarioID:   "login-success",
				ScenarioName: "Login",
				Call:         "POST {auth}/api/v1/login",
				Passed:       true,
				StatusCode:   200,
				ExpectedCode: 200,
			},
			{
				ScenarioID:   "user-profile",
				ScenarioName: "Get Profile",
				Call:         "GET {auth}/api/v1/me",
				Passed:       false,
				StatusCode:   401,
				ExpectedCode: 200,
				FailReason:   "expected HTTP status 200, got 401",
				AIDiagnosis:  "Missing Bearer token in request header",
			},
		},
		TotalDuration: 100 * time.Millisecond,
	}

	bytes, err := buildSARIF(summary, "tests/spectest.yaml")
	if err != nil {
		t.Fatalf("buildSARIF error: %v", err)
	}

	var log sarifLog
	if err := json.Unmarshal(bytes, &log); err != nil {
		t.Fatalf("unmarshal sarif error: %v, raw: %s", err, string(bytes))
	}

	if log.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", log.Version)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("runs count = %d, want 1", len(log.Runs))
	}

	run := log.Runs[0]
	if run.Tool.Driver.Name != "SpecTest" {
		t.Errorf("driver name = %q, want SpecTest", run.Tool.Driver.Name)
	}
	if len(run.Results) != 1 {
		t.Fatalf("results count = %d, want 1 (only failed step)", len(run.Results))
	}

	result := run.Results[0]
	if result.Level != "error" {
		t.Errorf("level = %q, want error", result.Level)
	}
	if !strings.Contains(result.Message.Text, "user-profile") {
		t.Errorf("expected scenario id in message: %s", result.Message.Text)
	}
	if !strings.Contains(result.Message.Text, "AI Diagnosis") {
		t.Errorf("expected AI Diagnosis in message: %s", result.Message.Text)
	}
	if result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "tests/spectest.yaml" {
		t.Errorf("uri = %q, want tests/spectest.yaml", result.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
}
