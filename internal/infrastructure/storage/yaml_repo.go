package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/domain"
	"gopkg.in/yaml.v3"
)

// isInsideDir checks whether target is equal to or resides inside the parent directory.
func isInsideDir(parent, target string) bool {
	parent = filepath.Clean(parent)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(parent, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// isWindowsReservedName checks if a file name matches Windows-reserved device names
// (CON, PRN, AUX, NUL, COM1-COM9, LPT1-LPT9), regardless of file extension.
func isWindowsReservedName(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.ToUpper(strings.TrimSuffix(base, ext))
	switch name {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

// sanitizeFilePath validates and cleans a filesystem destination path.
// It cleans relative components and rejects paths attempting to access or overwrite protected system directories
// across Linux, macOS, and Windows environments.
func sanitizeFilePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("file path cannot be empty")
	}
	clean := filepath.Clean(trimmed)
	if clean == "." || clean == string(filepath.Separator) || clean == "/" || clean == "\\" {
		return "", fmt.Errorf("invalid file path %q", path)
	}

	// Check Windows-style drive root (e.g. "C:", "C:\", "D:/") even if running on non-Windows host
	if len(clean) >= 2 && clean[1] == ':' && ((clean[0] >= 'a' && clean[0] <= 'z') || (clean[0] >= 'A' && clean[0] <= 'Z')) {
		rest := clean[2:]
		if rest == "" || rest == "/" || rest == "\\" || rest == string(filepath.Separator) {
			return "", fmt.Errorf("file path cannot be root volume %q", path)
		}
	}

	if vol := filepath.VolumeName(clean); vol != "" {
		if clean == vol || clean == vol+string(filepath.Separator) || clean == vol+"/" || clean == vol+"\\" {
			return "", fmt.Errorf("file path cannot be root volume %q", path)
		}
	}

	if isWindowsReservedName(clean) {
		return "", fmt.Errorf("file path %q uses a reserved device name", path)
	}

	abs, err := filepath.Abs(clean)
	if err == nil {
		var criticalDirs []string
		if runtime.GOOS == "windows" {
			for _, envVar := range []string{"SystemRoot", "windir", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
				if val := os.Getenv(envVar); val != "" {
					criticalDirs = append(criticalDirs, filepath.Clean(val))
				}
			}
			if vol := filepath.VolumeName(abs); vol != "" {
				criticalDirs = append(criticalDirs,
					filepath.Join(vol, "Windows"),
					filepath.Join(vol, "Program Files"),
					filepath.Join(vol, "Program Files (x86)"),
					filepath.Join(vol, "ProgramData"),
				)
			}
		} else {
			criticalDirs = []string{
				"/etc", "/bin", "/sbin", "/usr/bin", "/usr/sbin", "/boot", "/sys", "/proc", "/dev",
			}
			if runtime.GOOS == "darwin" {
				criticalDirs = append(criticalDirs, "/System", "/Library", "/private/etc")
			}
		}

		for _, dir := range criticalDirs {
			if isInsideDir(dir, abs) {
				return "", fmt.Errorf("path %q targets protected system directory %s", path, dir)
			}
		}
	}

	return clean, nil
}

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
	cleanPath, err := sanitizeFilePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid suite path: %w", err)
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read suite file %s: %w", cleanPath, err)
	}
	var suite domain.TestSuiteFile
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("unmarshal suite yaml %s: %w", cleanPath, err)
	}
	if suite.Version == "" {
		suite.Version = "1.0"
	}
	return &suite, nil
}

// Save marshals a domain.TestSuiteFile into formatted 2-space indented YAML with a
// header banner and writes it to disk, creating parent directories if needed.
func (r *YAMLRepository) Save(path string, suite *domain.TestSuiteFile) error {
	cleanPath, err := sanitizeFilePath(path)
	if err != nil {
		return fmt.Errorf("invalid save path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", cleanPath, err)
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

	if err := os.WriteFile(cleanPath, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write suite file %s: %w", cleanPath, err)
	}
	return nil
}

// ExportTestMatrix generates a human-readable Markdown coverage matrix (`test_matrix.md`)
// summarizing all scenarios, steps, hybrid test case variations, and assertion rules.
func (r *YAMLRepository) ExportTestMatrix(path string, suite *domain.TestSuiteFile) error {
	cleanPath, err := sanitizeFilePath(path)
	if err != nil {
		return fmt.Errorf("invalid matrix path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
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

	return os.WriteFile(cleanPath, []byte(sb.String()), 0o644)
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
	cleanPath, err := sanitizeFilePath(path)
	if err != nil {
		return fmt.Errorf("invalid postman path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
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
	return os.WriteFile(cleanPath, out, 0o644)
}
