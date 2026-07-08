// Package answer renders unattended-install answer files from provider
// templates and profile configuration (§12). Generated files never touch
// the original ISO — they are served per installation at boot time.
package answer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
)

// Params are the boot-time values available to templates in addition to
// the profile configuration document.
type Params struct {
	// AnswerFileURL is where the installer fetches this very file.
	AnswerFileURL string
	// ISOURL serves the complete original ISO over HTTP (range capable).
	ISOURL string
	// ISOContentURL serves single files from inside the ISO:
	// <ISOContentURL>/casper/vmlinuz
	ISOContentURL string
	// CloudInitURL is the NoCloud seed directory (user-data, meta-data).
	CloudInitURL string
	// ReportURL lets late-commands report success/failure back.
	ReportURL string
	// Hostname of the machine being installed.
	Hostname string
	// MAC of the machine being installed.
	MAC string
}

// RenderFile executes the template file with the profile configuration
// and boot parameters.
func RenderFile(templatePath string, configJSON string, params Params) ([]byte, error) {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read answer template: %w", err)
	}
	return Render(string(raw), configJSON, params)
}

// Render executes a template string with the profile configuration
// (a JSON document) merged with the boot parameters. Config keys are
// exposed directly ({{ .Username }}, {{ .Packages }}, ...); boot
// parameters win on conflict so profiles cannot break the boot chain.
func Render(tmplStr string, configJSON string, params Params) ([]byte, error) {
	data := map[string]any{}
	if strings.TrimSpace(configJSON) != "" {
		if err := json.Unmarshal([]byte(configJSON), &data); err != nil {
			return nil, fmt.Errorf("parse profile config: %w", err)
		}
	}
	for k, v := range map[string]any{
		"AnswerFileURL": params.AnswerFileURL,
		"ISOURL":        params.ISOURL,
		"ISOContentURL": params.ISOContentURL,
		"CloudInitURL":  params.CloudInitURL,
		"ReportURL":     params.ReportURL,
		"MAC":           params.MAC,
	} {
		data[k] = v
	}
	// Hostname from the host record only fills in when the profile does
	// not set one.
	if _, ok := data["Hostname"]; !ok {
		data["Hostname"] = params.Hostname
	}

	tmpl, err := template.New("answer").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse answer template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render answer file: %w", err)
	}
	// missingkey=zero renders absent map keys as "<no value>"; blank
	// them so installers see empty strings instead.
	return bytes.ReplaceAll(buf.Bytes(), []byte("<no value>"), nil), nil
}
