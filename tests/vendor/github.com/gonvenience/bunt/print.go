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
	"io"
)

func evaluateInputs(in ...interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, x := range in {
		switch obj := x.(type) {
		case string:
			result[i] = evaluateString(obj)
		default:
			result[i] = obj
		}
	}

	return result
}

func evaluateString(input string) string {
	if result, err := ParseString(input, ProcessTextAnnotations()); err == nil {
		return result.String()
	}

	return input
}

// Print wraps fmt.Print(a ...interface{}) and evaluates any text markers into its respective format
func Print(a ...interface{}) (n int, err error) {
	return fmt.Print(evaluateInputs(a...)...)
}

// Printf wraps fmt.Printf(format string, a ...interface{}) and evaluates any text markers into its respective format
func Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(evaluateString(format), a...)
}

// Println wraps fmt.Println(a ...interface{}) and evaluates any text markers into its respective format
func Println(a ...interface{}) (n int, err error) {
	return fmt.Println(evaluateInputs(a...)...)
}

// Fprint wraps fmt.Fprint(w io.Writer, a ...interface{}) and evaluates any text markers into its respective format
func Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	return fmt.Fprint(w, evaluateInputs(a...)...)
}

// Fprintf wraps fmt.Fprintf(w io.Writer, format string, a ...interface{}) and evaluates any text markers into its respective format
func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(w, evaluateString(format), a...)
}

// Fprintln wraps fmt.Fprintln(w io.Writer, a ...interface{}) and evaluates any text markers into its respective format
func Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	return fmt.Fprintln(w, evaluateInputs(a...)...)
}

// Sprint wraps fmt.Sprint(a ...interface{}) and evaluates any text markers into its respective format
func Sprint(a ...interface{}) string {
	return fmt.Sprint(evaluateInputs(a...)...)
}

// Sprintf wraps fmt.Sprintf(format string, a ...interface{}) and evaluates any text markers into its respective format
func Sprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(evaluateString(format), a...)
}

// Sprintln wraps fmt.Sprintln(a ...interface{}) and evaluates any text markers into its respective format
func Sprintln(a ...interface{}) string {
	return fmt.Sprintln(evaluateInputs(a...)...)
}
