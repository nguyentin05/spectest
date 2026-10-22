package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
	"gopkg.in/yaml.v3"
)

// YAMLRepository implements domain.SuiteRepository for reading and writing `spectest.yaml`
// test suite files, as well as exporting Markdown coverage matrices and Postman v2.1 collections.
type YAMLRepository struct{}

// NewYAMLRepository creates a new stateless YAMLRepository instance.
func NewYAMLRepository() *YAMLRepository {
	return &YAMLRepository{}
}

// Load reads a `spectest.yaml` file from disk, unmarshals its YAML content into a
// domain.TestSuiteFile, and defaults the schema version to `"1.0"` if omitted.
func (r *YAMLRepository) Load(path string) (*domain.TestSuiteFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite file %s: %w", path, err)
	}
	var suite domain.TestSuiteFile
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("unmarshal suite yaml %s: %w", path, err)
	}
	if suite.Version == "" {
		suite.Version = "1.0"
	}
	return &suite, nil
}

// Save marshals a domain.TestSuiteFile into formatted 2-space indented YAML with a
// header banner and writes it to disk, creating parent directories if needed.
func (r *YAMLRepository) Save(path string, suite *domain.TestSuiteFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	if suite.Version == "" {
		suite.Version = "1.0"
	}

	var buf strings.Builder
	buf.WriteString("# =============================================================================\n")
	buf.WriteString("# SpecTest Single-File Test Suite\n")
	buf.WriteString("# Generated & Maintained by SpecTest\n")
	buf.WriteString("# =============================================================================\n\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(suite); err != nil {
		return fmt.Errorf("marshal suite yaml: %w", err)
	}
	_ = enc.Close()

	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write suite file %s: %w", path, err)
	}
	return nil
}

// ExportTestMatrix generates a human-readable Markdown coverage matrix (`test_matrix.md`)
// summarizing all scenarios, steps, hybrid test case variations, and assertion rules.
func (r *YAMLRepository) ExportTestMatrix(path string, suite *domain.TestSuiteFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for test matrix: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# SpecTest Coverage Matrix\n\n")
	sb.WriteString(fmt.Sprintf("- **Suite Name:** %s\n", suite.Metadata.Name))
	sb.WriteString(fmt.Sprintf("- **Target Base URL:** `%s`\n", suite.Metadata.TargetBaseURL))
	sb.WriteString(fmt.Sprintf("- **Schema Version:** `%s`\n", suite.Version))
	sb.WriteString(fmt.Sprintf("- **Generated At:** `%s`\n\n", time.Now().Format(time.RFC3339)))

	sb.WriteString("## 1. Scenarios Overview\n\n")
	sb.WriteString("| Scenario ID | Scenario Name | Steps | Test Cases |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: |\n")

	totalSteps := 0
	totalCases := 0
	for _, sc := range suite.Base.Scenarios {
		caseCount := 0
		for _, st := range sc.Steps {
			if len(st.TestCases) > 0 {
				caseCount += len(st.TestCases)
			} else {
				caseCount++
			}
		}
		totalSteps += len(sc.Steps)
		totalCases += caseCount
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %d |\n", sc.ID, sc.Name, len(sc.Steps), caseCount))
	}

	sb.WriteString(fmt.Sprintf("\n> **Total:** `%d` scenarios, `%d` steps, `%d` test case variations.\n\n",
		len(suite.Base.Scenarios), totalSteps, totalCases))

	sb.WriteString("## 2. Test Cases & Assertions\n\n")
	sb.WriteString("| Scenario | Step ID | Method & Endpoint | Case ID | Type | Expected HTTP | Assertions |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :---: | :--- |\n")

	for _, sc := range suite.Base.Scenarios {
		for _, st := range sc.Steps {
			endpointStr := fmt.Sprintf("`%s %s`", st.Request.Method, st.Request.Endpoint)
			if len(st.TestCases) == 0 {
				status := 200
				rules := []string{}
				if st.Assert != nil {
					if st.Assert.Status > 0 {
						status = st.Assert.Status
					}
					if st.Assert.MaxLatencyMs > 0 {
						rules = append(rules, fmt.Sprintf("latency <= %dms", st.Assert.MaxLatencyMs))
					}
					if len(st.Assert.ForbidKeys) > 0 {
						rules = append(rules, fmt.Sprintf("forbid: %s", strings.Join(st.Assert.ForbidKeys, ", ")))
					}
					rules = append(rules, st.Assert.JSONPath...)
				}
				ruleDesc := strings.Join(rules, "; ")
				if ruleDesc == "" {
					ruleDesc = "Status check"
				}
				sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | `DEFAULT` | `step_flow` | `%d` | %s |\n",
					sc.ID, st.ID, endpointStr, status, ruleDesc))
				continue
			}

			for _, tc := range st.TestCases {
				rules := []string{}
				if tc.Assert.SchemaMatch {
					rules = append(rules, "schema_match")
				}
				if len(tc.Assert.ForbidKeys) > 0 {
					rules = append(rules, fmt.Sprintf("forbid: %s", strings.Join(tc.Assert.ForbidKeys, ", ")))
				}
				rules = append(rules, tc.Assert.JSONPath...)
				ruleDesc := strings.Join(rules, "; ")
				if ruleDesc == "" {
					ruleDesc = "Status validation"
				}
				sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | `%s` | `%s` | `%d` | %s |\n",
					sc.ID, st.ID, endpointStr, tc.ID, tc.Type, tc.Assert.Status, ruleDesc))
			}
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

type postmanCollection struct {
	Info struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name    string          `json:"name"`
	Item    []postmanItem   `json:"item,omitempty"`
	Request *postmanRequest `json:"request,omitempty"`
}

type postmanRequest struct {
	Method string          `json:"method"`
	Header []postmanHeader `json:"header,omitempty"`
	Body   *postmanBody    `json:"body,omitempty"`
	URL    postmanURL      `json:"url"`
}

type postmanHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type postmanBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

type postmanURL struct {
	Raw string `json:"raw"`
}

// ExportPostmanCollection converts the scenarios and steps of a TestSuiteFile into
// a Postman Collection v2.1 JSON file and writes it to disk at the given path.
func (r *YAMLRepository) ExportPostmanCollection(path string, suite *domain.TestSuiteFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var col postmanCollection
	col.Info.Name = suite.Metadata.Name
	col.Info.Schema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

	baseURL := strings.TrimRight(suite.Metadata.TargetBaseURL, "/")

	for _, sc := range suite.Base.Scenarios {
		folder := postmanItem{Name: sc.Name}
		for _, st := range sc.Steps {
			var headers []postmanHeader
			for k, v := range st.Request.Headers {
				headers = append(headers, postmanHeader{Key: k, Value: v})
			}
			var body *postmanBody
			payload := st.Request.Body
			if payload == nil && len(st.TestCases) > 0 {
				payload = st.TestCases[0].Payload
			}
			if payload != nil {
				rawBytes, _ := json.MarshalIndent(payload, "", "  ")
				body = &postmanBody{
					Mode: "raw",
					Raw:  string(rawBytes),
				}
			}
			folder.Item = append(folder.Item, postmanItem{
				Name: st.Name,
				Request: &postmanRequest{
					Method: st.Request.Method,
					Header: headers,
					Body:   body,
					URL:    postmanURL{Raw: baseURL + st.Request.Endpoint},
				},
			})
		}
		col.Item = append(col.Item, folder)
	}

	out, err := json.MarshalIndent(col, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
