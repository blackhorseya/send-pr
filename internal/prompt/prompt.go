package prompt

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
)

var (
	//go:embed templates/*
	f embed.FS

	templates    map[string]*template.Template
	templatesDir = "templates"
)

const (
	SummarizePRDiffTemplate = "summarize_pr_diff.tmpl"
)

func init() {
	err := loadTemplates(f)
	if err != nil {
		panic(err)
	}
}

// GetPromptString returns the prompt string for the given prompt name and data.
func GetPromptString(name string, data map[string]interface{}) (string, error) {
	tmpl, ok := templates[name]
	if !ok {
		return "", fs.ErrNotExist
	}

	var tpl bytes.Buffer
	err := tmpl.Execute(&tpl, data)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}

// loadTemplates loads all the templates found in the templates directory from the embedded filesystem.
// It returns an error if reading the directory or parsing any template fails.
func loadTemplates(files embed.FS) error {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		return err
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(files, templatesDir+"/"+tmpl.Name())
		if err != nil {
			return err
		}

		templates[tmpl.Name()] = pt
	}

	return nil
}
