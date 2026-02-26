package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gh "github.com/tomfevang/go-work/internal/github"
)

// claudeMsg is the subset of fields we care about from claude's stream-json output.
type claudeMsg struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	// assistant message
	Message *claudeMessageBlock `json:"message,omitempty"`
	// result message
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type claudeMessageBlock struct {
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Run drives a full session for one issue: creates a worktree, runs the
// planning phase, waits for approval via approveCh, then runs the
// implementation phase, and finally creates a PR.
//
// All progress is sent to eventCh. The caller must close approveCh or send
// false to cancel after WaitingApproval.
func Run(issue Issue, repoRoot string, eventCh chan<- Event, approveCh <-chan bool) {
	num := issue.Number
	send := func(t EventType, text string) {
		eventCh <- Event{IssueNumber: num, Type: t, Text: text}
	}
	fail := func(err error) {
		send(EventError, err.Error())
	}

	// --- worktree ---
	worktreeDir := filepath.Join(repoRoot, ".worktrees", fmt.Sprintf("%d", num))
	branch := fmt.Sprintf("issue-%d", num)

	// Remove stale worktree if it exists.
	_ = exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", worktreeDir).Run()
	_ = exec.Command("git", "-C", repoRoot, "branch", "-D", branch).Run()

	addCmd := exec.Command("git", "-C", repoRoot, "worktree", "add", worktreeDir, "-b", branch)
	if out, err := addCmd.CombinedOutput(); err != nil {
		fail(fmt.Errorf("create worktree: %w\n%s", err, out))
		return
	}

	// --- phase 1: planning ---
	planPrompt := fmt.Sprintf(
		"You are working on the following GitHub issue:\n\nTitle: %s\n\n%s\n\n"+
			"Create a concise implementation plan. List the files you will change, "+
			"your approach, and any edge cases. Do NOT write any code yet. "+
			"End your response with the exact line: PLAN COMPLETE",
		issue.Title, issue.Body,
	)

	send(EventOutput, "=== Planning phase ===\n")
	planText, err := runClaude(worktreeDir, planPrompt, nil, send)
	if err != nil {
		fail(fmt.Errorf("planning: %w", err))
		return
	}
	send(EventPlanDone, planText)

	// --- wait for approval ---
	approved, ok := <-approveCh
	if !ok || !approved {
		send(EventError, "plan rejected or session cancelled")
		_ = exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", worktreeDir).Run()
		return
	}

	// --- phase 2: implementation ---
	implPrompt := fmt.Sprintf(
		"Implement the following approved plan for GitHub issue #%d.\n\n"+
			"ISSUE TITLE: %s\n\nISSUE BODY:\n%s\n\nAPPROVED PLAN:\n%s\n\n"+
			"Write the code. After implementation, run the project's tests if a "+
			"test command is available. Do NOT create a pull request.",
		num, issue.Title, issue.Body, planText,
	)

	allowedTools := []string{"Edit", "Write", "Bash", "Glob", "Grep", "Read"}
	send(EventOutput, "\n=== Implementation phase ===\n")
	_, err = runClaude(worktreeDir, implPrompt, allowedTools, send)
	if err != nil {
		fail(fmt.Errorf("implementation: %w", err))
		return
	}
	send(EventImplDone, "")

	// --- create PR ---
	send(EventOutput, "\n=== Creating PR ===\n")
	prURL, err := gh.CreatePR(worktreeDir, issue.Number, issue.Title)
	if err != nil {
		fail(fmt.Errorf("create PR: %w", err))
		return
	}
	send(EventPRDone, prURL)
}

// runClaude spawns claude with --output-format stream-json and streams its
// output to eventCh. It returns the concatenated assistant text.
func runClaude(cwd, prompt string, allowedTools []string, send func(EventType, string)) (string, error) {
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose"}
	if len(allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowedTools, ","))
	}

	cmd := exec.Command("claude", args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start claude: %w", err)
	}

	var fullText strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg claudeMsg
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not JSON — pass through as raw output.
			send(EventOutput, line+"\n")
			continue
		}

		switch msg.Type {
		case "assistant":
			if msg.Message != nil {
				for _, block := range msg.Message.Content {
					if block.Type == "text" && block.Text != "" {
						fullText.WriteString(block.Text)
						send(EventOutput, block.Text)
					}
				}
			}
		case "result":
			if msg.Error != "" {
				return fullText.String(), fmt.Errorf("claude error: %s", msg.Error)
			}
			if msg.Result != "" {
				fullText.WriteString(msg.Result)
				send(EventOutput, msg.Result)
			}
		case "system":
			if msg.Subtype == "init" {
				send(EventOutput, "[session started]\n")
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fullText.String(), fmt.Errorf("claude exited: %w", err)
	}
	return fullText.String(), nil
}
