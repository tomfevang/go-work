package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/tomfevang/go-work/internal/github"
	"github.com/tomfevang/go-work/internal/session"
)

// appModel is the root model that owns screen transitions.
type appModel struct {
	current  tea.Model
	repoRoot string
	width    int
	height   int
}

// issuesLoadedMsg carries the result of the initial issue fetch.
type issuesLoadedMsg struct {
	issues []session.Issue
	err    error
}

// New creates and returns the root Bubble Tea program.
func New(repoRoot string) *tea.Program {
	m := appModel{repoRoot: repoRoot}
	return tea.NewProgram(m, tea.WithAltScreen())
}

func (m appModel) Init() tea.Cmd {
	return fetchIssuesCmd()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.current != nil {
			updated, cmd := m.current.Update(msg)
			m.current = updated
			return m, cmd
		}
		return m, nil

	case issuesLoadedMsg:
		if msg.err != nil {
			fmt.Fprintf(os.Stderr, "error loading issues: %v\n", msg.err)
			return m, tea.Quit
		}
		sel := newIssueSelectModel(msg.issues)
		m.current = sel
		return m, sel.Init()

	case startSessionsMsg:
		return m.startSessions(msg.issues)
	}

	if m.current != nil {
		updated, cmd := m.current.Update(msg)
		m.current = updated
		return m, cmd
	}
	return m, nil
}

func (m appModel) View() string {
	if m.current == nil {
		return "Loading issues…\n"
	}
	return m.current.View()
}

// startSessions initialises session state, kicks off goroutines, and switches
// to the dashboard screen.
func (m appModel) startSessions(issues []session.Issue) (tea.Model, tea.Cmd) {
	eventCh := make(chan session.Event, 64)
	sessions := make(map[int]*session.Session, len(issues))
	approveChs := make(map[int]chan bool, len(issues))
	order := make([]int, len(issues))

	for i, iss := range issues {
		sessions[iss.Number] = &session.Session{
			Issue: iss,
			State: session.Pending,
		}
		approveChs[iss.Number] = make(chan bool, 1)
		order[i] = iss.Number
	}

	for _, iss := range issues {
		iss := iss
		approveCh := approveChs[iss.Number]
		sessions[iss.Number].State = session.Planning
		go session.Run(iss, m.repoRoot, eventCh, approveCh)
	}

	dash := newDashboard(sessions, order, approveChs, eventCh, m.width, m.height)
	m.current = dash
	return m, dash.Init()
}

// fetchIssuesCmd returns a Cmd that calls gh issue list.
func fetchIssuesCmd() tea.Cmd {
	return func() tea.Msg {
		issues, err := gh.ListIssues()
		return issuesLoadedMsg{issues: issues, err: err}
	}
}
