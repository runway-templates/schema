package schema_test

import (
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeFile_n8n(t *testing.T) {
	t.Parallel()
	tmpl := loadFixture(t, "n8n.yaml")

	assert.Equal(t, "schema.runway.horse/templates/v1alpha1", tmpl.APIVersion)
	assert.Equal(t, "Template", tmpl.Kind)
	assert.Equal(t, "n8n", tmpl.Metadata.Name)
	assert.Equal(t, "SustainableUse-1.0", tmpl.Metadata.License)
	assert.Equal(t, "automation", tmpl.Metadata.Category)

	require.Contains(t, tmpl.Inputs, "postgres_port")
	port := tmpl.Inputs["postgres_port"]
	assert.Equal(t, "integer", port.Type)
	require.NotNil(t, port.Min)
	assert.Equal(t, 1, *port.Min)
	require.NotNil(t, port.Max)
	assert.Equal(t, 65535, *port.Max)

	require.Len(t, tmpl.Services, 1)
	svc := tmpl.Services[0]
	assert.Equal(t, "n8n", svc.Name)
	assert.Contains(t, svc.Image, "n8nio/n8n")

	// Shorthand env decodes as {Value: "..."}.
	require.Contains(t, svc.Env, "PORT")
	assert.Equal(t, "5678", svc.Env["PORT"].Value)
	assert.Empty(t, svc.Env["PORT"].Warning)

	// Object env preserves the warning field.
	require.Contains(t, svc.Env, "N8N_ENCRYPTION_KEY")
	enc := svc.Env["N8N_ENCRYPTION_KEY"]
	assert.Equal(t, "${secret(48)}", enc.Value)
	assert.Contains(t, enc.Warning, "credentials")

	require.NotNil(t, svc.Volume)
	assert.Equal(t, "/home/node/.n8n", svc.Volume.MountPath)
	require.Len(t, svc.Init, 1)
	require.Len(t, svc.Workers, 1)
	require.NotNil(t, svc.Healthcheck)
	assert.Equal(t, "http", svc.Healthcheck.Type)
}

func TestDecode_EnvValueShapes(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
	  "apiVersion": "schema.runway.horse/templates/v1alpha1",
	  "kind": "Template",
	  "metadata": {
	    "name": "demo", "version": "1.0.0", "displayName": "Demo",
	    "description": "x", "category": "other"
	  },
	  "services": [{
	    "name": "web", "image": "nginx:1.27",
	    "env": {
	      "PORT": "8080",
	      "DB_URL": {"value": "postgres://...", "description": "db"}
	    }
	  }]
	}`)
	tmpl, err := schema.Decode(raw)
	require.NoError(t, err)
	env := tmpl.Services[0].Env
	assert.Equal(t, "8080", env["PORT"].Value)
	assert.Equal(t, "postgres://...", env["DB_URL"].Value)
	assert.Equal(t, "db", env["DB_URL"].Description)
}
