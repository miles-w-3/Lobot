package modes

import (
	"bytes"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/util"
	"sigs.k8s.io/yaml"
)

func formatManifest(resource k8s.TrackedObject) string {
	if resource == nil || resource.GetRaw() == nil {
		return "No resource selected"
	}

	return formatManifestObject(resource.GetRaw().Object)
}

func formatManifestObject(object interface{}) string {
	yamlBytes, err := yaml.Marshal(object)
	if err != nil {
		return fmt.Sprintf("Error formatting manifest: %v", err)
	}

	content := string(yamlBytes)
	var highlighted bytes.Buffer

	lexer := lexers.Get("yaml")
	if lexer == nil {
		lexer = lexers.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err == nil {
		if err := formatter.Format(&highlighted, style, iterator); err == nil {
			content = highlighted.String()
		}
	}

	lines := strings.Split(content, "\n")
	lineNumberWidth := len(fmt.Sprintf("%d", len(lines)))
	lineNumberStyle := lipgloss.NewStyle().Foreground(util.ColorMuted)

	var numbered strings.Builder
	for index, line := range lines {
		lineNumber := lineNumberStyle.Render(fmt.Sprintf("%*d", lineNumberWidth, index+1))
		fmt.Fprintf(&numbered, "%s │ %s\n", lineNumber, line)
	}
	return numbered.String()
}
