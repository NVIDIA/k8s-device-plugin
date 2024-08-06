// Copyright © 2019 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

/*
Package term contains convenience functions for terminal related tasks, for
example to get the current terminal width or a reasonable fallback.
*/
package term

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/mitchellh/go-ps"
	"golang.org/x/term"
)

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
)

// DefaultTerminalWidth is the default fallback terminal width.
const DefaultTerminalWidth = 80

// DefaultTerminalHeight is the default fallback terminal height.
const DefaultTerminalHeight = 25

// FixedTerminalWidth allows a manual fixed override of the terminal width.
var FixedTerminalWidth = -1

// FixedTerminalHeight allows a manual fixed override of the terminal height.
var FixedTerminalHeight = -1

// GetTerminalWidth return the terminal width (available characters per line)
func GetTerminalWidth() int {
	width, _ := GetTerminalSize()
	return width
}

// GetTerminalHeight return the terminal height (available lines).
func GetTerminalHeight() int {
	_, height := GetTerminalSize()
	return height
}

// GetTerminalSize return the terminal size as a width and height tuple. In
// case the terminal size cannot be determined, a reasonable default is
// used: 80x25. A manual override is possible using FixedTerminalWidth
// and FixedTerminalHeight.
func GetTerminalSize() (int, int) {
	// In case this is a garden container, disable the terminal size detection
	// and fall back to a reasonable assumption that is a bit bigger in size
	// than the default terminal fallback dimensions.
	if FixedTerminalWidth < 0 && FixedTerminalHeight < 0 && IsGardenContainer() {
		FixedTerminalWidth = 120
		FixedTerminalHeight = 25
	}

	// Return user preference (explicit overwrite) of both width and height
	if FixedTerminalWidth > 0 && FixedTerminalHeight > 0 {
		return FixedTerminalWidth, FixedTerminalHeight
	}

	width, height, err := term.GetSize(int(os.Stdout.Fd()))

	switch {
	// Return default fallback value
	case err != nil:
		return DefaultTerminalWidth, DefaultTerminalHeight

	// Return user preference of width, actual value for height
	case FixedTerminalWidth > 0:
		return FixedTerminalWidth, height

	// Return user preference of height, actual value for width
	case FixedTerminalHeight > 0:
		return width, FixedTerminalHeight

	// Return actual determined values
	default:
		return width, height
	}
}

// IsDumbTerminal returns whether the current terminal has a limited feature set
func IsDumbTerminal() bool {
	return os.Getenv("TERM") == "dumb"
}

// IsTerminal returns whether this program runs in a terminal and not in a pipe
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) ||
		isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// IsTrueColor returns whether the current terminal supports 24 bit colors
func IsTrueColor() bool {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return true

	default:
		return false
	}
}

// IsGardenContainer returns whether the current process is started in the
// process tree of garden container (https://github.com/cloudfoundry/garden).
func IsGardenContainer() bool {
	if process, err := ps.FindProcess(1); err == nil && process != nil {
		switch process.Executable() {
		case "garden-init":
			return true
		}
	}

	return false
}

// HideCursor sends the ANSI sequence to hide the cursor symbol
func HideCursor() {
	fmt.Print(hideCursor)
}

// ShowCursor sends the ANSI sequence to show the cursor symbol
func ShowCursor() {
	fmt.Print(showCursor)
}
