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

import "fmt"

// EmptyTableError is used to describe that the input table was either nil, or empty.
type EmptyTableError struct {
}

func (e *EmptyTableError) Error() string {
	return "unable to render table, the input table is empty"
}

// ImbalancedTableError is used to describe that not all rows have the same number of columns
type ImbalancedTableError struct {
}

func (e *ImbalancedTableError) Error() string {
	return "unable to render table, some rows have more or less columns than other rows"
}

// RowLengthExceedsDesiredWidthError is used to describe that the table cannot be rendered, because at least one row exceeds the desired width
type RowLengthExceedsDesiredWidthError struct {
}

func (e *RowLengthExceedsDesiredWidthError) Error() string {
	return "unable to render table, because at least one row exceeds the desired width"
}

// ColumnIndexIsOutOfBoundsError is used to describe that a provided column index is out of bounds
type ColumnIndexIsOutOfBoundsError struct {
	ColumnIdx int
}

func (e *ColumnIndexIsOutOfBoundsError) Error() string {
	return fmt.Sprintf("unable to render table, the provided column index %d is out of bounds", e.ColumnIdx)
}
