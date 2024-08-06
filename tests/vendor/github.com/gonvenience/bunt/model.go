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

// String is a string with color information
type String []ColoredRune

// ColoredRune is a rune with additional color information.
//
// Bit details:
// - 1st bit, foreground color on/off
// - 2nd bit, background color on/off
// - 3rd bit, bold on/off
// - 4th bit, italic on/off
// - 5th bit, underline on/off
// - 6th-8th bit, unused/reserved
// - 9th-32nd bit, 24 bit RGB foreground color
// - 33rd-56th bit, 24 bit RGB background color
// - 57th-64th bit, unused/reserved
type ColoredRune struct {
	Symbol   rune
	Settings uint64
}

// Substring cuts the String to a sub-string using the provided absolute start
// and end indicies.
func (s *String) Substring(from, to int) {
	*s = (*s)[from:to]
}
