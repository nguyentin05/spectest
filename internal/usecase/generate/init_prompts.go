package generate

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

//go:embed prompts/*.txt
var initPromptsFS embed.FS

// renderPrompt loads a template from prompts/ and replaces placeholders with values.
func renderPrompt(name string, vars map[string]string) (string, error) {
	data, err := initPromptsFS.ReadFile("prompts/" + name)
	if err != nil {
		return "", fmt.Errorf("read prompt %q: %w", name, err)
	}

	content := string(data)

	// Inject core rules if placeholder exists
	if strings.Contains(content, "<<CORE_RULES>>") {
		coreRules, err := initPromptsFS.ReadFile("prompts/core_rules.txt")
		if err == nil {
			content = strings.ReplaceAll(content, "<<CORE_RULES>>", string(coreRules))
		}
	}

	for k, v := range vars {
		placeholder := "<<" + k + ">>"
		content = strings.ReplaceAll(content, placeholder, v)
	}

	return content, nil
}

func buildContractPrompt(facts map[string]*apiFacts, requirements string) (string, string, error) {
	ops := make([]map[string]any, 0, len(facts))
	for _, f := range facts {
		ops = append(ops, map[string]any{
			"key":                 f.Key,
			"method":              f.Method,
			"path":                f.Path,
			"auth":                f.Auth,
			"parameters":          f.Parameters,
			"has_request_body":    f.HasRequestBody,
			"documented_statuses": f.DocumentedStatuses,
		})
	}

	payloadData, err := json.MarshalIndent(map[string]any{
		"operations":   ops,
		"requirements": requirements,
	}, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode contract payload: %w", err)
	}

	system := "You are a lead QA engineer reconciling OpenAPI specifications with business requirements."
	user, err := renderPrompt("contract.txt", map[string]string{
		"PAYLOAD": string(payloadData),
	})
	if err != nil {
		return "", "", err
	}
	return system, user, nil
}

func buildInitSetupPrompt(facts map[string]*apiFacts, globalNotes []string, requirements string) (string, string, error) {
	keys := make([]string, 0, len(facts))
	for k := range facts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ops := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		f := facts[k]
		ops = append(ops, map[string]any{
			"key":                 f.Key,
			"method":              f.Method,
			"path":                f.Path,
			"auth":                f.Auth,
			"parameters":          f.Parameters,
			"has_request_body":    f.HasRequestBody,
			"request_body_fields": f.RequestBodyFields,
			"response_fields":     f.ResponseFields,
			"documented_statuses": f.DocumentedStatuses,
		})
	}

	payloadData, err := json.MarshalIndent(map[string]any{
		"operations":   ops,
		"global_notes": globalNotes,
		"requirements": requirements,
	}, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode setup payload: %w", err)
	}

	primaryServer := "server"
	if len(facts) > 0 {
		for _, f := range facts {
			if f.Server != "" {
				primaryServer = f.Server
				break
			}
		}
	}

	system := "You are a QA lead designing reusable setup procedures and seed data for an API test suite."
	user, err := renderPrompt("setup.txt", map[string]string{
		"PAYLOAD":      string(payloadData),
		"SERVER_ALIAS": primaryServer,
	})
	if err != nil {
		return "", "", err
	}
	return system, user, nil
}

func buildInitScenarioPrompt(
	targetFact *apiFacts,
	globalNotes []string,
	setups map[string]domain.Setup,
	seed map[string]domain.Seed,
	unresolved []unresolvedItem,
) (string, string, error) {
	var opSchema any
	if targetFact.RawSnippet != "" {
		var parsed any
		if err := json.Unmarshal([]byte(targetFact.RawSnippet), &parsed); err == nil {
			opSchema = parsed
		}
	}
	if opSchema == nil {
		opSchema = map[string]any{
			"method":              targetFact.Method,
			"path":                targetFact.Path,
			"description":         targetFact.Description,
			"parameters":          targetFact.Parameters,
			"auth":                targetFact.Auth,
			"documented_statuses": targetFact.DocumentedStatuses,
		}
	}

	payloadData, err := json.MarshalIndent(map[string]any{
		"target_api":       targetFact.Key,
		"operation_schema": opSchema,
		"global_notes":     globalNotes,
		"seed":             seed,
		"available_setups": setups,
		"unresolved":       unresolved,
	}, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode scenario payload: %w", err)
	}

	serverAlias := targetFact.Server
	if serverAlias == "" {
		serverAlias = "server"
	}

	system := "You are a QA engineer designing test scenarios for a specific API endpoint."
	user, err := renderPrompt("scenarios.txt", map[string]string{
		"PAYLOAD":      string(payloadData),
		"SERVER_ALIAS": serverAlias,
	})
	if err != nil {
		return "", "", err
	}
	return system, user, nil
}

func buildInitAnalysisPrompt(
	outputLang string,
	stats map[string]any,
	seed map[string]domain.Seed,
	setupNames []string,
	unresolved []unresolvedItem,
	skipped []skippedCase,
	noScenario map[string]string,
	inferred []inferredExpect,
	reviewNotes map[string]any,
) (string, string, error) {
	if outputLang == "" {
		outputLang = "English"
	}
	payloadData, err := json.MarshalIndent(map[string]any{
		"output_language":       outputLang,
		"stats":                 stats,
		"seed":                  seed,
		"setup_names":           setupNames,
		"unresolved":            unresolved,
		"skipped_cases":         skipped,
		"no_scenario":           noScenario,
		"inferred_expectations": inferred,
		"review_notes":          reviewNotes,
	}, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode analysis payload: %w", err)
	}

	system := "You are a QA lead writing the narrative analysis section of an API test suite report."
	user, err := renderPrompt("analysis.txt", map[string]string{
		"PAYLOAD": string(payloadData),
	})
	if err != nil {
		return "", "", err
	}
	return system, user, nil
}

