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

package wrap

import (
	"errors"
	"fmt"
	"strings"
)

// ContextError interface describes the simple type that is able to provide a
// textual context as well as the cause explaining the underlying error.
type ContextError interface {
	Context() string
	Cause() error
}

// wrappedError describes an error with added context information
type wrappedError struct {
	context string
	cause   error
}

func (e *wrappedError) Error() string {
	return fmt.Sprintf("%s: %v", e.context, e.cause)
}

func (e *wrappedError) Context() string {
	return e.context
}

func (e *wrappedError) Cause() error {
	return e.cause
}

// ListOfErrors interface describes a list of errors with additional context
// information with an explanation.
type ListOfErrors interface {
	Context() string
	Errors() []error
}

// wrappedErrors describes a list of errors with context information
type wrappedErrors struct {
	context string
	errors  []error
}

func (e *wrappedErrors) Error() string {
	tmp := make([]string, len(e.errors))
	for i, err := range e.errors {
		tmp[i] = fmt.Sprintf("- %s", err.Error())
	}

	return fmt.Sprintf("%s:\n%s", e.context, strings.Join(tmp, "\n"))
}

func (e *wrappedErrors) Context() string {
	return e.context
}

func (e *wrappedErrors) Errors() []error {
	return e.errors
}

// Error creates an error with additional context
func Error(err error, context string) error {
	switch {
	case err == nil:
		return errors.New(context)

	default:
		return &wrappedError{context, err}
	}
}

// Errorf creates an error with additional formatted context
func Errorf(err error, format string, a ...interface{}) error {
	return Error(err, fmt.Sprintf(format, a...))
}

// Errors creates a list of errors with additional context
func Errors(errs []error, context string) error {
	switch {
	case errs == nil:
		return errors.New(context)

	case len(errs) == 1:
		return Error(errs[0], context)

	default:
		return &wrappedErrors{context, errs}
	}
}

// Errorsf creates a list of errors with additional formatted context
func Errorsf(errors []error, format string, a ...interface{}) error {
	return Errors(errors, fmt.Sprintf(format, a...))
}
