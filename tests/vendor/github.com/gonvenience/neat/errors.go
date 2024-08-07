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

package neat

import (
	"fmt"
	"io"
	"strings"

	"github.com/gonvenience/bunt"
	"github.com/gonvenience/wrap"
)

// PrintError prints the provided error to stdout
func PrintError(err error) {
	fmt.Print(SprintError(err))
}

// FprintError prints the provided error to the provided writer
func FprintError(w io.Writer, err error) {
	fmt.Fprint(w, SprintError(err))
}

// SprintError prints the provided error as a string
func SprintError(err error) string {
	switch e := err.(type) {
	case wrap.ContextError:
		var content string
		switch e.Cause().(type) {
		case wrap.ContextError:
			content = SprintError(e.Cause())

		default:
			content = e.Cause().Error()
		}

		return ContentBox(
			fmt.Sprintf("Error: %s", e.Context()),
			content,
			HeadlineColor(bunt.OrangeRed),
			ContentColor(bunt.Red),
		)

	default:
		if parts := strings.SplitN(err.Error(), ":", 2); len(parts) == 2 {
			message, cause := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			return ContentBox(
				fmt.Sprintf("Error: %s", message),
				cause,
				HeadlineColor(bunt.OrangeRed),
				ContentColor(bunt.Red),
			)
		}

		return ContentBox(
			"Error",
			err.Error(),
			HeadlineColor(bunt.OrangeRed),
			ContentColor(bunt.Red),
		)
	}
}
