package cmd

import "encoding/json"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type buildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func currentBuildInfo() buildInfo {
	return buildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}

func (b buildInfo) String() string {
	return "gh-dep-risk " + b.Version + " (commit " + b.Commit + ", built " + b.Date + ")"
}

func (b buildInfo) JSON() (string, error) {
	payload, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}
