package domain

// Change targets accepted by history entries.
const (
	TargetAnalysis = "analysis"
	TargetSeed     = "seed"
	TargetSetup    = "setup"
	TargetScenario = "scenario"
)

// Entry authors accepted by history entries.
const (
	ByAI   = "ai"
	ByUser = "user"
)

// Track API states.
const (
	StatusActive  = "active"
	StatusRemoved = "removed"
)

// LogFile is the append-only `spectest.log.yaml` history. Base snapshots the
// first generated suite and every Update entry records a subsequent change.
type LogFile struct {
	Base   BaseEntry     `yaml:"base" json:"base"`
	Update []UpdateEntry `yaml:"update,omitempty" json:"update,omitempty"`
}

// BaseEntry is the initial suite snapshot.
type BaseEntry struct {
	At               string     `yaml:"at" json:"at"`
	By               string     `yaml:"by" json:"by"`
	Spectest         *Suite     `yaml:"spectest" json:"spectest"`
	AfterHash        string     `yaml:"after_hash" json:"after_hash"`
	APICatalog       APICatalog `yaml:"api_catalog,omitempty" json:"api_catalog,omitempty"`
	RequirementsHash string     `yaml:"requirements_hash,omitempty" json:"requirements_hash,omitempty"`
}

// UpdateEntry is one append-only history record. N starts at 1 and increases
// without gaps. Trigger is only present on AI-authored entries.
type UpdateEntry struct {
	N            int          `yaml:"n" json:"n"`
	At           string       `yaml:"at" json:"at"`
	By           string       `yaml:"by" json:"by"`
	Note         string       `yaml:"note,omitempty" json:"note,omitempty"`
	Trigger      *Trigger     `yaml:"trigger,omitempty" json:"trigger,omitempty"`
	CatalogPatch APICatalog   `yaml:"catalog_patch,omitempty" json:"catalog_patch,omitempty"`
	Impact       UpdateImpact `yaml:"impact,omitempty" json:"impact,omitempty"`
	Create       *CreateSet   `yaml:"create,omitempty" json:"create,omitempty"`
	Change       []Change     `yaml:"change,omitempty" json:"change,omitempty"`
	AfterHash    string       `yaml:"after_hash" json:"after_hash"`
}

// Trigger describes the source changes that caused an AI update.
type Trigger struct {
	OpenAPI      *OpenAPITrigger      `yaml:"openapi,omitempty" json:"openapi,omitempty"`
	Requirements *RequirementsTrigger `yaml:"requirements,omitempty" json:"requirements,omitempty"`
}

// OpenAPITrigger lists semantic API changes since the previous run.
type OpenAPITrigger struct {
	Added    []string `yaml:"added,omitempty" json:"added,omitempty"`
	Changed  []string `yaml:"changed,omitempty" json:"changed,omitempty"`
	Removed  []string `yaml:"removed,omitempty" json:"removed,omitempty"`
	Restored []string `yaml:"restored,omitempty" json:"restored,omitempty"`
}

// RequirementsTrigger records the normalized requirements hashes before and after.
type RequirementsTrigger struct {
	BeforeHash string `yaml:"before_hash" json:"before_hash"`
	AfterHash  string `yaml:"after_hash" json:"after_hash"`
}

// CreateSet lists entities added by an entry.
type CreateSet struct {
	Seed      map[string]Seed  `yaml:"seed,omitempty" json:"seed,omitempty"`
	Setup     map[string]Setup `yaml:"setup,omitempty" json:"setup,omitempty"`
	Scenarios []Scenario       `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
}

// Change records a field- or entity-level mutation. Before and After are
// intentionally untyped: a nil After at entity level means deletion, while a
// nil field value inside a patch means the field is removed. The After field
// deliberately has no omitempty so explicit `after: null` survives encoding.
type Change struct {
	Target string `yaml:"target" json:"target"`
	ID     string `yaml:"id,omitempty" json:"id,omitempty"`
	Reason string `yaml:"reason" json:"reason"`
	Before any    `yaml:"before,omitempty" json:"before,omitempty"`
	After  any    `yaml:"after" json:"after"`
}
