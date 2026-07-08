package answer

import (
	"strings"
	"testing"
)

func TestRenderMergesConfigAndParams(t *testing.T) {
	tmpl := "user={{ .Username }} url={{ .AnswerFileURL }} host={{ .Hostname }} pkgs={{ range .Packages }}{{ . }} {{ end }}"
	config := `{"Username": "pi", "Packages": ["curl", "htop"]}`
	out, err := Render(tmpl, config, Params{
		AnswerFileURL: "http://h/boot/answer/tok",
		Hostname:      "node1",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)
	for _, want := range []string{"user=pi", "url=http://h/boot/answer/tok", "host=node1", "curl htop"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestRenderProfileCannotOverrideBootParams(t *testing.T) {
	out, err := Render("{{ .ReportURL }}", `{"ReportURL": "http://evil"}`, Params{ReportURL: "http://good"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "http://good" {
		t.Errorf("boot param overridden: %s", out)
	}
}

func TestRenderProfileHostnameWins(t *testing.T) {
	out, err := Render("{{ .Hostname }}", `{"Hostname": "from-profile"}`, Params{Hostname: "from-host"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "from-profile" {
		t.Errorf("profile hostname should win, got %s", out)
	}
}

func TestRenderMissingKeysAreEmpty(t *testing.T) {
	out, err := Render("[{{ .DoesNotExist }}]", `{}`, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[]" {
		t.Errorf("missing key rendered as %q, want empty", out)
	}
}

func TestRenderRejectsBrokenConfig(t *testing.T) {
	if _, err := Render("x", `{not json`, Params{}); err == nil {
		t.Error("expected error for invalid config JSON")
	}
}
