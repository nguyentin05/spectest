package domain

// APIInput is one `--api <server>=<path>` command-line value.
type APIInput struct {
	Server string
	Path   string
}

// SourceSet is the fully loaded and normalized view of all CLI inputs.
// Servers preserves --api order; APIs is sorted by server, path, then method.
type SourceSet struct {
	Input        InputMode            `yaml:"input" json:"input"`
	Servers      []string             `yaml:"servers" json:"servers"`
	APIs         []API                `yaml:"apis" json:"apis"`
	OpenAPI      map[string]FileState `yaml:"openapi,omitempty" json:"openapi,omitempty"`
	OpenAPIRaw   map[string]string    `yaml:"-" json:"-"`
	Requirements *RequirementSource   `yaml:"requirements,omitempty" json:"requirements,omitempty"`
}

// RequirementSource is the raw requirements input; it is context for the LLM,
// not structured data, and therefore has no IDs or sections.
type RequirementSource struct {
	File string `yaml:"file" json:"file"`
	Raw  string `yaml:"raw" json:"raw"`
}

// API is a normalized OpenAPI operation. The Key is `METHOD {server}PATH`.
type API struct {
	Key         string            `yaml:"key" json:"key"`
	Method      string            `yaml:"method" json:"method"`
	Server      string            `yaml:"server" json:"server"`
	Path        string            `yaml:"path" json:"path"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Hash        string            `yaml:"hash" json:"hash"`
	Sig         APISignature      `yaml:"sig" json:"sig"`
	Parameters  []APIParameter    `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Request     *RequestSpec      `yaml:"request,omitempty" json:"request,omitempty"`
	Responses   map[string]Schema `yaml:"responses,omitempty" json:"responses,omitempty"`
	RawSnippet  string            `yaml:"raw_snippet,omitempty" json:"raw_snippet,omitempty"`
}

// APIParameter is a normalized path, query, header, or cookie parameter.
type APIParameter struct {
	Name     string `yaml:"name" json:"name"`
	In       string `yaml:"in" json:"in"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Schema   Schema `yaml:"schema" json:"schema"`
}

// RequestSpec is the selected request body media type and its schema.
// JSON is false when the API accepts only non-JSON bodies.
type RequestSpec struct {
	Required  bool   `yaml:"required,omitempty" json:"required,omitempty"`
	MediaType string `yaml:"media_type" json:"media_type"`
	JSON      bool   `yaml:"json" json:"json"`
	Schema    Schema `yaml:"schema" json:"schema"`
}

// Schema is the shallow schema subset used by the v1 validator. Constructs the
// validator cannot reason about are marked Unsupported instead of guessed.
type Schema struct {
	Type                 string            `yaml:"type,omitempty" json:"type,omitempty"`
	Required             []string          `yaml:"required,omitempty" json:"required,omitempty"`
	Properties           map[string]Schema `yaml:"properties,omitempty" json:"properties,omitempty"`
	Items                *Schema           `yaml:"items,omitempty" json:"items,omitempty"`
	AdditionalProperties *bool             `yaml:"additional_properties,omitempty" json:"additional_properties,omitempty"`
	Unsupported          bool              `yaml:"unsupported,omitempty" json:"unsupported,omitempty"`
}

// APIMention is an API mentioned in the requirements and whether it is
// present in OpenAPI. Mentions never create APIs; they only produce warnings.
type APIMention struct {
	Method   string `yaml:"method,omitempty" json:"method,omitempty"`
	Path     string `yaml:"path,omitempty" json:"path,omitempty"`
	Service  string `yaml:"service,omitempty" json:"service,omitempty"`
	Evidence string `yaml:"evidence" json:"evidence"`
	Known    bool   `yaml:"known,omitempty" json:"known,omitempty"`
}
