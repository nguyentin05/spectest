package run

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nguyentin05/spectest/internal/domain"
)

type mockSuiteRepo struct {
	suite *domain.TestSuiteFile
	err   error
}

func (m *mockSuiteRepo) Load(path string) (*domain.TestSuiteFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.suite, nil
}

func (m *mockSuiteRepo) Save(path string, suite *domain.TestSuiteFile) error {
	return nil
}

type mockLifecycleManager struct {
	upErr       error
	teardownFn  func()
	teardownRan *atomic.Bool
}

func (m *mockLifecycleManager) Up(ctx context.Context) (func(), error) {
	if m.upErr != nil {
		return nil, m.upErr
	}
	return func() {
		if m.teardownRan != nil {
			m.teardownRan.Store(true)
		}
		if m.teardownFn != nil {
			m.teardownFn()
		}
	}, nil
}

func sampleSuite() *domain.TestSuiteFile {
	return &domain.TestSuiteFile{
		Version: "1.0",
		Metadata: domain.SuiteMetadata{
			Name:          "Order API Suite",
			TargetBaseURL: "http://localhost:8080",
		},
		Base: domain.BaseSuite{
			Scenarios: []domain.Scenario{
				{
					ID:   "SCN_1",
					Name: "Checkout flow",
					Steps: []domain.Step{
						{
							ID:   "STEP_1",
							Name: "Create order",
							TestCases: []domain.TestCase{
								{ID: "TC_1", Name: "Valid payload"},
								{ID: "TC_2", Name: "Boundary payload"},
							},
						},
					},
				},
			},
		},
	}
}

func TestRunner_Execute_Success(t *testing.T) {
	repo := &mockSuiteRepo{suite: sampleSuite()}
	runner := NewRunner(repo, nil)

	var teardownCalled atomic.Bool
	runner.SetLifecycleFactory(func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
		return &mockLifecycleManager{
			teardownRan: &teardownCalled,
		}
	})

	summary, err := runner.Execute(context.Background(), Options{
		SuitePath: "spectest.yaml",
		TargetURL: "http://localhost:9090",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.SuiteName != "Order API Suite" {
		t.Errorf("SuiteName = %q, want 'Order API Suite'", summary.SuiteName)
	}
	if summary.TargetURL != "http://localhost:9090" {
		t.Errorf("TargetURL = %q, want 'http://localhost:9090'", summary.TargetURL)
	}
	if summary.TotalChecks != 2 {
		t.Errorf("TotalChecks = %d, want 2", summary.TotalChecks)
	}
	if !teardownCalled.Load() {
		t.Error("expected lifecycle teardown to be called via defer")
	}
}

func TestRunner_Execute_MissingTargetURL(t *testing.T) {
	suite := sampleSuite()
	suite.Metadata.TargetBaseURL = "" // empty target in suite
	repo := &mockSuiteRepo{suite: suite}
	runner := NewRunner(repo, nil)

	_, err := runner.Execute(context.Background(), Options{
		SuitePath: "spectest.yaml",
		TargetURL: "", // also empty in options
	})
	if err == nil {
		t.Fatal("expected error when target URL is completely missing")
	}
	if !strings.Contains(err.Error(), "target URL is required") {
		t.Errorf("expected 'target URL is required' error message, got: %v", err)
	}
}

func TestRunner_Execute_AutoPrefixesScheme(t *testing.T) {
	repo := &mockSuiteRepo{suite: sampleSuite()}
	runner := NewRunner(repo, nil)

	var gotTargetURL string
	runner.SetLifecycleFactory(func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
		gotTargetURL = targetURL
		return &mockLifecycleManager{}
	})

	summary, err := runner.Execute(context.Background(), Options{
		SuitePath: "spectest.yaml",
		TargetURL: "localhost:7777", // no http://
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotTargetURL != "http://localhost:7777" {
		t.Errorf("lifecycle manager received targetURL = %q, want 'http://localhost:7777'", gotTargetURL)
	}
	if summary.TargetURL != "http://localhost:7777" {
		t.Errorf("summary TargetURL = %q, want 'http://localhost:7777'", summary.TargetURL)
	}
}

func TestRunner_Execute_SuiteLoadError(t *testing.T) {
	repo := &mockSuiteRepo{err: errors.New("file not found")}
	runner := NewRunner(repo, nil)

	_, err := runner.Execute(context.Background(), Options{
		SuitePath: "missing.yaml",
	})
	if err == nil {
		t.Fatal("expected error for missing suite file")
	}
}

func TestRunner_Execute_LifecycleError(t *testing.T) {
	repo := &mockSuiteRepo{suite: sampleSuite()}
	runner := NewRunner(repo, nil)

	runner.SetLifecycleFactory(func(composeFile, targetURL string, timeoutSec int) domain.ServerLifecycleManager {
		return &mockLifecycleManager{
			upErr: errors.New("docker daemon not running"),
		}
	})

	_, err := runner.Execute(context.Background(), Options{
		SuitePath:   "spectest.yaml",
		ComposeFile: "docker-compose.yml",
	})
	if err == nil {
		t.Fatal("expected error when server lifecycle setup fails")
	}
}
