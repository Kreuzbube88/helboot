package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func schemaManifest(fields ...SettingsField) *Manifest {
	return &Manifest{Name: "test", DisplayName: "Test", Family: "test",
		Capabilities: map[string]bool{"iso": true}, SettingsSchema: fields}
}

func TestValidateSchemaRejectsBrokenSchemas(t *testing.T) {
	cases := map[string][]SettingsField{
		"bad key":            {{Key: "bad key", Type: FieldString, Label: "x"}},
		"duplicate key":      {{Key: "A", Type: FieldString, Label: "x"}, {Key: "A", Type: FieldString, Label: "x"}},
		"unknown type":       {{Key: "A", Type: "csv", Label: "x"}},
		"select w/o options": {{Key: "A", Type: FieldSelect, Label: "x"}},
		"missing label":      {{Key: "A", Type: FieldString}},
		"dangling depends":   {{Key: "A", Type: FieldString, Label: "x", DependsOn: &FieldDependency{Field: "Nope", Value: true}}},
		"default type":       {{Key: "A", Type: FieldInt, Label: "x", Default: "one"}},
	}
	for name, fields := range cases {
		if err := schemaManifest(fields...).Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	min1, max3 := 1, 3
	m := schemaManifest(
		SettingsField{Key: "Username", Type: FieldString, Label: "Username", Required: true},
		SettingsField{Key: "Layout", Type: FieldSelect, Label: "Layout", Options: []string{"lvm", "direct"}},
		SettingsField{Key: "Index", Type: FieldInt, Label: "Index", Min: &min1, Max: &max3},
		SettingsField{Key: "UseZFS", Type: FieldBool, Label: "ZFS"},
		SettingsField{Key: "Pool", Type: FieldString, Label: "Pool", Required: true,
			DependsOn: &FieldDependency{Field: "UseZFS", Value: true}},
		SettingsField{Key: "Packages", Type: FieldList, Label: "Packages"},
	)
	if err := m.Validate(); err != nil {
		t.Fatalf("schema should be valid: %v", err)
	}

	valid := []string{
		`{"Username": "kai"}`,
		`{"Username": "kai", "Layout": "lvm", "Index": 2, "Packages": ["curl", "git"]}`,
		`{"Username": "kai", "UseZFS": false}`,       // Pool not required while dependency off
		`{"Username": "kai", "Unknown": "whatever"}`, // undeclared keys are preserved, not rejected
	}
	for _, doc := range valid {
		if err := m.ValidateConfig(unmarshalDoc(t, doc)); err != nil {
			t.Errorf("config %s: unexpected error: %v", doc, err)
		}
	}

	invalid := map[string]string{
		`{}`:                                    "Username is required",
		`{"Username": ""}`:                      "Username is required",
		`{"Username": 5}`:                       "must be a string",
		`{"Username": "kai", "Layout": "zfs"}`:  "must be one of",
		`{"Username": "kai", "Index": 9}`:       "at most",
		`{"Username": "kai", "Index": 1.5}`:     "integer",
		`{"Username": "kai", "UseZFS": true}`:   "Pool is required",
		`{"Username": "kai", "Packages": [1]}`:  "list of strings",
		`{"Username": "kai", "UseZFS": "true"}`: "boolean",
	}
	for doc, want := range invalid {
		err := m.ValidateConfig(unmarshalDoc(t, doc))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("config %s: got %v, want error containing %q", doc, err, want)
		}
	}
}

func unmarshalDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestShippedManifestsHaveValidSchemas loads the real providers/ tree so
// a broken settings_schema fails CI, not a homelab at boot.
func TestShippedManifestsHaveValidSchemas(t *testing.T) {
	reg, err := LoadDir("../../../providers", discardLogger())
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	all := reg.All()
	if len(all) < 11 {
		t.Fatalf("loaded %d shipped providers, want at least 11 — a manifest failed validation", len(all))
	}
	for _, m := range all {
		if m.Has(CapUnattendedInstall) && len(m.SettingsSchema) == 0 && m.AnswerFile.Template != "" {
			t.Errorf("provider %s supports unattended install but declares no settings_schema", m.Name)
		}
	}
}

func TestLoadDirsOverlay(t *testing.T) {
	base, extra := t.TempDir(), t.TempDir()
	writeManifest(t, base, "debian", validManifest)
	writeManifest(t, extra, "debian", strings.Replace(validManifest, `"Debian"`, `"Debian (patched)"`, 1))

	reg, err := LoadDirs(discardLogger(), base, extra)
	if err != nil {
		t.Fatalf("LoadDirs: %v", err)
	}
	m := reg.Get("debian")
	if m == nil || m.DisplayName != "Debian (patched)" {
		t.Fatalf("later load location must win, got %+v", m)
	}
}
