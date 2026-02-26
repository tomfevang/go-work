package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tomfevang/go-work/internal/session"
)

// issueItem wraps session.Issue so it satisfies the list.Item interface.
type issueItem struct {
	issue    session.Issue
	selected bool
}

func (i issueItem) Title() string {
	mark := "  "
	if i.selected {
		mark = "✓ "
	}
	return fmt.Sprintf("%s#%d  %s", mark, i.issue.Number, i.issue.Title)
}
func (i issueItem) Description() string { return truncate(i.issue.Body, 80) }
func (i issueItem) FilterValue() string { return i.issue.Title }

// issueSelectModel is the first screen: browse and multi-select GitHub issues.
type issueSelectModel struct {
	list     list.Model
	issues   []session.Issue
	selected map[int]bool // issue number → selected
	width    int
	height   int
	err      error
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func newIssueSelectModel(issues []session.Issue) issueSelectModel {
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = issueItem{issue: iss}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select issues to work on"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return issueSelectModel{
		list:     l,
		issues:   issues,
		selected: make(map[int]bool),
	}
}

func (m issueSelectModel) Init() tea.Cmd { return nil }

func (m issueSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height-3)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case " ": // toggle selection
			idx := m.list.Index()
			item, ok := m.list.SelectedItem().(issueItem)
			if !ok {
				break
			}
			item.selected = !item.selected
			m.selected[item.issue.Number] = item.selected
			m.list.SetItem(idx, item)
			return m, nil

		case "enter":
			chosen := m.chosenIssues()
			if len(chosen) == 0 {
				// If nothing explicitly selected, use the highlighted item.
				if item, ok := m.list.SelectedItem().(issueItem); ok {
					chosen = []session.Issue{item.issue}
				}
			}
			if len(chosen) > 0 {
				return m, startSessionsCmd(chosen)
			}
			return m, nil

		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m issueSelectModel) View() string {
	count := len(m.selected)
	for _, v := range m.selected {
		if !v {
			count--
		}
	}
	hint := statusStyle.Render(fmt.Sprintf(
		"space: toggle  enter: start (%d selected)  q: quit",
		m.countSelected(),
	))
	return m.list.View() + "\n" + hint
}

func (m issueSelectModel) chosenIssues() []session.Issue {
	var out []session.Issue
	for _, iss := range m.issues {
		if m.selected[iss.Number] {
			out = append(out, iss)
		}
	}
	return out
}

func (m issueSelectModel) countSelected() int {
	n := 0
	for _, v := range m.selected {
		if v {
			n++
		}
	}
	return n
}

// startSessionsCmd is a Bubble Tea command that transitions to the dashboard.
func startSessionsCmd(issues []session.Issue) tea.Cmd {
	return func() tea.Msg {
		return startSessionsMsg{issues: issues}
	}
}

type startSessionsMsg struct {
	issues []session.Issue
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
