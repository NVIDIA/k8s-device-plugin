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
	"regexp"
	"strings"
	"unicode/utf8"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// StyleOption defines style option for colored strings
type StyleOption struct {
	flags       []string
	postProcess func(*String, map[string]struct{})
}

// PlainTextLength returns the length of the input text without any escape
// sequences.
func PlainTextLength(text string) int {
	return utf8.RuneCountInString(RemoveAllEscapeSequences(text))
}

// RemoveAllEscapeSequences return the input string with all escape sequences
// removed.
func RemoveAllEscapeSequences(input string) string {
	escapeSeqFinderRegExp := regexp.MustCompile(`\x1b\[([\d;]*)m`)

	for loc := escapeSeqFinderRegExp.FindStringIndex(input); loc != nil; loc = escapeSeqFinderRegExp.FindStringIndex(input) {
		start := loc[0]
		end := loc[1]
		input = strings.Replace(input, input[start:end], "", -1)
	}

	return input
}

// Substring returns a substring of a text that may contains escape sequences.
// The function will panic in the unlikely case of a parse issue.
func Substring(text string, start int, end int) string {
	result, err := ParseString(text)
	if err != nil {
		panic(err)
	}

	result.Substring(start, end)

	return result.String()
}

// Bold applies the bold text parameter
func Bold() StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			_, skipNewLine := flags["skipNewLine"]
			for i := range *s {
				if skipNewLine && (*s)[i].Symbol == '\n' {
					continue
				}

				(*s)[i].Settings |= 1 << 2
			}
		},
	}
}

// Italic applies the italic text parameter
func Italic() StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			_, skipNewLine := flags["skipNewLine"]
			for i := range *s {
				if skipNewLine && (*s)[i].Symbol == '\n' {
					continue
				}

				(*s)[i].Settings |= 1 << 3
			}
		},
	}
}

// Underline applies the underline text parameter
func Underline() StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			_, skipNewLine := flags["skipNewLine"]
			for i := range *s {
				if skipNewLine && (*s)[i].Symbol == '\n' {
					continue
				}

				(*s)[i].Settings |= 1 << 4
			}
		},
	}
}

// Foreground sets the given color as the foreground color of the text
func Foreground(color colorful.Color) StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			_, skipNewLine := flags["skipNewLine"]
			_, blendColors := flags["blendColors"]

			for i := range *s {
				if skipNewLine && (*s)[i].Symbol == '\n' {
					continue
				}

				r, g, b := color.RGB255()
				if blendColors {
					if fgColor := ((*s)[i].Settings >> 8 & 0xFFFFFF); fgColor != 0 {
						r, g, b = blend(r, g, b, fgColor)
					}
				}

				// reset currently set foreground color
				(*s)[i].Settings &= 0xFFFFFFFF000000FF

				(*s)[i].Settings |= 1
				(*s)[i].Settings |= uint64(r) << 8
				(*s)[i].Settings |= uint64(g) << 16
				(*s)[i].Settings |= uint64(b) << 24
			}
		},
	}
}

// ForegroundFunc uses the provided function to set an individual foreground
// color for each part of the text, which can be based on the position (x, y)
// or the content (rune) in the text.
func ForegroundFunc(f func(int, int, rune) *colorful.Color) StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			_, blendColors := flags["blendColors"]

			var x, y int
			for i, c := range *s {
				if c.Symbol == '\n' {
					x = 0
					y++
					continue
				}

				if color := f(x, y, c.Symbol); color != nil {
					r, g, b := color.RGB255()
					if blendColors {
						if fgColor := ((*s)[i].Settings >> 8 & 0xFFFFFF); fgColor != 0 {
							r, g, b = blend(r, g, b, fgColor)
						}
					}

					(*s)[i].Settings &= 0xFFFFFFFF000000FF
					(*s)[i].Settings |= 1
					(*s)[i].Settings |= uint64(r) << 8
					(*s)[i].Settings |= uint64(g) << 16
					(*s)[i].Settings |= uint64(b) << 24
				}

				x++
			}
		},
	}
}

// EnableTextAnnotations enables post-processing to evaluate text annotations
func EnableTextAnnotations() StyleOption {
	return StyleOption{
		postProcess: func(s *String, flags map[string]struct{}) {
			if err := processTextAnnotations(s); err != nil {
				panic(err)
			}
		},
	}
}

// EachLine enables that new line sequences will be ignored during coloring,
// which will lead to strings that are colored line by line and not as a block.
func EachLine() StyleOption {
	return StyleOption{
		flags: []string{"skipNewLine"},
	}
}

// Blend enables that applying a color does not completely reset an existing
// existing color, but rather mixes/blends both colors together.
func Blend() StyleOption {
	return StyleOption{
		flags: []string{"blendColors"},
	}
}

// Style is a multi-purpose function to programmatically apply styles and other
// changes to an input text. The function will panic in the unlikely case of a
// parse issue.
func Style(text string, styleOptions ...StyleOption) string {
	result, err := ParseString(text)
	if err != nil {
		panic(err)
	}

	flags := map[string]struct{}{}
	for _, styleOption := range styleOptions {
		for _, flag := range styleOption.flags {
			flags[flag] = struct{}{}
		}

		if styleOption.postProcess != nil {
			styleOption.postProcess(result, flags)
		}
	}

	return result.String()
}

func blend(r, g, b uint8, currentColor uint64) (uint8, uint8, uint8) {
	color1 := colorful.Color{
		R: float64(r),
		G: float64(g),
		B: float64(b),
	}

	color2 := colorful.Color{
		R: float64((currentColor >> 0) & 0xFF),
		G: float64((currentColor >> 8) & 0xFF),
		B: float64((currentColor >> 16) & 0xFF),
	}

	return color2.BlendLab(color1, 0.5).RGB255()
}
