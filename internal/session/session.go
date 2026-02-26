package session

import gh "github.com/tomfevang/go-work/internal/github"

// Issue is an alias for github.Issue for convenience within this package.
type Issue = gh.Issue

// State represents the lifecycle stage of a session.
type State int

const (
	Pending         State = iota // queued, not yet started
	Planning                     // claude is generating a plan
	WaitingApproval              // plan ready, awaiting user decision
	Implementing                 // claude is implementing the approved plan
	CreatingPR                   // running gh pr create
	Done                         // PR opened successfully
	Failed                       // terminal error
)

func (s State) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Planning:
		return "Planning"
	case WaitingApproval:
		return "Needs approval"
	case Implementing:
		return "Implementing"
	case CreatingPR:
		return "Creating PR"
	case Done:
		return "Done"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// EventType classifies messages sent from a runner goroutine to the TUI.
type EventType int

const (
	EventOutput   EventType = iota // new text output to append to the log
	EventPlanDone                  // planning phase complete, plan text ready
	EventImplDone                  // implementation phase complete
	EventPRDone                    // PR created, URL in Text
	EventError                     // unrecoverable error
)

// Event is sent from a runner goroutine to the TUI via a shared channel.
type Event struct {
	IssueNumber int
	Type        EventType
	Text        string
}

// Session tracks one Claude Code session working on a single issue.
type Session struct {
	Issue Issue
	State State
	Plan  string // accumulated plan text (set after planning phase)
	Log   string // full streamed output
	PR    string // PR URL (set after Done)
	Err   error
}

// Badge returns a short status indicator for display in the session list.
func (s *Session) Badge() string {
	switch s.State {
	case Pending:
		return "○"
	case Planning:
		return "⟳"
	case WaitingApproval:
		return "⏸"
	case Implementing:
		return "●"
	case CreatingPR:
		return "↑"
	case Done:
		return "✓"
	case Failed:
		return "✗"
	default:
		return "?"
	}
}
