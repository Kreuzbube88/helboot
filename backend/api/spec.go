// Package api holds the OpenAPI contract for HELBOOT's REST API
// (ADR-0010). The YAML document is embedded so the running server can
// serve its own specification.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
