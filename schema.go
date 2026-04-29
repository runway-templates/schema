// Package schema validates Runway template files against the v1alpha1
// JSON Schema. The schema itself is embedded, so callers do not need to
// ship v1alpha1.json alongside their binary.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	sigsyaml "sigs.k8s.io/yaml"
)

//go:embed v1alpha1.json
var schemaJSON []byte

// ID is the canonical $id of the v1alpha1 schema. It matches the URL
// referenced by the yaml-language-server header in template files.
const ID = "https://raw.githubusercontent.com/runway-templates/schema/refs/heads/main/v1alpha1.json"

var compile = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(ID, doc); err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}
	sch, err := c.Compile(ID)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return sch, nil
})

// Schema returns the compiled v1alpha1 schema. The schema is compiled lazily
// on first call and cached for the lifetime of the process.
func Schema() (*jsonschema.Schema, error) {
	return compile()
}

// ValidateFile reads a YAML or JSON template from path and validates it
// against the v1alpha1 schema.
func ValidateFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Validate(raw)
}

// Validate parses raw as YAML or JSON (JSON is a subset of YAML, so either
// works) and validates the decoded document against the v1alpha1 schema.
func Validate(raw []byte) error {
	asJSON, err := sigsyaml.YAMLToJSON(raw)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	var doc any
	if err := json.Unmarshal(asJSON, &doc); err != nil {
		return fmt.Errorf("decode template: %w", err)
	}
	sch, err := Schema()
	if err != nil {
		return err
	}
	return sch.Validate(doc)
}
