package prompt

import (
	"embed"
)

//go:embed templates/*
var f embed.FS

const (
	SummarizePRDiffTemplate = "summarize_pr_diff.tmpl"
)

func init() {
}
