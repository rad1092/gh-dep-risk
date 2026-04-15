package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"gh-dep-risk/internal/analysis"
)

type PullRequestMetadata struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Draft       bool   `json:"draft"`
	BaseSHA     string `json:"base_sha"`
	HeadSHA     string `json:"head_sha"`
	AuthorLogin string `json:"author_login"`
}

type Report struct {
	Repo     string                  `json:"repo"`
	PR       PullRequestMetadata     `json:"pr"`
	Analysis analysis.AnalysisResult `json:"analysis"`
}

func Render(report Report, format, lang string) (string, error) {
	switch strings.ToLower(format) {
	case "human":
		return renderHuman(report, lang), nil
	case "markdown":
		return renderMarkdown(report, lang), nil
	case "json":
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(payload) + "\n", nil
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
}
