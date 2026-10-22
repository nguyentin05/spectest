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

// Request encapsulates the parameters for the `spectest generate` usecase.
type Request struct {
	APIPath   string
	ReqPath   string
	OutputDir string
	ExportFmt string
	Enrich    bool
}

// Result contains the generated suite and telemetry from the generation pipeline.
type Result struct {
	Suite            *domain.TestSuiteFile
	SuitePath        string
	MatrixPath       string
	ExportPath       string
	CompressionRatio float64
	ChunksProcessed  int
	AuthProducer     string
}

// Generator orchestrates the test suite generation pipeline.
type Generator struct {
	llm  domain.LLMService
	repo *storage.YAMLRepository
}

// NewGenerator creates a new Generator usecase instance.
func NewGenerator(llmSvc domain.LLMService, repo *storage.YAMLRepository) *Generator {
	if repo == nil {
		repo = storage.NewYAMLRepository()
	}
	return &Generator{
		llm:  llmSvc,
		repo: repo,
	}
}

// Execute runs the generation pipeline:
// 1. Spec Pruning (0 tokens)
// 2. Zero-Token Auth Detection (0 tokens)
// 3. Domain Chunking by OpenAPI tags
// 4. PRD Slicing by Markdown ## headings
// 5. Concurrent Single-Shot Goroutine LLM generation
// 6. Hybrid Data expansion (LLM Business + Go Fuzzing/Security) & file export
func (g *Generator) Execute(ctx context.Context, req Request) (*Result, error) {
	rawSpec, err := os.ReadFile(req.APIPath)
	if err != nil {
		return nil, fmt.Errorf("read openapi file %s: %w", req.APIPath, err)
	}

	// Step 1 & 2: Prune Spec & Zero-Token Auth Detection
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

	// Step 3: Domain Chunking
	chunks := openapi.ChunkByDomain(pruned.Operations, 5)

	// Step 4: PRD Slicing & Pairing
	prdSections, _ := prd.SliceMarkdownFile(req.ReqPath)
	for i := range chunks {
		chunks[i].PRDSection = prd.MatchSectionToTag(prdSections, chunks[i].Tag)
	}

	// Step 5 & 6: Concurrent Single-Shot Goroutines + Hybrid Data Augmentation
	scenarios := g.buildScenariosConcurrently(ctx, chunks)

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
		_ = g.repo.ExportPostmanCollection(exportPath, suite)
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

func (g *Generator) buildScenariosConcurrently(ctx context.Context, chunks []openapi.DomainChunk) []domain.Scenario {
	results := make([]domain.Scenario, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, ch openapi.DomainChunk) {
			defer wg.Done()
			ops := ch.Operations
			if ch.TokenProducer != nil {
				ops = append([]openapi.NormalizedOperation{*ch.TokenProducer}, ops...)
			}
			results[idx] = g.synthesizeChunkScenario(ctx, idx, ch.Tag, ops, ch.PRDSection)
		}(i, chunk)
	}
	wg.Wait()
	return results
}

func (g *Generator) synthesizeChunkScenario(ctx context.Context, idx int, tag string, ops []openapi.NormalizedOperation, prdText string) domain.Scenario {
	if g.llm != nil {
		if sc, ok := g.tryLLMScenario(ctx, tag, ops, prdText); ok {
			for i := range sc.Steps {
				if matchedOp, found := findMatchingOp(sc.Steps[i], ops); found {
					EnrichStepWithHybridData(&sc.Steps[i], matchedOp)
				}
			}
			return sc
		}
	}

	sc := domain.Scenario{
		ID:   fmt.Sprintf("SCENARIO_%s_%02d", strings.ToUpper(tag), idx+1),
		Name: fmt.Sprintf("%s workflow scenario", tag),
	}
	for i, op := range ops {
		step := buildSchemaDrivenStep(i+1, op)
		EnrichStepWithHybridData(&step, op)
		sc.Steps = append(sc.Steps, step)
	}
	return sc
}

func (g *Generator) tryLLMScenario(ctx context.Context, tag string, ops []openapi.NormalizedOperation, prdText string) (domain.Scenario, bool) {
	opsYAML, _ := yaml.Marshal(ops)
	sysPrompt := `You are SpecTest Test Engineering Engine. Given pruned OpenAPI operations and PRD business rules, output a JSON Scenario object matching domain.Scenario (fields: id, name, steps). Ensure token producers extract user_token and consumers include Authorization: Bearer {{user_token}}.`
	userPrompt := fmt.Sprintf("Domain Tag: %s\n\nPruned OpenAPI Operations:\n%s\n\nPRD Business Requirements:\n%s", tag, string(opsYAML), prdText)

	rawJSON, err := g.llm.GenerateStructured(ctx, sysPrompt, userPrompt, nil)
	if err != nil || strings.TrimSpace(rawJSON) == "" {
		return domain.Scenario{}, false
	}

	var sc domain.Scenario
	if err := json.Unmarshal([]byte(rawJSON), &sc); err != nil || len(sc.Steps) == 0 {
		return domain.Scenario{}, false
	}
	return sc, true
}

func buildSchemaDrivenStep(stepNum int, op openapi.NormalizedOperation) domain.Step {
	stepName := op.Summary
	if stepName == "" {
		stepName = fmt.Sprintf("%s %s", op.Method, op.Endpoint)
	}

	headers := map[string]string{}
	if op.RequiresAuth {
		headers["Authorization"] = "Bearer {{user_token}}"
	}

	var body interface{}
	if op.RequestSchema != nil && len(op.RequestSchema.Properties) > 0 {
		headers["Content-Type"] = "application/json"
		body = openapi.SynthesizeSamplePayload(*op.RequestSchema)
	}

	step := domain.Step{
		ID:   fmt.Sprintf("STEP_%02d_%s", stepNum, strings.ToUpper(op.OperationID)),
		Name: stepName,
		TargetRef: &domain.TargetRef{
			Endpoint:    op.Endpoint,
			Method:      op.Method,
			OperationID: op.OperationID,
		},
		Request: domain.RequestDef{
			Method:   op.Method,
			Endpoint: op.Endpoint,
			Headers:  headers,
			Body:     body,
		},
		Assert: &domain.AssertionRule{
			Status: 200,
		},
	}

	if op.IsTokenProducer {
		step.Extract = map[string]string{
			"user_token": "$.data.access_token",
		}
		step.Assert.ForbidKeys = []string{"password", "password_hash", "salt"}
	}

	return step
}

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
