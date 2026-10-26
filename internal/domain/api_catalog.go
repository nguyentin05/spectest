package domain

// APICatalog maps an operation key (e.g. "POST {shop}/api/orders") to its tracked state.
type APICatalog map[string]APIState

// UpdateImpact records the setups and scenarios affected by an update.
type UpdateImpact struct {
	Setups    []string `yaml:"setups,omitempty" json:"setups,omitempty"`
	Scenarios []string `yaml:"scenarios,omitempty" json:"scenarios,omitempty"`
}

// APIStateRef pairs an API key with its state, used in catalog diffs.
type APIStateRef struct {
	Key   string   `yaml:"key" json:"key"`
	State APIState `yaml:"state" json:"state"`
}
