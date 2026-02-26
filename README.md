# go-work

A TUI that automates GitHub issue resolution using Claude Code. Manages parallel sessions, each following a plan → approve → implement → PR workflow.

## Requirements

- Go 1.25+
- [`gh`](https://cli.github.com/) (GitHub CLI, authenticated)
- `git`
- `claude` (Claude Code CLI)

## Installation

```
git clone https://github.com/tomfevang/go-work
cd go-work
go build -o go-work .
```

## Usage

Run `./go-work` from a git repo root. The TUI will:

1. List open GitHub issues — select the ones you want to work on
2. Start a Claude Code session per issue; Claude drafts a plan
3. Review each plan and approve or reject it
4. Claude implements the approved plan in an isolated git worktree
5. A pull request is created automatically when the work is complete

## Key bindings

| Key | Action |
|-----|--------|
| `Space` | Toggle issue selection |
| `Enter` | Start sessions for selected issues |
| `Tab` | Switch panes |
| `y` | Approve plan |
| `r` | Reject plan |
| `q` / `Ctrl+C` | Quit |
