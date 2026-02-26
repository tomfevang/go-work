package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Issue holds the GitHub issue data we care about.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// ListIssues fetches open issues from the GitHub repo in the current directory.
func ListIssues() ([]Issue, error) {
	cmd := exec.Command("gh", "issue", "list",
		"--state", "open",
		"--json", "number,title,body",
		"--limit", "50",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}
	return issues, nil
}

// CreatePR creates a pull request from the given worktree directory and returns
// the PR URL.
func CreatePR(worktreeDir string, issueNum int, title string) (string, error) {
	prTitle := fmt.Sprintf("Fix #%d: %s", issueNum, title)
	body := fmt.Sprintf("Closes #%d\n\nAutomatically implemented via go-work.", issueNum)

	cmd := exec.Command("gh", "pr", "create",
		"--title", prTitle,
		"--body", body,
		"--fill-first",
	)
	cmd.Dir = worktreeDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
