package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tomfevang/go-work/internal/session"
)

// issueItem wraps session.Issue so it satisfies the list.Item interface.
type issueItem struct {
	issue    session.Issue
	selected bool // checked for starting a session
}

func (i issueItem) Title() string       { return i.issue.Title }
func (i issueItem) Description() string { return truncate(i.issue.Body, 120) }
func (i issueItem) FilterValue() string { return fmt.Sprintf("#%d %s", i.issue.Number, i.issue.Title) }

// issueDelegate is a custom list delegate that renders each issue with a
// visible cursor, issue number, title, body preview and selection mark.
type issueDelegate struct{}

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	numberStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	titleNormal   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	titleFocused  = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	descNormal    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	descFocused   = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	checkSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	checkEmpty    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (d issueDelegate) Height() int  { return 3 }
func (d issueDelegate) Spacing() int { return 0 }
func (d issueDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d issueDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	iss, ok := item.(issueItem)
	if !ok {
		return
	}

	focused := index == m.Index()
	width := m.Width() - 4 // leave room for cursor + check columns

	// Cursor column (2 chars)
	cursor := "  "
	if focused {
		cursor = cursorStyle.Render("▶ ")
	}

	// Check column (2 chars)
	check := checkEmpty.Render("○ ")
	if iss.selected {
		check = checkSelected.Render("✓ ")
	}

	// Number
	num := numberStyle.Render(fmt.Sprintf("#%-4d", iss.issue.Number))

	// Title
	titleText := truncate(iss.issue.Title, width-6)
	var title string
	if focused {
		title = titleFocused.Render(titleText)
	} else {
		title = titleNormal.Render(titleText)
	}

	// Body preview
	bodyText := truncate(iss.issue.Body, width-6)
	var desc string
	if focused {
		desc = descFocused.Render(bodyText)
	} else {
		desc = descNormal.Render(bodyText)
	}

	line1 := cursor + check + num + " " + title
	line2 := "    " + desc // indent to align under title
	line3 := ""            // spacer

	fmt.Fprintln(w, line1)
	fmt.Fprintln(w, line2)
	fmt.Fprint(w, line3)
}

// issueSelectModel is the first screen: browse and multi-select GitHub issues.
type issueSelectModel struct {
	list     list.Model
	issues   []session.Issue
	selected map[int]bool
	width    int
	height   int
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func newIssueSelectModel(issues []session.Issue, width, height int) issueSelectModel {
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = issueItem{issue: iss}
	}

	listHeight := height - 3
	if listHeight < 1 {
		listHeight = 1
	}

	l := list.New(items, issueDelegate{}, width, listHeight)
	l.Title = "Select issues  —  space: toggle  enter: start  /: filter  q: quit"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return issueSelectModel{
		list:     l,
		issues:   issues,
		selected: make(map[int]bool),
		width:    width,
		height:   height,
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
		case " ":
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
	hint := statusStyle.Render(fmt.Sprintf("%d selected", m.countSelected()))
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
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
