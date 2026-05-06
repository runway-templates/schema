package schema

import (
	"encoding/json"
	"fmt"
	"os"

	sigsyaml "sigs.k8s.io/yaml"
)

// Template is the typed representation of a v1alpha1 template document.
// It mirrors the JSON Schema in v1alpha1.json. Field-level rules (pattern,
// enum, etc.) are not enforced here — run Validate for that.
type Template struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   Metadata         `json:"metadata"`
	Inputs     map[string]Input `json:"inputs,omitempty"`
	Services   []Service        `json:"services"`
}

type Metadata struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	DisplayName   string   `json:"displayName"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Tags          []string `json:"tags,omitempty"`
	Logo          string   `json:"logo,omitempty"`
	Documentation string   `json:"documentation,omitempty"`
	Website       string   `json:"website,omitempty"`
	Source        string   `json:"source,omitempty"`
	License       string   `json:"license,omitempty"`
	Maintainer    string   `json:"maintainer,omitempty"`
}

// Input describes a single user-supplied input. Default and Enum are typed
// as any because the value type depends on Type (string, integer, boolean).
type Input struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Enum        []any  `json:"enum,omitempty"`
	Min         *int   `json:"min,omitempty"`
	Max         *int   `json:"max,omitempty"`
	MinLength   *int   `json:"minLength,omitempty"`
	MaxLength   *int   `json:"maxLength,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	Category    string `json:"category,omitempty"`
}

type Service struct {
	Name        string              `json:"name"`
	Image       string              `json:"image"`
	Command     []string            `json:"command,omitempty"`
	Args        []string            `json:"args,omitempty"`
	Env         map[string]EnvValue `json:"env,omitempty"`
	Volume      *Volume             `json:"volume,omitempty"`
	Init        []InitContainer     `json:"init,omitempty"`
	Workers     []WorkerContainer   `json:"workers,omitempty"`
	Outputs     map[string]string   `json:"outputs,omitempty"`
	Healthcheck *Healthcheck        `json:"healthcheck,omitempty"`
	MinPlan     string              `json:"minPlan,omitempty"`
}

// EnvValue accepts either a bare string ("PORT": "8080") or the full object
// form ({"value": "8080", "description": "..."}). The shorthand is decoded
// as if it were {"value": "<string>"}.
type EnvValue struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Warning     string `json:"warning,omitempty"`
}

func (e *EnvValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Value = s
		return nil
	}
	type alias EnvValue
	return json.Unmarshal(data, (*alias)(e))
}

func (e EnvValue) MarshalJSON() ([]byte, error) {
	if e.Description == "" && e.Warning == "" {
		return json.Marshal(e.Value)
	}
	type alias EnvValue
	return json.Marshal(alias(e))
}

type Volume struct {
	MountPath string `json:"mountPath"`
}

type InitContainer struct {
	Name    string              `json:"name"`
	Image   string              `json:"image,omitempty"`
	Command []string            `json:"command,omitempty"`
	Args    []string            `json:"args,omitempty"`
	Env     map[string]EnvValue `json:"env,omitempty"`
}

type WorkerContainer struct {
	Name    string              `json:"name"`
	Image   string              `json:"image,omitempty"`
	Command []string            `json:"command,omitempty"`
	Args    []string            `json:"args,omitempty"`
	Env     map[string]EnvValue `json:"env,omitempty"`
}

type Healthcheck struct {
	Type                string   `json:"type,omitempty"`
	Path                string   `json:"path,omitempty"`
	Command             []string `json:"command,omitempty"`
	InitialDelaySeconds *int     `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       *int     `json:"periodSeconds,omitempty"`
	TimeoutSeconds      *int     `json:"timeoutSeconds,omitempty"`
}

// DecodeFile reads a YAML or JSON template from path and returns the typed
// Template. It does not run schema validation — call Validate or
// ValidateFile separately if you need the structural checks.
func DecodeFile(path string) (*Template, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(raw)
}

// Decode parses raw as YAML or JSON into a Template. It does not run schema
// validation; pair it with Validate when both a typed model and structural
// checks are needed.
func Decode(raw []byte) (*Template, error) {
	asJSON, err := sigsyaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var t Template
	if err := json.Unmarshal(asJSON, &t); err != nil {
		return nil, fmt.Errorf("decode template: %w", err)
	}
	return &t, nil
}
