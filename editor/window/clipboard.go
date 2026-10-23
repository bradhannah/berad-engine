package window

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const clipboardTimeout = 2 * time.Second

// systemClipboard reads and writes the operating system's clipboard through
// its command-line tools, behind the textedit.Clipboard interface. Where no
// tool is known it keeps text within the window, so copy and paste still work
// inside one field.
//
// A clipboard is useful to any Ebitengine program, so this is a candidate for
// berad-sdk once a second program needs it.
type systemClipboard struct {
	mu     sync.Mutex
	memory string
}

type clipboardCommand struct {
	read, write []string
}

func clipboardCommands() (clipboardCommand, bool) {
	switch runtime.GOOS {
	case "darwin":
		return clipboardCommand{read: []string{"pbpaste"}, write: []string{"pbcopy"}}, true
	case "windows":
		return clipboardCommand{
			read:  []string{"powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw"},
			write: []string{"powershell", "-NoProfile", "-Command", "$input | Set-Clipboard"},
		}, true
	default:
		if _, err := exec.LookPath("wl-paste"); err == nil {
			return clipboardCommand{read: []string{"wl-paste", "--no-newline"}, write: []string{"wl-copy"}}, true
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return clipboardCommand{
				read:  []string{"xclip", "-selection", "clipboard", "-o"},
				write: []string{"xclip", "-selection", "clipboard"},
			}, true
		}

		return clipboardCommand{read: nil, write: nil}, false
	}
}

// Read returns the clipboard's text.
func (c *systemClipboard) Read() (string, error) {
	commands, ok := clipboardCommands()
	if !ok {
		c.mu.Lock()
		defer c.mu.Unlock()

		return c.memory, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()
	// The commands are fixed platform clipboard tools, not user input.
	output, err := exec.CommandContext(ctx, commands.read[0], commands.read[1:]...).Output() //nolint:gosec // See above.
	if err != nil {
		return "", fmt.Errorf("read clipboard: %w", err)
	}

	return string(output), nil
}

// Write replaces the clipboard's text.
func (c *systemClipboard) Write(value string) error {
	commands, ok := clipboardCommands()
	if !ok {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.memory = value

		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()
	// The commands are fixed platform clipboard tools, not user input.
	cmd := exec.CommandContext(ctx, commands.write[0], commands.write[1:]...) //nolint:gosec // See above.
	cmd.Stdin = bytes.NewBufferString(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}

	return nil
}
