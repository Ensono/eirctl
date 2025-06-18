package cmdutils

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type item struct {
	title       string
	description string
}

func NewItem(title, desc string) list.Item {
	return item{title, desc}
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.title }

type TuiModel struct {
	list               list.Model
	delegateKeys       *delegateKeyMap
	selected           string
	quitting           bool
	appStyle           lipgloss.Style
	statusMessageStyle lipgloss.Style
}

func (m TuiModel) Selected() string {
	return m.selected
}

func NewTuiModel(initialItems []list.Item) TuiModel {
	delegateKeys := newDelegateKeyMap()
	tm := TuiModel{
		delegateKeys: delegateKeys,
		appStyle:     lipgloss.NewStyle().Padding(1, 2),
		statusMessageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}),
	}

	// Setup TUI
	delegate := newItemDelegate(delegateKeys)
	optionsList := list.New(initialItems, delegate, 0, 0)
	optionsList.Title = "Select the pipeline or task to run"
	optionsList.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#25A065")).
		Padding(0, 1)

	tm.list = optionsList
	return tm
}

func (m TuiModel) Init() tea.Cmd {
	return nil
}

func (m TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := m.appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.delegateKeys.choose):
			if selectedItem, ok := m.list.SelectedItem().(item); ok {
				m.selected = selectedItem.Title()
				m.quitting = true

				// set a message for user about which task/pipeline runs
				statusCmd := m.list.NewStatusMessage(m.statusMessageStyle.Render("Running " + m.selected + "..."))

				// Quit after 1000 milliseconds (enough time for the message to render)
				quitCmd := tea.Tick(time.Millisecond*1000, func(t time.Time) tea.Msg {
					return tea.Quit()
				})

				return m, tea.Batch(statusCmd, quitCmd)
			}
		}
	}

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m TuiModel) View() string {
	return m.appStyle.Render(m.list.View())
}

func newItemDelegate(keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		return nil
	}

	help := []key.Binding{keys.choose}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

type delegateKeyMap struct {
	choose key.Binding
}

// Additional short help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		d.choose,
	}
}

// Additional full help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			d.choose,
		},
	}
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
	}
}

func TuiRun(initialModel TuiModel) (string, error) {
	final, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if m, ok := final.(TuiModel); ok && m.quitting {
		return m.selected, nil
	}

	return "", nil
}
