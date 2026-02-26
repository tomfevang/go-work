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

// CreatePR commits any uncommitted changes, pushes the worktree branch, and
// opens a pull request, returning the PR URL.
func CreatePR(worktreeDir string, issueNum int, title string) (string, error) {
	// Push the branch so gh can open a PR against it.
	pushCmd := exec.Command("git", "push", "-u", "origin", "HEAD")
	pushCmd.Dir = worktreeDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git push: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	prTitle := fmt.Sprintf("Fix #%d: %s", issueNum, title)
	body := fmt.Sprintf("Closes #%d\n\nAutomatically implemented via go-work.", issueNum)

	prCmd := exec.Command("gh", "pr", "create",
		"--title", prTitle,
		"--body", body,
	)
	prCmd.Dir = worktreeDir

	out, err := prCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
