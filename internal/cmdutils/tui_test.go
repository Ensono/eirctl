package cmdutils_test

import (
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/cmdutils"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func Test_TuiRun(t *testing.T) {
	items := []list.Item{
		cmdutils.NewItem("Option_1", "desc 1"),
		cmdutils.NewItem("Option_2", "desc 2"),
	}

	// Create the model
	m := cmdutils.NewTuiModel(items)

	// Create program
	p := tea.NewProgram(m, tea.WithoutRenderer(), tea.WithoutSignals(), tea.WithInput(nil)) // Headless mode

	// Simulate messages
	go func() {
		time.Sleep(20 * time.Millisecond)
		tmsg := tea.KeyMsg{}
		tmsg.Type = tea.KeyEnter
		p.Send(tmsg)
	}()

	// Start the program
	finalModel, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}

	// Assert selected and quitting state
	final, ok := finalModel.(cmdutils.TuiModel)
	if !ok {
		t.Fatal("type of final model is not correct ...")
	}
	if final.Selected() != "Option_1" {
		t.Errorf("got %s, wanted Option_1", final.Selected())
	}
}
