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

package dyff

import (
	"io"

	"github.com/gonvenience/ytbx"
	yamlv3 "gopkg.in/yaml.v3"
)

// Constants to distinguish between the different kinds of differences
const (
	ADDITION     = '+'
	REMOVAL      = '-'
	MODIFICATION = '±'
	ORDERCHANGE  = '⇆'
	// ILLEGAL      = '✕'
	// ATTENTION    = '⚠'
)

// Detail encapsulate the actual details of a change, mainly the kind of
// difference and the values
type Detail struct {
	From *yamlv3.Node
	To   *yamlv3.Node
	Kind rune
}

// Diff encapsulates everything noteworthy about a difference
type Diff struct {
	Path    *ytbx.Path
	Details []Detail
}

// Report encapsulates the actual end-result of the comparison: The input data
// and the list of differences
type Report struct {
	From  ytbx.InputFile
	To    ytbx.InputFile
	Diffs []Diff
}

// ReportWriter defines the interface required for types that can write reports
type ReportWriter interface {
	WriteReport(out io.Writer) error
}
