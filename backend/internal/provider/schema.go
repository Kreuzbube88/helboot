package provider

import (
	"fmt"
	"regexp"
	"strings"
)

// SettingsField describes one configurable profile setting of a
// provider (ADR-0012). The field key doubles as the template variable
// in the provider's answer-file template, so schema, stored config and
// generated file stay in lockstep. The frontend renders the profile
// form generically from these descriptors.
type SettingsField struct {
	// Key is the config document key and template variable name.
	Key string `yaml:"key" json:"key"`
	// Type is one of the FieldType* constants.
	Type string `yaml:"type" json:"type"`
	// Label is the English fallback label; the UI first tries the i18n
	// key profileFields.<Key>.
	Label string `yaml:"label" json:"label"`
	// Group buckets fields in the form (localization, users, network,
	// storage, packages, scripts, install).
	Group string `yaml:"group" json:"group,omitempty"`
	// Default pre-fills the form; its type must match Type.
	Default any `yaml:"default" json:"default,omitempty"`
	// Required fields must be present and non-empty — unless an
	// unsatisfied DependsOn hides them.
	Required bool `yaml:"required" json:"required,omitempty"`
	// Help is an optional hint shown under the field.
	Help string `yaml:"help" json:"help,omitempty"`
	// Placeholder is an optional input placeholder.
	Placeholder string `yaml:"placeholder" json:"placeholder,omitempty"`
	// Options enumerates the allowed values (select type only).
	Options []string `yaml:"options" json:"options,omitempty"`
	// Min/Max bound int values (int type only).
	Min *int `yaml:"min" json:"min,omitempty"`
	Max *int `yaml:"max" json:"max,omitempty"`
	// DependsOn shows (and requires) this field only while another
	// field has a specific value.
	DependsOn *FieldDependency `yaml:"depends_on" json:"dependsOn,omitempty"`
}

// FieldDependency gates a field on another field's value.
type FieldDependency struct {
	Field string `yaml:"field" json:"field"`
	Value any    `yaml:"value" json:"value"`
}

// Field types understood by the form generator and the validator.
const (
	FieldString   = "string"   // single-line text
	FieldText     = "text"     // multiline text (scripts, commands)
	FieldPassword = "password" // masked input, stored as given
	FieldBool     = "bool"
	FieldInt      = "int"
	FieldSelect   = "select" // one of Options
	FieldList     = "list"   // list of strings (packages, commands)
)

var fieldKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// validateSchema checks the structural invariants of a settings schema;
// a broken schema disables the provider at load time (ADR-0012).
func validateSchema(fields []SettingsField) error {
	keys := map[string]bool{}
	for i, f := range fields {
		if !fieldKeyRe.MatchString(f.Key) {
			return fmt.Errorf("settings_schema[%d]: key %q must be a letter followed by letters/digits", i, f.Key)
		}
		if keys[f.Key] {
			return fmt.Errorf("settings_schema: duplicate key %q", f.Key)
		}
		keys[f.Key] = true
		switch f.Type {
		case FieldString, FieldText, FieldPassword, FieldBool, FieldInt, FieldList:
		case FieldSelect:
			if len(f.Options) == 0 {
				return fmt.Errorf("settings_schema: field %s: select requires options", f.Key)
			}
		default:
			return fmt.Errorf("settings_schema: field %s: unknown type %q", f.Key, f.Type)
		}
		if f.Label == "" {
			return fmt.Errorf("settings_schema: field %s: label is required", f.Key)
		}
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return fmt.Errorf("settings_schema: field %s: min > max", f.Key)
		}
		if f.Default != nil {
			if err := checkFieldValue(f, f.Default); err != nil {
				return fmt.Errorf("settings_schema: field %s: default: %w", f.Key, err)
			}
		}
	}
	for _, f := range fields {
		if f.DependsOn != nil && !keys[f.DependsOn.Field] {
			return fmt.Errorf("settings_schema: field %s depends on unknown field %q", f.Key, f.DependsOn.Field)
		}
	}
	return nil
}

// ValidateConfig checks a profile configuration document against the
// manifest's settings schema: types, required fields (respecting
// dependencies), select membership and int ranges. Keys not declared in
// the schema are deliberately ignored so hand-written configs and older
// backups stay importable (ADR-0012).
func (m *Manifest) ValidateConfig(doc map[string]any) error {
	var problems []string
	for _, f := range m.SettingsSchema {
		value, present := doc[f.Key]
		if !present || isEmptyValue(value) {
			if f.Required && m.dependencySatisfied(f, doc) {
				problems = append(problems, fmt.Sprintf("%s is required", f.Key))
			}
			continue
		}
		if err := checkFieldValue(f, value); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", f.Key, err))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// dependencySatisfied reports whether f's dependency (if any) holds for
// the given document, i.e. whether the field is active.
func (m *Manifest) dependencySatisfied(f SettingsField, doc map[string]any) bool {
	if f.DependsOn == nil {
		return true
	}
	return looseEqual(doc[f.DependsOn.Field], f.DependsOn.Value)
}

// checkFieldValue validates a single non-empty value against its field
// descriptor. JSON numbers arrive as float64, YAML defaults as int —
// both are accepted for int fields.
func checkFieldValue(f SettingsField, value any) error {
	switch f.Type {
	case FieldString, FieldText, FieldPassword:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case FieldBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case FieldInt:
		n, ok := asInt(value)
		if !ok {
			return fmt.Errorf("must be an integer")
		}
		if f.Min != nil && n < *f.Min {
			return fmt.Errorf("must be at least %d", *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return fmt.Errorf("must be at most %d", *f.Max)
		}
	case FieldSelect:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		for _, opt := range f.Options {
			if s == opt {
				return nil
			}
		}
		return fmt.Errorf("must be one of: %s", strings.Join(f.Options, ", "))
	case FieldList:
		items, ok := value.([]any)
		if !ok {
			// YAML defaults decode as []string.
			if _, ok := value.([]string); ok {
				return nil
			}
			return fmt.Errorf("must be a list of strings")
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("must be a list of strings")
			}
		}
	}
	return nil
}

// isEmptyValue treats "" and nil as absent so required-checks and
// dependency gates behave like the form does.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

// asInt accepts the integer encodings that reach us: JSON float64,
// YAML int/int64, and Go int from tests.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

// looseEqual compares a config value with a dependency value across the
// JSON/YAML type seam (float64 vs int, bool vs bool).
func looseEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	if ai, ok := asInt(a); ok {
		if bi, ok := asInt(b); ok {
			return ai == bi
		}
	}
	return a == b
}
