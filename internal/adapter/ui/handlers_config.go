package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nguyentin05/spectest/internal/infrastructure/llm"
	"github.com/nguyentin05/spectest/internal/infrastructure/storage"
)

type configResponse struct {
	ConfigFile      string `json:"config_file"`
	DefaultProvider string `json:"default_provider"`
	ActiveModel     string `json:"active_model"`
	EndpointURL     string `json:"endpoint_url"`
	APIKeyMasked    string `json:"api_key_masked"`
	HasAPIKey       bool   `json:"has_api_key"`
}

type configUpdateRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
}

type testLLMRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
}

// handleGetConfig returns the active credentials and configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := s.factory.CredentialStore()
	prov := store.GetDefaultProvider()
	key, _ := store.GetAPIKey(prov)
	ep, _ := store.GetEndpoint(prov)
	model := store.GetDefaultModel(prov)

	resp := configResponse{
		ConfigFile:      store.GlobalPath(),
		DefaultProvider: prov,
		ActiveModel:     model,
		EndpointURL:     ep,
		APIKeyMasked:    storage.MaskKey(key),
		HasAPIKey:       strings.TrimSpace(key) != "",
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleUpdateConfig updates active credentials.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req configUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	store := s.factory.CredentialStore()
	prov := strings.ToLower(strings.TrimSpace(req.Provider))
	if prov == "" {
		prov = store.GetDefaultProvider()
	}
	if prov == "" {
		writeError(w, http.StatusBadRequest, "provider name is required")
		return
	}

	if err := store.SetDefaultProvider(prov); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save default provider: "+err.Error())
		return
	}

	if req.Endpoint != "" {
		if err := store.SetEndpoint(prov, strings.TrimSpace(req.Endpoint)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save endpoint: "+err.Error())
			return
		}
	}

	if req.Model != "" {
		if err := store.SetDefaultModel(prov, strings.TrimSpace(req.Model)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save model: "+err.Error())
			return
		}
	}

	if req.APIKey != "" {
		if err := store.SetAPIKey(prov, strings.TrimSpace(req.APIKey)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save API key: "+err.Error())
			return
		}
	}

	s.sessionMgr.AppendLog("Configuration updated for provider: " + prov)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Configuration saved successfully",
	})
}

// handleTestLLM tests connectivity to the requested LLM provider.
func (s *Server) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req testLLMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	store := s.factory.CredentialStore()
	prov := strings.ToLower(strings.TrimSpace(req.Provider))
	if prov == "" {
		prov = store.GetDefaultProvider()
	}

	key := strings.TrimSpace(req.APIKey)
	if key == "" {
		key, _ = store.GetAPIKey(prov)
	}

	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		endpoint, _ = store.GetEndpoint(prov)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = store.GetDefaultModel(prov)
	}

	if prov == "" {
		writeError(w, http.StatusBadRequest, "no LLM provider specified")
		return
	}

	// Cloud endpoints require an API key; local model endpoints (localhost / 127.0.0.1) do not
	if !isLocalEndpoint(endpoint) && key == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "API key is required for cloud provider " + prov,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	client := llm.NewClient(prov, endpoint, key, model)
	// Send lightweight ping prompt
	_, err := client.GenerateStructured(ctx, `Reply with a JSON object containing a status field set to "ok".`, "ping", nil)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "LLM connection check failed: " + err.Error(),
		})
		return
	}

	s.sessionMgr.AppendLog("LLM connectivity verified successfully for: " + prov)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"provider": prov,
		"model":    model,
		"message":  "Connection to LLM provider succeeded",
	})
}

func isLocalEndpoint(endpoint string) bool {
	ep := strings.ToLower(strings.TrimSpace(endpoint))
	return strings.Contains(ep, "localhost") ||
		strings.Contains(ep, "127.0.0.1") ||
		strings.Contains(ep, "0.0.0.0") ||
		strings.HasPrefix(ep, "http://192.168.") ||
		strings.HasPrefix(ep, "http://10.")
}
