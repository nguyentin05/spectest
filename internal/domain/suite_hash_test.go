package domain

import (
	"testing"
)

func TestSuiteHash_DerivedFieldsIgnored(t *testing.T) {
	suiteA := &Suite{
		Version:  1,
		Input:    InputOpenAPI,
		Servers:  []string{"auth"},
		APIs:     []string{"POST {auth}/api/login"},
		Analysis: "User auth flow",
		Scenarios: []Scenario{
			{ID: "sc-1", Name: "Scenario 1", Steps: []Step{{Call: "POST {auth}/login", Expect: 200}}},
		},
	}

	suiteB := &Suite{
		Version:  99,
		Input:    InputBoth,
		Servers:  []string{"completely", "different", "servers"},
		APIs:     []string{"GET /other/endpoint"},
		Analysis: "User auth flow",
		Scenarios: []Scenario{
			{ID: "sc-1", Name: "Scenario 1", Steps: []Step{{Call: "POST {auth}/login", Expect: 200}}},
		},
	}

	hashA := SuiteHash(suiteA)
	hashB := SuiteHash(suiteB)

	if hashA != hashB {
		t.Errorf("SuiteHash mismatch on derived fields: hashA = %q, hashB = %q", hashA, hashB)
	}
	if len(hashA) != 12 {
		t.Errorf("expected 12-char hex hash, got len = %d (%q)", len(hashA), hashA)
	}
}

func TestSuiteHash_ScenarioOrderInvariance(t *testing.T) {
	sc1 := Scenario{ID: "sc-alpha", Name: "Alpha", Steps: []Step{{Call: "GET /alpha", Expect: 200}}}
	sc2 := Scenario{ID: "sc-beta", Name: "Beta", Steps: []Step{{Call: "GET /beta", Expect: 200}}}
	sc3 := Scenario{ID: "sc-gamma", Name: "Gamma", Steps: []Step{{Call: "GET /gamma", Expect: 200}}}

	suiteOrder1 := &Suite{
		Analysis:  "Test suite",
		Scenarios: []Scenario{sc3, sc1, sc2},
	}
	suiteOrder2 := &Suite{
		Analysis:  "Test suite",
		Scenarios: []Scenario{sc1, sc2, sc3},
	}
	suiteOrder3 := &Suite{
		Analysis:  "Test suite",
		Scenarios: []Scenario{sc2, sc3, sc1},
	}

	hash1 := SuiteHash(suiteOrder1)
	hash2 := SuiteHash(suiteOrder2)
	hash3 := SuiteHash(suiteOrder3)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("SuiteHash must be invariant to scenario ordering: hash1=%q, hash2=%q, hash3=%q", hash1, hash2, hash3)
	}
}

func TestSuiteHash_SetupUseOrderInvariance(t *testing.T) {
	suiteA := &Suite{
		Setup: map[string]Setup{
			"init": {
				Use:   []string{"zebra", "alpha", "middle"},
				Steps: []Step{{Call: "POST /init", Expect: 200}},
			},
		},
		Scenarios: []Scenario{},
	}

	suiteB := &Suite{
		Setup: map[string]Setup{
			"init": {
				Use:   []string{"alpha", "middle", "zebra"},
				Steps: []Step{{Call: "POST /init", Expect: 200}},
			},
		},
		Scenarios: []Scenario{},
	}

	hashA := SuiteHash(suiteA)
	hashB := SuiteHash(suiteB)

	if hashA != hashB {
		t.Errorf("SuiteHash must be invariant to setup Use slice ordering: hashA=%q, hashB=%q", hashA, hashB)
	}
}

func TestSuiteHash_AnalysisCRLFAndTrailingWhitespace(t *testing.T) {
	rawCRLF := "Analysis summary.\r\nLine 2 with trailing spaces.   \r\n\r\n\r\n"
	rawLF := "Analysis summary.\nLine 2 with trailing spaces.\n"

	suiteCRLF := &Suite{
		Analysis:  rawCRLF,
		Scenarios: []Scenario{},
	}
	suiteLF := &Suite{
		Analysis:  rawLF,
		Scenarios: []Scenario{},
	}

	hashCRLF := SuiteHash(suiteCRLF)
	hashLF := SuiteHash(suiteLF)

	if hashCRLF != hashLF {
		t.Errorf("SuiteHash must normalize CRLF and trailing whitespace: hashCRLF=%q, hashLF=%q", hashCRLF, hashLF)
	}
}

func TestSuiteHash_DefaultAndEmptyOmission(t *testing.T) {
	// Suite 1: explicitly sets empty maps, locked=false, skip=""
	suiteExplicitDefaults := &Suite{
		Seed: map[string]Seed{
			"s1": {Vars: map[string]any{}},
		},
		Setup: map[string]Setup{
			"setup1": {
				Use:   []string{},
				Vars:  map[string]any{},
				Steps: []Step{{Call: "GET /1", Headers: map[string]string{}, Save: map[string]string{}, Expect: 200}},
			},
		},
		Scenarios: []Scenario{
			{
				ID:     "sc-1",
				Name:   "Scenario 1",
				Setup:  []string{},
				Locked: false,
				Skip:   "",
				Steps:  []Step{{Call: "GET /1", Headers: map[string]string{}, Save: map[string]string{}, Expect: 200}},
			},
		},
	}

	// Suite 2: omits empty maps/slices and default bool/string values
	suiteOmittedDefaults := &Suite{
		Seed: map[string]Seed{
			"s1": {Vars: nil},
		},
		Setup: map[string]Setup{
			"setup1": {
				Use:   nil,
				Vars:  nil,
				Steps: []Step{{Call: "GET /1", Headers: nil, Save: nil, Expect: 200}},
			},
		},
		Scenarios: []Scenario{
			{
				ID:     "sc-1",
				Name:   "Scenario 1",
				Setup:  nil,
				Locked: false,
				Skip:   "",
				Steps:  []Step{{Call: "GET /1", Headers: nil, Save: nil, Expect: 200}},
			},
		},
	}

	hash1 := SuiteHash(suiteExplicitDefaults)
	hash2 := SuiteHash(suiteOmittedDefaults)

	if hash1 != hash2 {
		t.Errorf("SuiteHash must produce identical hash for empty vs nil maps/slices and default flags: hash1=%q, hash2=%q", hash1, hash2)
	}
}

func TestSuiteHash_EmptySeedAndSetupBlocksOmitted(t *testing.T) {
	// Empty seed and setup blocks normalize to nil and produce the same hash as missing blocks
	suiteEmptyBlocks := &Suite{
		Analysis:  "Basic suite",
		Seed:      map[string]Seed{},
		Setup:     map[string]Setup{},
		Scenarios: []Scenario{},
	}
	suiteNilBlocks := &Suite{
		Analysis:  "Basic suite",
		Seed:      nil,
		Setup:     nil,
		Scenarios: nil,
	}

	hash1 := SuiteHash(suiteEmptyBlocks)
	hash2 := SuiteHash(suiteNilBlocks)

	if hash1 != hash2 {
		t.Errorf("empty seed/setup/scenarios must hash identically to nil blocks: hash1=%q, hash2=%q", hash1, hash2)
	}
}

func TestSuiteHash_GoldenFixture(t *testing.T) {
	suite := &Suite{
		Version:  1,
		Input:    InputOpenAPI,
		Servers:  []string{"auth"},
		APIs:     []string{"POST {auth}/login"},
		Analysis: "Authentication and session management flows.",
		Seed: map[string]Seed{
			"global": {Vars: map[string]any{"base_url": "https://api.test"}},
		},
		Setup: map[string]Setup{
			"auth": {
				Steps: []Step{
					{
						Call:   "POST {auth}/login",
						Expect: 200,
						Save:   map[string]string{"token": "body.token"},
					},
				},
			},
		},
		Scenarios: []Scenario{
			{
				ID:    "login-success",
				Name:  "Successful login",
				Setup: []string{"auth"},
				Steps: []Step{
					{
						Call:    "GET {auth}/profile",
						Headers: map[string]string{"Authorization": "Bearer {{token}}"},
						Expect:  200,
					},
				},
			},
		},
	}

	got := SuiteHash(suite)
	// Compute expected canonical JSON:
	// Analysis: "Authentication and session management flows.\n"
	// Seed: {"global":{"vars":{"base_url":"https://api.test"}}}
	// Setup: {"auth":{"steps":[{"call":"POST {auth}/login","expect":200,"save":{"token":"body.token"}}]}}
	// Scenarios: [{"id":"login-success","name":"Successful login","setup":["auth"],"steps":[{"call":"GET {auth}/profile","headers":{"Authorization":"Bearer {{token}}"},"expect":200}]}]
	// SHA-256 of the above canonical JSON's first 12 hex chars:
	want := "426d8bf27669"

	if got != want {
		t.Errorf("SuiteHash(goldenSuite) = %q, want pinned hash %q", got, want)
	}
}
