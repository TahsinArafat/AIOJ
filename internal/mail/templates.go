package mail

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

//go:embed templates/*.txt
var templateFS embed.FS

func LoadTemplates() (*template.Template, error) {
	tpl := template.New("")
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("read embed dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := tpl.New(e.Name()).Parse(string(body)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
	}
	return tpl, nil
}

func RenderTemplate(tpl *template.Template, name string, data any, w io.Writer) error {
	return tpl.ExecuteTemplate(w, name, data)
}
