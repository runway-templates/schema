package schema_test

import (
	"testing"

	"github.com/runway-templates/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpressions_AllForms(t *testing.T) {
	t.Parallel()
	exprs, err := schema.Expressions(
		"${{ inputs.image_tag }} ${{services.valkey.outputs.DSN}} ${{ env.PORT }} ${{ runway.fqdn }} ${{ runway.secret(32) }}")
	require.NoError(t, err)
	require.Len(t, exprs, 5)

	assert.Equal(t, schema.Expr{Kind: schema.ExprInput, Name: "image_tag"}, exprs[0])
	assert.Equal(t, schema.Expr{Kind: schema.ExprOutput, Service: "valkey", Output: "DSN"}, exprs[1])
	assert.Equal(t, schema.Expr{Kind: schema.ExprEnv, Name: "PORT"}, exprs[2])
	assert.Equal(t, schema.Expr{Kind: schema.ExprRunway, Name: "fqdn"}, exprs[3])
	assert.Equal(t, schema.Expr{Kind: schema.ExprSecret, N: 32}, exprs[4])
}

func TestExpressions_MixedWithLiteralText(t *testing.T) {
	t.Parallel()
	exprs, err := schema.Expressions("redis://default:${{ env.PASSWORD }}@${{ runway.fqdn }}:6379")
	require.NoError(t, err)
	require.Len(t, exprs, 2)
	assert.Equal(t, "env.PASSWORD", exprs[0].String())
	assert.Equal(t, "runway.fqdn", exprs[1].String())
}

// Plain text, shell syntax, and k8s expansion syntax are not expressions.
func TestExpressions_LiteralOnly(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "no interpolation", "${VAR}", "$(VAR)", "$VAR", "{{ x }}"} {
		exprs, err := schema.Expressions(s)
		require.NoError(t, err, s)
		assert.Empty(t, exprs, s)
	}
}

// "$${{" escapes a literal "${{" and yields no expression.
func TestExpressions_Escape(t *testing.T) {
	t.Parallel()
	exprs, err := schema.Expressions("literal $${{ not.an.expr }} and ${{ env.REAL }}")
	require.NoError(t, err)
	require.Len(t, exprs, 1)
	assert.Equal(t, "env.REAL", exprs[0].String())
}

func TestExpressions_Errors(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]string{
		"unterminated":       "${{ env.PORT",
		"unknown namespace":  "${{ foo.bar }}",
		"bare service ref":   "${{ valkey.DSN }}",
		"missing outputs":    "${{ services.valkey.DSN }}",
		"lowercase env key":  "${{ env.port }}",
		"old secret form":    "${{ secret(32) }}",
		"zero secret length": "${{ runway.secret(0) }}",
		"empty body":         "${{ }}",
		"nested braces":      "${{ env.${{ env.A }} }}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := schema.Expressions(s)
			assert.Error(t, err, s)
		})
	}
}
