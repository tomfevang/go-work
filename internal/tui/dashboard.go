package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tomfevang/go-work/internal/session"
)

const leftPaneWidth = 32

var (
	sessionListStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	approveBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("220")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Padding(0, 1)

	doneBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	badgePending  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render
	badgePlanning = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render
	badgeApprove  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render
	badgeImpl     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render
	badgeDone     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render
	badgeFailed   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render
)

// sessionListItem wraps a session number for the left-pane list.
type sessionListItem struct {
	issueNum int
	title    string
	state    session.State
}

func (s sessionListItem) Title() string {
	badge := renderBadge(s.state)
	return fmt.Sprintf("%s #%d", badge, s.issueNum)
}
func (s sessionListItem) Description() string {
	return truncate(s.title, leftPaneWidth-4)
}
func (s sessionListItem) FilterValue() string {
	return fmt.Sprintf("#%d %s", s.issueNum, s.title)
}

func renderBadge(state session.State) string {
	switch state {
	case session.Pending:
		return badgePending("○")
	case session.Planning:
		return badgePlanning("⟳")
	case session.WaitingApproval:
		return badgeApprove("⏸")
	case session.Implementing:
		return badgeImpl("●")
	case session.CreatingPR:
		return badgeImpl("↑")
	case session.Done:
		return badgeDone("✓")
	case session.Failed:
		return badgeFailed("✗")
	default:
		return "?"
	}
}

// dashboardModel is the main screen after sessions are started.
type dashboardModel struct {
	sessions    map[int]*session.Session // keyed by issue number
	order       []int                    // issue numbers in display order
	approveChs  map[int]chan bool
	eventCh     chan session.Event
	list        list.Model
	viewport    viewport.Model
	width       int
	height      int
	focusedPane int // 0 = list, 1 = viewport
}

func newDashboard(
	sessions map[int]*session.Session,
	order []int,
	approveChs map[int]chan bool,
	eventCh chan session.Event,
	width, height int,
) dashboardModel {
	items := make([]list.Item, len(order))
	for i, num := range order {
		s := sessions[num]
		items[i] = sessionListItem{issueNum: num, title: s.Issue.Title, state: s.State}
	}

	l := list.New(items, list.NewDefaultDelegate(), leftPaneWidth, height-2)
	l.Title = "Sessions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	vpWidth := width - leftPaneWidth - 6
	if vpWidth < 10 {
		vpWidth = 10
	}
	vp := viewport.New(vpWidth, height-4)
	vp.SetContent("")

	return dashboardModel{
		sessions:   sessions,
		order:      order,
		approveChs: approveChs,
		eventCh:    eventCh,
		list:       l,
		viewport:   vp,
		width:      width,
		height:     height,
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return waitForEvent(m.eventCh)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		vpWidth := msg.Width - leftPaneWidth - 6
		if vpWidth < 10 {
			vpWidth = 10
		}
		m.list.SetSize(leftPaneWidth, msg.Height-2)
		m.viewport.Width = vpWidth
		m.viewport.Height = msg.Height - 4
		return m, nil

	case session.Event:
		s, ok := m.sessions[msg.IssueNumber]
		if !ok {
			return m, waitForEvent(m.eventCh)
		}

		switch msg.Type {
		case session.EventOutput:
			s.Log += msg.Text
		case session.EventPlanDone:
			s.Plan = msg.Text
			s.State = session.WaitingApproval
		case session.EventImplDone:
			s.State = session.CreatingPR
		case session.EventPRDone:
			s.PR = msg.Text
			s.State = session.Done
			s.Log += "\n✓ PR: " + msg.Text + "\n"
		case session.EventError:
			s.Err = fmt.Errorf("%s", msg.Text)
			s.State = session.Failed
			s.Log += "\n✗ Error: " + msg.Text + "\n"
		}

		m.syncListItem(msg.IssueNumber)
		m.refreshViewport()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.focusedPane = 1 - m.focusedPane
			return m, nil

		case "y":
			// Approve the plan for the selected session.
			if num, ok := m.selectedIssueNum(); ok {
				s := m.sessions[num]
				if s.State == session.WaitingApproval {
					s.State = session.Implementing
					m.syncListItem(num)
					if ch, ok := m.approveChs[num]; ok {
						ch <- true
					}
				}
			}
			return m, nil

		case "r":
			// Reject the plan for the selected session.
			if num, ok := m.selectedIssueNum(); ok {
				s := m.sessions[num]
				if s.State == session.WaitingApproval {
					s.State = session.Failed
					s.Log += "\n✗ Plan rejected by user.\n"
					m.syncListItem(num)
					if ch, ok := m.approveChs[num]; ok {
						ch <- false
					}
				}
			}
			return m, nil
		}

		// Route keyboard to focused pane.
		if m.focusedPane == 0 {
			var cmd tea.Cmd
			prev := m.list.Index()
			m.list, cmd = m.list.Update(msg)
			if m.list.Index() != prev {
				m.refreshViewport()
			}
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m dashboardModel) View() string {
	// Left pane.
	left := sessionListStyle.
		Width(leftPaneWidth).
		Height(m.height - 2).
		Render(m.list.View())

	// Right pane: viewport + footer.
	vpContent := m.viewport.View()
	footer := m.footerView()
	right := viewportStyle.
		Width(m.width - leftPaneWidth - 6).
		Height(m.height - 2).
		Render(vpContent + "\n" + footer)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m dashboardModel) footerView() string {
	num, ok := m.selectedIssueNum()
	if !ok {
		return ""
	}
	s := m.sessions[num]
	switch s.State {
	case session.WaitingApproval:
		return approveBarStyle.Render("  [y] Approve plan    [r] Reject  ")
	case session.Done:
		pr := s.PR
		if pr == "" {
			pr = "PR created"
		}
		return doneBarStyle.Render("✓ " + pr)
	default:
		return statusStyle.Render(s.State.String() + "…")
	}
}

func (m *dashboardModel) syncListItem(issueNum int) {
	for i, num := range m.order {
		if num == issueNum {
			s := m.sessions[num]
			m.list.SetItem(i, sessionListItem{
				issueNum: num,
				title:    s.Issue.Title,
				state:    s.State,
			})
			return
		}
	}
}

func (m *dashboardModel) refreshViewport() {
	num, ok := m.selectedIssueNum()
	if !ok {
		m.viewport.SetContent("")
		return
	}
	s := m.sessions[num]
	content := s.Log
	if s.State == session.WaitingApproval && s.Plan != "" {
		content = "=== PLAN ===\n\n" + s.Plan + "\n\n" + strings.Repeat("─", 40) + "\n"
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m dashboardModel) selectedIssueNum() (int, bool) {
	item, ok := m.list.SelectedItem().(sessionListItem)
	if !ok {
		return 0, false
	}
	return item.issueNum, true
}

// waitForEvent returns a Cmd that blocks until the next Event arrives.
func waitForEvent(ch chan session.Event) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
