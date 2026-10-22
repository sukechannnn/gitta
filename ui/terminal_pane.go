package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sukechannnn/giff/util"
)

// The output pane can only be scrolled while it holds the focus, so Tab
// moves between typing and reading and the title says which one is active.
const (
	terminalInputTitle  = " Terminal (Tab: scroll output, Esc: close) "
	terminalOutputTitle = " Terminal output (j/k: scroll, Tab: back to input, Esc: close) "
)

// terminalPane is the 't' command line: an input line over the output of the
// last command, docked at the bottom of a view while it is open
type terminalPane struct {
	app      *tview.Application
	repoRoot string

	flex   *tview.Flex
	input  *tview.InputField
	output *tview.TextView

	history      []string
	historyIndex int    // -1 = not browsing history
	historyStash string // stash current input when browsing

	host        *tview.Flex     // the view the pane is docked in, nil while closed
	focusBefore tview.Primitive // focused before the pane opened, focused again on close
	onDone      func()          // called after each command, e.g. to refresh the view
}

func newTerminalPane(app *tview.Application, repoRoot string) *terminalPane {
	t := &terminalPane{
		app:          app,
		repoRoot:     repoRoot,
		history:      util.LoadShellHistory(1000),
		historyIndex: -1,
	}

	t.input = tview.NewInputField().
		SetLabel("(giff) $ ").
		SetFieldBackgroundColor(util.BackgroundColor.ToTcellColor()).
		SetLabelStyle(tcell.StyleDefault.
			Foreground(tcell.ColorAqua).
			Background(util.BackgroundColor.ToTcellColor()))
	t.input.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	t.input.SetBorder(false)

	t.output = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetScrollable(true)
	t.output.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	t.output.SetBorder(false)

	t.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.input, 1, 0, true).
		AddItem(t.output, 0, 1, false)
	t.flex.SetBackgroundColor(util.BackgroundColor.ToTcellColor())
	t.flex.SetBorder(true)
	t.flex.SetBorderColor(util.CommitAreaBorderColor.ToTcellColor())
	t.flex.SetTitleAlign(tview.AlignLeft)
	t.flex.SetTitleColor(tcell.ColorWhite)
	t.flex.SetTitle(terminalInputTitle)

	t.setupKeyBindings()
	return t
}

// Open docks the pane at the bottom of host and focuses its input line. The
// focus goes back to focusBefore on close, and onDone runs after each command.
func (t *terminalPane) Open(host *tview.Flex, focusBefore tview.Primitive, onDone func()) {
	if t.host != nil {
		t.focusInput()
		return
	}
	t.host = host
	t.focusBefore = focusBefore
	t.onDone = onDone
	t.output.Clear()
	t.input.SetText("")
	host.AddItem(t.flex, 12, 0, true)
	t.focusInput()
}

func (t *terminalPane) close() {
	if t.host == nil {
		return
	}
	t.host.RemoveItem(t.flex)
	t.host = nil
	if t.focusBefore != nil {
		t.app.SetFocus(t.focusBefore)
	}
}

func (t *terminalPane) focusInput() {
	t.flex.SetTitle(terminalInputTitle)
	t.app.SetFocus(t.input)
}

func (t *terminalPane) focusOutput() {
	t.flex.SetTitle(terminalOutputTitle)
	t.app.SetFocus(t.output)
}

func (t *terminalPane) setupKeyBindings() {
	// The output pane borrows the focus only to be scrolled: Tab hands it back
	// and Esc closes the terminal, the same as from the input line.
	t.output.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			t.focusInput()
			return nil
		case tcell.KeyEsc:
			t.close()
			return nil
		}
		return event
	})

	// Handle up/down arrow for shell history browsing
	t.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyBacktab:
			// Move to the output so it can be scrolled; typing resumes on Tab.
			t.focusOutput()
			return nil
		case tcell.KeyUp:
			if len(t.history) == 0 {
				return nil
			}
			if t.historyIndex == -1 {
				// Start browsing: stash current input as prefix filter
				t.historyStash = t.input.GetText()
			}
			// Find next matching entry
			prefix := strings.ToLower(t.historyStash)
			for next := t.historyIndex + 1; next < len(t.history); next++ {
				if prefix == "" || strings.HasPrefix(strings.ToLower(t.history[next]), prefix) {
					t.historyIndex = next
					t.input.SetText(t.history[next])
					return nil
				}
			}
			return nil
		case tcell.KeyDown:
			if t.historyIndex < 0 {
				return nil
			}
			// Find previous matching entry
			prefix := strings.ToLower(t.historyStash)
			for next := t.historyIndex - 1; next >= 0; next-- {
				if prefix == "" || strings.HasPrefix(strings.ToLower(t.history[next]), prefix) {
					t.historyIndex = next
					t.input.SetText(t.history[next])
					return nil
				}
			}
			// No more matches: back to stashed input
			t.historyIndex = -1
			t.input.SetText(t.historyStash)
			return nil
		}
		return event
	})

	t.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			cmdText := t.input.GetText()
			if cmdText == "" {
				return
			}
			t.input.SetText("")
			t.historyIndex = -1
			t.historyStash = ""
			t.run(cmdText)
		case tcell.KeyEscape:
			t.close()
		}
	})
}

// run executes the command in the user's shell without blocking the UI and
// shows its output
func (t *terminalPane) run(cmdText string) {
	// Show running state
	t.output.Clear()
	t.output.SetText("[dimgray]running...[-]")
	t.output.ScrollToBeginning()

	go func() {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		var cmdScript string
		if snapPath := util.GetSnapshotPath(); snapPath != "" {
			cmdScript = fmt.Sprintf("source %q 2>/dev/null; eval %q", snapPath, cmdText)
		} else {
			cmdScript = cmdText
		}
		cmd := exec.Command(shell, "-c", cmdScript)
		cmd.Dir = t.repoRoot
		output, err := cmd.CombinedOutput()

		t.app.QueueUpdateDraw(func() {
			// Clear and show result
			t.output.Clear()
			var result strings.Builder
			result.WriteString("[#50E88A]$ " + tview.Escape(cmdText) + "[-]\n")
			if err != nil {
				if len(output) > 0 {
					result.WriteString("[red]" + tview.Escape(string(output)) + "[-]")
					if output[len(output)-1] != '\n' {
						result.WriteString("\n")
					}
				}
				result.WriteString("[red]exit: " + err.Error() + "[-]")
			} else if len(output) > 0 {
				result.WriteString(tview.Escape(string(output)))
				if output[len(output)-1] != '\n' {
					result.WriteString("\n")
				}
			}
			t.output.SetText(result.String())
			t.output.ScrollToBeginning()

			if t.onDone != nil {
				t.onDone()
			}
		})
	}()
}
