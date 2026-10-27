package run

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/nguyentin05/spectest/internal/platform/buildinfo"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	ShortDescription sarifMultiformatDesc `json:"shortDescription"`
}

type sarifMultiformatDesc struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

// buildSARIF converts a test execution Summary into standard SARIF 2.1.0 format.
func buildSARIF(summary *ucrun.Summary, suitePath string) ([]byte, error) {
	ruleID := "SPECTEST-001"
	driver := sarifDriver{
		Name:           "SpecTest",
		Version:        buildinfo.App(),
		InformationURI: "https://github.com/nguyentin05/spectest",
		Rules: []sarifRule{
			{
				ID:   ruleID,
				Name: "ContractAssertionFailure",
				ShortDescription: sarifMultiformatDesc{
					Text: "API expectation failed or unexpected HTTP status returned",
				},
			},
		},
	}

	results := make([]sarifResult, 0)
	uri := suitePath
	if uri == "" {
		uri = "spectest.yaml"
	}

	if summary != nil {
		for _, rec := range summary.Records {
			if !rec.Passed {
				msg := fmt.Sprintf("[%s] %s failed: expected HTTP %d, got %d (%s)",
					rec.ScenarioID, rec.Call, rec.ExpectedCode, rec.StatusCode, rec.FailReason)
				if rec.AIDiagnosis != "" {
					msg += fmt.Sprintf(" | AI Diagnosis: %s", rec.AIDiagnosis)
				}
				results = append(results, sarifResult{
					RuleID:  ruleID,
					Level:   "error",
					Message: sarifMessage{Text: msg},
					Locations: []sarifLocation{
						{
							PhysicalLocation: sarifPhysicalLocation{
								ArtifactLocation: sarifArtifactLocation{
									URI: filepath.ToSlash(uri),
								},
							},
						},
					},
				})
			}
		}
	}

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool:    sarifTool{Driver: driver},
				Results: results,
			},
		},
	}

	return json.MarshalIndent(log, "", "  ")
}
