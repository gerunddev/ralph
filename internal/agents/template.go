// Package agents provides agent orchestration and prompt management for Ralph.
package agents

import (
	"bytes"
	"fmt"
	"text/template"
)

// executeTemplate parses and executes a Go text/template with the given data.
// It returns the rendered string or an error if parsing or execution fails.
func executeTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
