package run

import (
	"encoding/json"
	"net/url"
	"strings"
)

// redactPayload parses a JSON string (object or array) and masks sensitive fields.
func redactPayload(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return raw
	}
	redactValue(data)
	redacted, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return string(redacted)
}

func redactValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if isSensitiveKey(k) {
				val[k] = "[REDACTED]"
			} else {
				redactValue(child)
			}
		}
	case []any:
		for _, item := range val {
			redactValue(item)
		}
	}
}

func isSensitiveKey(key string) bool {
	clean := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(key), "-", ""), "_", "")
	if clean == "password" || clean == "passwd" || clean == "pass" || clean == "secret" ||
		clean == "token" || clean == "accesstoken" || clean == "refreshtoken" || clean == "idtoken" ||
		clean == "jwt" || clean == "bearer" ||
		clean == "apikey" || clean == "clientsecret" || clean == "credential" || clean == "credentials" ||
		clean == "privatekey" || clean == "auth" || clean == "authorization" ||
		clean == "session" || clean == "sessionid" {
		return true
	}
	if strings.Contains(clean, "password") || strings.Contains(clean, "secret") || strings.Contains(clean, "token") || strings.Contains(clean, "apikey") || strings.Contains(clean, "jwt") {
		return true
	}
	return false
}

func redactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveHeader(k) {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func isSensitiveHeader(header string) bool {
	lower := strings.ToLower(header)
	return strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "api-key") ||
		strings.Contains(lower, "cookie") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "jwt")
}

// redactURL parses an URL string and masks userinfo passwords and sensitive query parameters.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// 1. Redact userinfo password and sensitive username
	if u.User != nil {
		username := u.User.Username()
		if isSensitiveKey(username) {
			username = "[REDACTED]"
		}
		if _, hasPass := u.User.Password(); hasPass {
			u.User = url.UserPassword(username, "[REDACTED]")
		} else if username != u.User.Username() {
			u.User = url.User(username)
		}
	}

	// 2. Redact sensitive query parameters
	if u.RawQuery != "" {
		q := u.Query()
		modified := false
		for k := range q {
			if isSensitiveKey(k) {
				q[k] = []string{"[REDACTED]"}
				modified = true
			}
		}
		if modified {
			encoded := q.Encode()
			u.RawQuery = strings.ReplaceAll(encoded, "%5BREDACTED%5D", "[REDACTED]")
		}
	}

	res := u.String()
	return strings.ReplaceAll(res, "%5BREDACTED%5D", "[REDACTED]")
}

func redactExtracted(extracted map[string]any) map[string]any {
	if extracted == nil {
		return nil
	}
	res := make(map[string]any, len(extracted))
	for k, v := range extracted {
		if isSensitiveKey(k) {
			res[k] = "[REDACTED]"
		} else {
			res[k] = v
		}
	}
	return res
}
