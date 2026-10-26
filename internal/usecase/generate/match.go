package generate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nguyentin05/spectest/internal/domain"
)

// parsedCall is the decomposed form of a step's `call` field.
//
//	GET {auth}/api/users/{id}?include=profile
//	POST {order}/api/orders
//	DELETE {auth}/api/sessions/{{sessionId}}
type parsedCall struct {
	Method   string
	Server   string
	Path     string
	RawQuery string
}

// parseCall splits a raw call string into its components.
func parseCall(raw string) (parsedCall, error) {
	var p parsedCall

	s := strings.TrimSpace(raw)
	if s == "" {
		return p, fmt.Errorf("call is empty")
	}

	// method
	sp := strings.IndexByte(s, ' ')
	if sp <= 0 {
		return p, fmt.Errorf("call %q missing method", raw)
	}
	p.Method = strings.ToUpper(s[:sp])
	rest := strings.TrimLeft(s[sp+1:], " ")
	if rest == "" {
		return p, fmt.Errorf("call %q missing target", raw)
	}

	// server
	if !strings.HasPrefix(rest, "{") {
		return p, fmt.Errorf("call %q target must start with {server}", raw)
	}
	closeBrace := strings.IndexByte(rest, '}')
	if closeBrace <= 1 {
		return p, fmt.Errorf("call %q has empty or unterminated server", raw)
	}
	p.Server = rest[1:closeBrace]
	rest = rest[closeBrace+1:]

	// path + query
	if !strings.HasPrefix(rest, "/") {
		return p, fmt.Errorf("call %q path must start with /", raw)
	}
	if before, after, ok := strings.Cut(rest, "?"); ok {
		p.Path = before
		p.RawQuery = after
	} else {
		p.Path = rest
	}
	return p, nil
}

// splitCallPath splits a path into segments.
//
//	"/"            → nil
//	"/api/orders"  → ["api", "orders"]
//	"/api//orders" → ["api", "", "orders"]
func splitCallPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	p := strings.TrimPrefix(path, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func hasEmptySegment(segs []string) bool {
	return slices.Contains(segs, "")
}

// isTemplateSegment reports whether seg looks like `{name}` (OpenAPI path
// parameter). NOT `{{name}}` (call template).
func isTemplateSegment(seg string) bool {
	return len(seg) >= 3 && seg[0] == '{' && seg[len(seg)-1] == '}' &&
		!strings.HasPrefix(seg, "{{")
}

// matchSegments scores how well apiSegs matches callSegs.
//
//	literal API segment matching call segment → +2
//	template API segment matching call segment → +1
func matchSegments(apiSegs, callSegs []string) (int, bool) {
	if len(apiSegs) != len(callSegs) {
		return 0, false
	}
	score := 0
	for i, ap := range apiSegs {
		cs := callSegs[i]
		if cs == "" {
			return 0, false
		}
		if isTemplateSegment(ap) {
			score++
			continue
		}
		if ap != cs {
			return 0, false
		}
		score += 2
	}
	return score, true
}

// matchAPI finds the API whose method + server + path template matches `call`.
//
// Rules:
//   - Query in `call` is stripped before matching; API.Path never contains `?`.
//   - Literal segment wins over `{param}` template segment.
//   - Two candidates with identical specificity → ambiguous error.
//   - No candidate → (zero, false, nil).
//   - Invalid call syntax → (zero, false, err).
func matchAPI(apis map[string]domain.API, call string) (domain.API, bool, error) {
	pc, err := parseCall(call)
	if err != nil {
		return domain.API{}, false, err
	}

	callSegs := splitCallPath(pc.Path)
	if hasEmptySegment(callSegs) {
		return domain.API{}, false, fmt.Errorf("call %q contains empty path segment", call)
	}

	var (
		best   domain.API
		bestSc = -1
		tie    = false
	)

	for _, api := range apis {
		if !strings.EqualFold(api.Method, pc.Method) {
			continue
		}
		if api.Server != pc.Server {
			continue
		}
		sc, ok := matchSegments(splitCallPath(api.Path), callSegs)
		if !ok {
			continue
		}
		switch {
		case sc > bestSc:
			best, bestSc, tie = api, sc, false
		case sc == bestSc:
			tie = true
		}
	}

	if bestSc < 0 {
		return domain.API{}, false, nil
	}
	if tie {
		return domain.API{}, false, fmt.Errorf(
			"call %q is ambiguous: multiple APIs match with equal specificity", call)
	}
	return best, true, nil
}
