package schema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExprKind identifies the namespace of a parsed ${{ ... }} expression.
type ExprKind int

const (
	// ExprInput is ${{ inputs.<name> }}.
	ExprInput ExprKind = iota
	// ExprOutput is ${{ services.<service>.outputs.<KEY> }}.
	ExprOutput
	// ExprEnv is ${{ env.<KEY> }}.
	ExprEnv
	// ExprRunway is ${{ runway.<field> }}.
	ExprRunway
	// ExprSecret is ${{ runway.secret(<n>) }}.
	ExprSecret
)

// Expr is one interpolation expression found in a template string.
// The grammar is defined in docs/interpolation/interpolation.md.
type Expr struct {
	Kind ExprKind
	// Name holds the input name (ExprInput), env key (ExprEnv), or
	// runway field (ExprRunway).
	Name string
	// Service and Output are set for ExprOutput.
	Service string
	Output  string
	// N is the requested secret length for ExprSecret.
	N int
}

func (e Expr) String() string {
	switch e.Kind {
	case ExprInput:
		return "inputs." + e.Name
	case ExprOutput:
		return "services." + e.Service + ".outputs." + e.Output
	case ExprEnv:
		return "env." + e.Name
	case ExprRunway:
		return "runway." + e.Name
	case ExprSecret:
		return fmt.Sprintf("runway.secret(%d)", e.N)
	}
	return fmt.Sprintf("unknown expression kind %d", int(e.Kind))
}

var (
	inputRe  = regexp.MustCompile(`^inputs\.([a-zA-Z_][a-zA-Z0-9_]*)$`)
	outputRe = regexp.MustCompile(`^services\.([a-z](?:[a-z0-9-]{0,61}[a-z0-9])?)\.outputs\.([A-Z_][A-Z0-9_]*)$`)
	envRe    = regexp.MustCompile(`^env\.([A-Z_][A-Z0-9_]*)$`)
	secretRe = regexp.MustCompile(`^runway\.secret\(([1-9][0-9]*)\)$`)
	runwayRe = regexp.MustCompile(`^runway\.([a-z][a-z0-9_]*)$`)
)

// Expressions extracts every ${{ ... }} expression from s, in order of
// appearance. "$${{" escapes a literal "${{" and yields no expression;
// anything that is not a ${{ ... }} expression is literal text. It returns
// an error for an unterminated "${{" or a body that does not match the
// grammar.
func Expressions(s string) ([]Expr, error) {
	var out []Expr
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "$${{") {
			i += len("$${{")
			continue
		}
		if !strings.HasPrefix(s[i:], "${{") {
			i++
			continue
		}
		rest := s[i+len("${{"):]
		end := strings.Index(rest, "}}")
		if end < 0 {
			return nil, fmt.Errorf("unterminated ${{ at offset %d", i)
		}
		expr, err := parseExpr(strings.Trim(rest[:end], " "))
		if err != nil {
			return nil, err
		}
		out = append(out, expr)
		i += len("${{") + end + len("}}")
	}
	return out, nil
}

func parseExpr(body string) (Expr, error) {
	if m := inputRe.FindStringSubmatch(body); m != nil {
		return Expr{Kind: ExprInput, Name: m[1]}, nil
	}
	if m := outputRe.FindStringSubmatch(body); m != nil {
		return Expr{Kind: ExprOutput, Service: m[1], Output: m[2]}, nil
	}
	if m := envRe.FindStringSubmatch(body); m != nil {
		return Expr{Kind: ExprEnv, Name: m[1]}, nil
	}
	if m := secretRe.FindStringSubmatch(body); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return Expr{}, fmt.Errorf("invalid secret length %q: %w", m[1], err)
		}
		return Expr{Kind: ExprSecret, N: n}, nil
	}
	if m := runwayRe.FindStringSubmatch(body); m != nil {
		return Expr{Kind: ExprRunway, Name: m[1]}, nil
	}
	return Expr{}, fmt.Errorf("expression %q does not match any known form", body)
}
