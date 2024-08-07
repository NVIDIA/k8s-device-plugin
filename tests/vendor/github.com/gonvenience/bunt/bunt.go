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

package bunt

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/gonvenience/term"
	"github.com/mattn/go-isatty"
)

// Internal bit mask to mark feature states, e.g. foreground coloring
const (
	fgMask        = 0x1
	bgMask        = 0x2
	boldMask      = 0x4
	italicMask    = 0x8
	underlineMask = 0x10
)

// ColorSetting defines the coloring setting to be used
var ColorSetting SwitchState = SwitchState{value: AUTO}

// TrueColorSetting defines the true color usage setting to be used
var TrueColorSetting SwitchState = SwitchState{value: AUTO}

type state int

// Supported setting states
const (
	AUTO = state(iota)
	ON
	OFF
)

// SwitchState is the type to cover different preferences/settings like: on, off, or auto
type SwitchState struct {
	sync.Mutex
	value state
}

func (s state) String() string {
	switch s {
	case ON:
		return "on"

	case OFF:
		return "off"

	case AUTO:
		return "auto"
	}

	panic("unsupported state")
}

func (s *SwitchState) String() string {
	return s.value.String()
}

// Set updates the switch state based on the provided setting, or fails with an
// error in case the setting cannot be parsed
func (s *SwitchState) Set(setting string) error {
	s.Lock()
	defer s.Unlock()

	switch strings.ToLower(setting) {
	case "auto":
		s.value = AUTO

	case "off", "no", "false":
		s.value = OFF

	case "on", "yes", "true":
		s.value = ON

	default:
		return fmt.Errorf("invalid state '%s' used, supported modes are: auto, on, or off", setting)
	}

	return nil
}

// Type returns the type name of switch state, which is an empty string for now
func (s *SwitchState) Type() string {
	return ""
}

// UseColors return whether colors are used or not based on the configured color
// setting, operating system, and terminal capabilities
func UseColors() bool {
	ColorSetting.Lock()
	defer ColorSetting.Unlock()

	// Configured overrides take precedence
	switch ColorSetting.value {
	case ON:
		return true

	case OFF:
		return false
	}

	// Windows in non Cygwin environments is (currently) not supported
	if runtime.GOOS == "windows" && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return false
	}

	// In case StdOut is not a dumb terminal, assume colors can be used
	return term.IsTerminal() && !term.IsDumbTerminal()
}

// UseTrueColor returns whether true color colors should be used or not based on
// the configured true color usage setting or terminal capabilities
func UseTrueColor() bool {
	TrueColorSetting.Lock()
	defer TrueColorSetting.Unlock()

	return (TrueColorSetting.value == ON) ||
		(TrueColorSetting.value == AUTO && term.IsTrueColor())
}

// SetColorSettings is a convenience function to set both color settings at the
// same time using the internal locks
func SetColorSettings(color state, trueColor state) {
	ColorSetting.Lock()
	defer ColorSetting.Unlock()
	ColorSetting.value = color

	TrueColorSetting.Lock()
	defer TrueColorSetting.Unlock()
	TrueColorSetting.value = trueColor
}
