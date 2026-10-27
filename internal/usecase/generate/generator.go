package generate

import (
	"context"
	"fmt"

	"github.com/nguyentin05/spectest/internal/domain"
)

// Generator orchestrates the v3 generate pipeline.
type Generator struct {
	llm     domain.LLMService
	sources domain.SourceLoader
	store   domain.SuiteStore
	clock   domain.Clock
}

func NewGenerator(
	llm domain.LLMService,
	sources domain.SourceLoader,
	store domain.SuiteStore,
	clock domain.Clock,
) *Generator {
	return &Generator{
		llm:     llm,
		sources: sources,
		store:   store,
		clock:   clock,
	}
}

// Request is the input to Execute.
type Request struct {
	Mode             Mode
	APIs             []domain.APIInput
	RequirementsPath string
	OutputDir        string
	DryRun           bool
	Concurrency      int
	Progress         func(current, total int, message string)
}

// Result is what the CLI prints.
type Result struct {
	Mode           Mode
	UpdateN        int
	SuiteChanged   bool
	APIDiff        APIDiff
	Impact         domain.UpdateImpact
	PlannedActions []UpdateAction
	LLMUsed        bool
	Input          domain.InputMode
	Servers        []string
	APITotal       int
	Warnings       []string
	SuitePath      string
	LogPath        string
	NoFilesWritten bool
}

// Execute dispatches to the requested mode pipeline.
func (g *Generator) Execute(ctx context.Context, req Request) (*Result, error) {
	mode := req.Mode
	if mode == "" {
		mode = ModeInitial
	}

	switch mode {
	case ModeInitial:
		return g.executeInitial(ctx, req)
	case ModeUpdate:
		return g.executeUpdate(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported generate mode %q", mode)
	}
}
