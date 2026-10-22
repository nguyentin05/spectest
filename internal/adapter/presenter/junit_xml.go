package presenter

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nguyentin05/spectest/internal/usecase/run"
)

type junitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Time     string           `xml:"time,attr"`
	Suites   []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// ExportJUnitXML writes the test execution summary to a standard JUnit XML file (e.g. reports/junit.xml).
func ExportJUnitXML(path string, sum *run.Summary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create junit report dir: %w", err)
	}

	suiteMap := make(map[string]*junitTestSuite)
	var suiteOrder []string

	for _, rec := range sum.Records {
		ts, exists := suiteMap[rec.ScenarioID]
		if !exists {
			ts = &junitTestSuite{
				Name: fmt.Sprintf("%s: %s", rec.ScenarioID, rec.ScenarioName),
			}
			suiteMap[rec.ScenarioID] = ts
			suiteOrder = append(suiteOrder, rec.ScenarioID)
		}

		ts.Tests++
		tc := junitTestCase{
			Name:      fmt.Sprintf("[%s] %s - %s (%s %s)", rec.StepID, rec.CaseID, rec.CaseName, rec.Method, rec.Endpoint),
			ClassName: rec.ScenarioID,
			Time:      fmt.Sprintf("%.3f", float64(rec.LatencyMs)/1000.0),
		}

		if !rec.Passed {
			ts.Failures++
			body := fmt.Sprintf("Failed Tier: %s\nReason: %s", rec.FailedTier, rec.FailReason)
			if rec.AIDiagnosis != "" {
				body += "\n\nAI Root-Cause Diagnosis:\n" + rec.AIDiagnosis
			}
			tc.Failure = &junitFailure{
				Message: rec.FailReason,
				Type:    rec.FailedTier,
				Content: body,
			}
		}

		ts.TestCases = append(ts.TestCases, tc)
	}

	root := junitTestSuites{
		Name:     sum.SuiteName,
		Tests:    sum.TotalChecks,
		Failures: sum.FailedChecks,
		Time:     fmt.Sprintf("%.3f", sum.TotalDuration.Seconds()),
	}
	for _, id := range suiteOrder {
		root.Suites = append(root.Suites, *suiteMap[id])
	}

	out, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal junit xml: %w", err)
	}

	header := []byte(xml.Header)
	return os.WriteFile(path, append(header, out...), 0o644)
}
