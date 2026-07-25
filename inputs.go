package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"unicode/utf8"
)

// ValidateInputs checks user-supplied values against the inputs declared
// in t.
// It returns the normalized value set — defaults filled in, string values
// as string, integer values as int64, boolean values as bool — and the
// issues found. All issues are errors; a non-empty list means the  values
// must not be deployed.
//
// The returned map contains only declared inputs. An input that is
// optional, has no default, and was not provided is left out.
func ValidateInputs(t *Template, values map[string]any) (map[string]any, []Issue) {
	if t == nil {
		return nil, nil
	}
	var issues []Issue
	errf := func(name, format string, args ...any) {
		issues = append(issues, Issue{
			Path:     "inputs." + name,
			Message:  fmt.Sprintf(format, args...),
			Severity: SeverityError,
		})
	}

	for name := range values {
		if _, ok := t.Inputs[name]; !ok {
			errf(name, "not a declared input")
		}
	}

	normalized := make(map[string]any, len(t.Inputs))
	for name, in := range t.Inputs {
		value, provided := values[name]
		if !provided {
			if in.Required {
				errf(name, "required input missing")
				continue
			}
			if in.Default == nil {
				continue
			}
			value = in.Default
		}
		v, err := normalizeValue(in.Type, value)
		if err != nil {
			errf(name, "%v", err)
			continue
		}
		if err := checkConstraints(in, v); err != nil {
			errf(name, "%v", err)
			continue
		}
		normalized[name] = v
	}
	return normalized, issues
}

// normalizeValue converts v to the canonical Go type for an input type:
// string, int64, or bool. Integers accept whole float64 and json.Number
// values because JSON and YAML decoding produce those.
func normalizeValue(typ string, v any) (any, error) {
	switch typ {
	case "string":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected a string, got %T", v)
		}
		return s, nil
	case "integer":
		n, ok := toInt64(v)
		if !ok {
			return nil, fmt.Errorf("expected an integer, got %v (%T)", v, v)
		}
		return n, nil
	case "boolean":
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected a boolean, got %T", v)
		}
		return b, nil
	}
	return nil, fmt.Errorf("input has unknown type %q", typ)
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n == math.Trunc(n) && !math.IsInf(n, 0) {
			return int64(n), true
		}
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

// checkConstraints applies the per-type input rules to a normalized value.
func checkConstraints(in Input, v any) error {
	switch tv := v.(type) {
	case string:
		length := utf8.RuneCountInString(tv)
		if in.MinLength != nil && length < *in.MinLength {
			return fmt.Errorf("must be at least %d characters, got %d", *in.MinLength, length)
		}
		if in.MaxLength != nil && length > *in.MaxLength {
			return fmt.Errorf("must be at most %d characters, got %d", *in.MaxLength, length)
		}
		if in.Pattern != "" {
			re, err := regexp.Compile(in.Pattern)
			if err != nil {
				return fmt.Errorf("pattern in template does not compile: %v", err)
			}
			if !re.MatchString(tv) {
				return fmt.Errorf("must match pattern %s", in.Pattern)
			}
		}
	case int64:
		if in.Min != nil && tv < int64(*in.Min) {
			return fmt.Errorf("must be at least %d, got %d", *in.Min, tv)
		}
		if in.Max != nil && tv > int64(*in.Max) {
			return fmt.Errorf("must be at most %d, got %d", *in.Max, tv)
		}
	}
	if len(in.Enum) > 0 && !enumContains(in.Enum, v) {
		return fmt.Errorf("must be one of %v", in.Enum)
	}
	return nil
}

func enumContains(enum []any, v any) bool {
	for _, e := range enum {
		switch tv := v.(type) {
		case string:
			if s, ok := e.(string); ok && s == tv {
				return true
			}
		case int64:
			if n, ok := toInt64(e); ok && n == tv {
				return true
			}
		}
	}
	return false
}
