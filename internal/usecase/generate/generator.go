// Package generate implements the application usecase for converting OpenAPI specifications
// and Markdown PRD documents into executable `spectest.yaml` test suites and hybrid datasets.
package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nguyentin05/spectest/internal/domain"
	"github.com/nguyentin05/spectest/internal/infrastructure/openapi"
	"github.com/nguyentin05/spectest/internal/infrastructure/prd"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
	"gopkg.in/yaml.v3"
)

// Request holds the input parameters for executing the `spectest generate` workflow.
type Request struct {
	APIPath   string
	ReqPath   string
	OutputDir string
	ExportFmt string
	Enrich    bool
}

// Result contains the generated TestSuiteFile artifact, output file paths, and
// preprocessing telemetry produced by a completed generation run.
type Result struct {
	Suite            *domain.TestSuiteFile
	SuitePath        string
	MatrixPath       string
	ExportPath       string
	CompressionRatio float64
	ChunksProcessed  int
	AuthProducer     string
}

// Generator coordinates the end-to-end test suite generation pipeline across
// OpenAPI preprocessing, PRD slicing, concurrent LLM scenario synthesis, and hybrid data expansion.
type Generator struct {
	llm  domain.LLMService
	repo *storage.YAMLRepository
}

// NewGenerator constructs a Generator wired with the given LLMService and YAMLRepository.
func NewGenerator(llmSvc domain.LLMService, repo *storage.YAMLRepository) *Generator {
	if repo == nil {
		repo = storage.NewYAMLRepository()
	}
	return &Generator{
		llm:  llmSvc,
		repo: repo,
	}
}

// Execute runs the 6-stage test generation pipeline:
//  1. Prune OpenAPI metadata and resolve `$ref` schemas at zero token cost.
//  2. Detect authentication producer and consumer endpoints via security schemes and regex.
//  3. Partition normalized endpoints into domain chunks by OpenAPI tags.
//  4. Slice the optional Markdown PRD and pair matching sections with each domain chunk.
//  5. Invoke the configured LLM concurrently across chunks to synthesize stateful scenarios.
//  6. Augment mutating steps with deterministic Go fuzzing/security cases and write artifacts.
//
// Execute fails fast and returns an error if the LLM service is unconfigured, unreachable,
// or returns invalid scenario JSON.
func (g *Generator) Execute(ctx context.Context, req Request) (*Result, error) {
	if g.llm == nil {
		return nil, fmt.Errorf("llm service is required to generate test scenarios")
	}

	rawSpec, err := os.ReadFile(req.APIPath)
	if err != nil {
		return nil, fmt.Errorf("read openapi file %s: %w", req.APIPath, err)
	}

	pruned, err := openapi.PruneSpec(rawSpec)
	if err != nil {
		return nil, err
	}

	authProducer := ""
	for _, op := range pruned.Operations {
		if op.IsTokenProducer {
			authProducer = fmt.Sprintf("%s %s", op.Method, op.Endpoint)
			break
		}
	}

	chunks := openapi.ChunkByDomain(pruned.Operations, 5)

	prdSections, err := prd.SliceMarkdownFile(req.ReqPath)
	if err != nil {
		return nil, fmt.Errorf("read prd file %s: %w", req.ReqPath, err)
	}
	for i := range chunks {
		chunks[i].PRDSection = prd.MatchSectionToTag(prdSections, chunks[i].Tag)
	}

	scenarios, err := g.buildScenariosConcurrently(ctx, chunks)
	if err != nil {
		return nil, err
	}

	suiteName := pruned.Title
	if suiteName == "" {
		suiteName = "SpecTest Automated Test Suite"
	}

	suite := &domain.TestSuiteFile{
		Version: "1.0",
		Metadata: domain.SuiteMetadata{
			Name:          suiteName,
			TargetBaseURL: pruned.BaseURL,
		},
		Base: domain.BaseSuite{
			Scenarios: scenarios,
		},
	}

	outDir := req.OutputDir
	if strings.TrimSpace(outDir) == "" {
		outDir = "./tests"
	}

	suitePath := filepath.Join(outDir, "spectest.yaml")
	if err := g.repo.Save(suitePath, suite); err != nil {
		return nil, err
	}

	matrixPath := filepath.Join(outDir, "test_matrix.md")
	if err := g.repo.ExportTestMatrix(matrixPath, suite); err != nil {
		return nil, err
	}

	exportPath := ""
	if strings.EqualFold(req.ExportFmt, "postman") {
		exportPath = filepath.Join(outDir, "postman_collection.json")
		if err := g.repo.ExportPostmanCollection(exportPath, suite); err != nil {
			return nil, err
		}
	}

	return &Result{
		Suite:            suite,
		SuitePath:        suitePath,
		MatrixPath:       matrixPath,
		ExportPath:       exportPath,
		CompressionRatio: pruned.CompressionRatio,
		ChunksProcessed:  len(chunks),
		AuthProducer:     authProducer,
	}, nil
}

// buildScenariosConcurrently dispatches one goroutine per DomainChunk to synthesize
// scenarios in parallel and returns the first error encountered if any chunk fails.
func (g *Generator) buildScenariosConcurrently(ctx context.Context, chunks []openapi.DomainChunk) ([]domain.Scenario, error) {
	results := make([]domain.Scenario, len(chunks))
	errs := make([]error, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, ch openapi.DomainChunk) {
			defer wg.Done()
			ops := ch.Operations
			if ch.TokenProducer != nil {
				ops = append([]openapi.NormalizedOperation{*ch.TokenProducer}, ops...)
			}
			sc, err := g.generateChunkScenario(ctx, ch.Tag, ops, ch.PRDSection)
			if err != nil {
				errs[idx] = err
				return
			}
			results[idx] = sc
		}(i, chunk)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// generateChunkScenario prompts the configured LLM with a single domain chunk's pruned
// operations and matched PRD requirements, unmarshals the returned JSON into a domain.Scenario,
// and enriches mutating steps with deterministic Go fuzzing and security test cases.
func (g *Generator) generateChunkScenario(ctx context.Context, tag string, ops []openapi.NormalizedOperation, prdText string) (domain.Scenario, error) {
	opsYAML, err := yaml.Marshal(ops)
	if err != nil {
		return domain.Scenario{}, fmt.Errorf("marshal operations for tag %q: %w", tag, err)
	}

	sysPrompt := `You are SpecTest Test Engineering Engine. Given pruned OpenAPI operations and PRD business rules, output a JSON Scenario object matching domain.Scenario (fields: id, name, steps). Ensure token producers extract user_token and consumers include Authorization: Bearer {{user_token}}.`
	userPrompt := fmt.Sprintf("Domain Tag: %s\n\nPruned OpenAPI Operations:\n%s\n\nPRD Business Requirements:\n%s", tag, string(opsYAML), prdText)

	rawJSON, err := g.llm.GenerateStructured(ctx, sysPrompt, userPrompt, nil)
	if err != nil {
		return domain.Scenario{}, fmt.Errorf("llm generation failed for domain %q: %w", tag, err)
	}
	if strings.TrimSpace(rawJSON) == "" {
		return domain.Scenario{}, fmt.Errorf("llm returned empty response for domain %q", tag)
	}

	var sc domain.Scenario
	if err := json.Unmarshal([]byte(rawJSON), &sc); err != nil {
		return domain.Scenario{}, fmt.Errorf("parse llm scenario json for domain %q: %w", tag, err)
	}
	if len(sc.Steps) == 0 {
		return domain.Scenario{}, fmt.Errorf("llm scenario for domain %q contains no steps", tag)
	}

	for i := range sc.Steps {
		if matchedOp, found := findMatchingOp(sc.Steps[i], ops); found {
			EnrichStepWithHybridData(&sc.Steps[i], matchedOp)
		}
	}
	return sc, nil
}

// findMatchingOp locates the NormalizedOperation corresponding to a generated Step
// by comparing TargetRef or Request method and endpoint paths.
func findMatchingOp(step domain.Step, ops []openapi.NormalizedOperation) (openapi.NormalizedOperation, bool) {
	for _, op := range ops {
		if step.TargetRef != nil && strings.EqualFold(step.TargetRef.Endpoint, op.Endpoint) && strings.EqualFold(step.TargetRef.Method, op.Method) {
			return op, true
		}
		if strings.EqualFold(step.Request.Endpoint, op.Endpoint) && strings.EqualFold(step.Request.Method, op.Method) {
			return op, true
		}
	}
	return openapi.NormalizedOperation{}, false
}
