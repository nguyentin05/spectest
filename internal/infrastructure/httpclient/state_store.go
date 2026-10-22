package httpclient

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var templateVarRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// StateStore manages shared variables across test steps safely in concurrent scenarios.
type StateStore struct {
	mu   sync.RWMutex
	vars map[string]string
}

// NewStateStore creates a new StateStore initialized with optional environment variables.
func NewStateStore(initial map[string]string) *StateStore {
	m := make(map[string]string)
	for k, v := range initial {
		m[k] = v
	}
	return &StateStore{vars: m}
}

// Set stores a variable in the state store.
func (s *StateStore) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vars[key] = value
}

// Get retrieves a variable from the state store.
func (s *StateStore) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vars[key]
	return v, ok
}

// Snapshot returns a copy of all variables currently stored.
func (s *StateStore) Snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]string, len(s.vars))
	for k, v := range s.vars {
		cp[k] = v
	}
	return cp
}

// InterpolateString replaces all {{var}} placeholders in a string with values from state.
func InterpolateString(input string, state map[string]string) string {
	if len(state) == 0 || !strings.Contains(input, "{{") {
		return input
	}
	return templateVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		sub := templateVarRegex.FindStringSubmatch(match)
		if len(sub) == 2 {
			if val, exists := state[sub[1]]; exists {
				return val
			}
		}
		return match
	})
}

// InterpolatePayload recursively replaces {{var}} placeholders inside maps, slices, and strings.
func InterpolatePayload(payload interface{}, state map[string]string) interface{} {
	if payload == nil || len(state) == 0 {
		return payload
	}
	switch v := payload.(type) {
	case string:
		return InterpolateString(v, state)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			out[k] = InterpolatePayload(val, state)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			out[fmt.Sprint(k)] = InterpolatePayload(val, state)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, val := range v {
			out[i] = InterpolatePayload(val, state)
		}
		return out
	default:
		return payload
	}
}

// QueryJSONPath extracts a value from a decoded JSON object or raw bytes using dot notation
// (e.g. "$.data.access_token", "$.data.user.id", "$.items[0].id").
func QueryJSONPath(rawJSON []byte, pathExpr string) (interface{}, bool) {
	if len(rawJSON) == 0 {
		return nil, false
	}
	var root interface{}
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return nil, false
	}
	return QueryJSONObject(root, pathExpr)
}

// QueryJSONObject traverses an unmarshaled JSON object using a JSONPath expression.
func QueryJSONObject(root interface{}, pathExpr string) (interface{}, bool) {
	clean := strings.TrimSpace(pathExpr)
	clean = strings.TrimPrefix(clean, "$.")
	clean = strings.TrimPrefix(clean, "$")
	if clean == "" {
		return root, true
	}

	// Normalize array indexing like items[0] -> items.0
	clean = strings.ReplaceAll(clean, "[", ".")
	clean = strings.ReplaceAll(clean, "]", "")

	segments := strings.Split(clean, ".")
	curr := root

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		switch node := curr.(type) {
		case map[string]interface{}:
			val, exists := node[seg]
			if !exists {
				return nil, false
			}
			curr = val
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			curr = node[idx]
		default:
			return nil, false
		}
	}

	return curr, true
}

// FormatExtractedValue converts an extracted JSON value into a string suitable for StateStore.
func FormatExtractedValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprint(val)
		}
		return string(b)
	}
}
