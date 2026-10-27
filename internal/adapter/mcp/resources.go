package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/nguyentin05/spectest/internal/infrastructure/suitefile"
	ucrun "github.com/nguyentin05/spectest/internal/usecase/run"
)

func (s *Server) registerResources() {
	// 1. spectest://suite
	s.mcpServer.AddResource(
		mcp.NewResource("spectest://suite",
			"Active SpecTest Suite",
			mcp.WithResourceDescription("The current spectest.yaml file content in tests directory"),
			mcp.WithMIMEType("application/x-yaml"),
		),
		s.handleResourceSuite,
	)

	// 2. spectest://history/latest
	s.mcpServer.AddResource(
		mcp.NewResource("spectest://history/latest",
			"Latest Audit History Entry",
			mcp.WithResourceDescription("The latest audit log entry from spectest.log.yaml"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleResourceLatestHistory,
	)

	// 3. spectest://jobs/{job_id} template
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("spectest://jobs/{job_id}",
			"Background Job Status",
			mcp.WithTemplateDescription("Status and progress events of an async execution job"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleResourceJob,
	)

	// 4. spectest://reports/{job_id} template
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate("spectest://reports/{job_id}",
			"Redacted Test Report",
			mcp.WithTemplateDescription("Redacted test execution summary and step assertions"),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleResourceReport,
	)
}

func (s *Server) handleResourceSuite(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	suitePath := filepath.Join(s.resolver.Root(), "tests", "spectest.yaml")
	data, err := os.ReadFile(suitePath)
	if err != nil {
		return nil, fmt.Errorf("suite not found at %s: %w", suitePath, err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "spectest://suite",
			MIMEType: "application/x-yaml",
			Text:     string(data),
		},
	}, nil
}

func (s *Server) handleResourceLatestHistory(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	logPath := filepath.Join(s.resolver.Root(), "tests", "spectest.log.yaml")
	logFile, err := suitefile.LoadLog(logPath)
	if err != nil {
		return nil, fmt.Errorf("audit log not found at %s: %w", logPath, err)
	}

	var latest any
	if len(logFile.Update) > 0 {
		latest = logFile.Update[len(logFile.Update)-1]
	} else {
		latest = logFile.Base
	}

	bytes, err := json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal latest history entry: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "spectest://history/latest",
			MIMEType: "application/json",
			Text:     string(bytes),
		},
	}, nil
}

func (s *Server) handleResourceJob(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid job uri: %s", uri)
	}
	jobID := parts[len(parts)-1]

	job, found := s.jobs.GetJob(jobID)
	if !found {
		return nil, fmt.Errorf("job %q not found", jobID)
	}

	bytes, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal job: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(bytes),
		},
	}, nil
}

func (s *Server) handleResourceReport(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid report uri: %s", uri)
	}
	jobID := parts[len(parts)-1]

	job, found := s.jobs.GetJob(jobID)
	if !found {
		return nil, fmt.Errorf("job %q not found", jobID)
	}

	resultBytes, err := json.Marshal(job.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal job result: %w", err)
	}

	redacted := ucrun.RedactPayload(string(resultBytes))

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     redacted,
		},
	}, nil
}
