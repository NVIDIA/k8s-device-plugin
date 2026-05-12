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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	escapeSeqRegExp = regexp.MustCompile(`\x1b\[(\d+(;\d+)*)m`)
	boldMarker      = regexp.MustCompile(`\*([^*]+?)\*`)
	italicMarker    = regexp.MustCompile(`_([^_]+?)_`)
	underlineMarker = regexp.MustCompile(`~([^~]+?)~`)
	colorMarker     = regexp.MustCompile(`(#?\w+)\{([^}]+?)\}`)
)

// ParseOption defines parser options
type ParseOption func(*String) error

// ProcessTextAnnotations specifies whether during parsing bunt-specific text
// annotations like *bold*, or _italic_ should be processed.
func ProcessTextAnnotations() ParseOption {
	return func(s *String) error {
		return processTextAnnotations(s)
	}
}

// ParseStream reads from the input reader and parses all supported ANSI
// sequences that are relevant for colored strings.
//
// Please notes: This function is still under development and should replace
// the `ParseString` function code eventually.
func ParseStream(in io.Reader, opts ...ParseOption) (*String, error) {
	var input *bufio.Reader
	switch typed := in.(type) {
	case *bufio.Reader:
		input = typed

	default:
		input = bufio.NewReader(in)
	}

	type seq struct {
		values string
		suffix rune
	}

	var readSGR = func() seq {
		var buf bytes.Buffer
		for {
			r, _, err := input.ReadRune()
			if err == io.EOF {
				break
			}

			switch r {
			case 'h', 'l', 'm', 'r', 'A', 'B', 'C', 'D', 'H', 'f', 'g', 'K', 'J', 'y', 'q':
				return seq{
					values: buf.String(),
					suffix: r,
				}

			default:
				buf.WriteRune(r)
			}
		}

		panic("failed to parse ANSI sequence")
	}

	var skipUntil = func(end rune) {
		for {
			r, _, err := input.ReadRune()
			if err == io.EOF {
				panic("reached end of file before reaching end identifier")
			}

			if r == end {
				return
			}
		}
	}

	var asUint = func(s string) uint {
		tmp, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			panic(err)
		}

		return uint(tmp)
	}

	var result String
	var line String
	var lineIdx uint
	var lineCap uint
	var settings uint64

	var readANSISeq = func() {
		r, _, err := input.ReadRune()
		if err == io.EOF {
			return
		}

		// https://en.wikipedia.org/wiki/ANSI_escape_code#Escape_sequences
		switch r {
		case '[':
			var seq = readSGR()
			switch seq.suffix {
			case 'm': // colors
				settings, err = parseSelectGraphicRenditionEscapeSequence(seq.values)
				if err != nil {
					panic(err)
				}

			case 'D': // move cursor left n lines
				n := asUint(seq.values)
				if n > lineIdx {
					panic("move cursor value is out of bounds")
				}

				lineIdx -= n

			case 'K': // clear line
				var start, end uint
				switch seq.values {
				case "", "0":
					start, end = lineIdx, lineCap

				case "1":
					start, end = 0, lineIdx

				case "2":
					start, end = 0, lineCap
				}

				for i := start; i < end; i++ {
					line[i].Symbol = ' '
					line[i].Settings = 0
				}

			default:
				// ignoring all other sequences
			}

		case ']':
			skipUntil('\a')
		}
	}

	var add = func(r rune) {
		var cr = ColoredRune{Symbol: r, Settings: settings}

		if lineIdx < lineCap {
			line[lineIdx] = cr
			lineIdx++

		} else {
			line = append(line, cr)
			lineIdx++
			lineCap++
		}
	}

	var del = func() {
		line = line[:len(line)-1]
		lineIdx--
		lineCap--
	}

	var flush = func() {
		// Prepare to remove trailing spaces by finding the last non-space rune
		var endIdx = len(line) - 1
		for ; endIdx >= 0; endIdx-- {
			if line[endIdx].Symbol != ' ' {
				break
			}
		}

		result = append(result, line[:endIdx+1]...)

		line = String{}
		lineIdx = 0
		lineCap = 0
	}

	var newline = func() {
		flush()
		result = append(result, ColoredRune{Symbol: '\n'})
	}

	for {
		r, _, err := input.ReadRune()
		if err != nil && err == io.EOF {
			break
		}

		switch r {
		case '\x1b':
			readANSISeq()

		case '\r':
			lineIdx = 0

		case '\n':
			newline()

		case '\b':
			del()

		default:
			add(r)
		}
	}

	flush()

	// Process optional parser options
	for _, opt := range opts {
		if err := opt(&result); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

// ParseString parses a string that can contain both ANSI escape code Select
// Graphic Rendition (SGR) codes and Markdown style text annotations, for
// example *bold* or _italic_.
// SGR details : https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
func ParseString(input string, opts ...ParseOption) (*String, error) {
	var (
		pointer int
		current uint64
		err     error
	)

	// Special case: the escape sequence without any parameter is equivalent to
	// the reset escape sequence.
	input = strings.Replace(input, "\x1b[m", "\x1b[0m", -1)

	// Ignore 'Set cursor key to application' sequence
	input = strings.Replace(input, "\x1b[?1h", "", -1)

	// Ignore keypad mode settings
	input = strings.Replace(input, "\x1b=", "", -1)
	input = strings.Replace(input, "\x1b>", "", -1)

	// Ignore clear line from cursor right
	input = strings.Replace(input, "\x1b[K", "", -1)

	// Ignore known mode settings
	input = regexp.MustCompile(`\x1b\[\?.+[lh]`).ReplaceAllString(input, "")

	// Ignore this unknown sequence, which seems to be an conditional check
	input = regexp.MustCompile(`\x1b\]11;\?.`).ReplaceAllString(input, "")

	var result String
	var applyToResult = func(str string, mask uint64) {
		for _, r := range str {
			result = append(result, ColoredRune{r, mask})
		}
	}

	for _, submatch := range escapeSeqRegExp.FindAllStringSubmatchIndex(input, -1) {
		fullMatchStart, fullMatchEnd := submatch[0], submatch[1]
		settingsStart, settingsEnd := submatch[2], submatch[3]

		applyToResult(input[pointer:fullMatchStart], current)

		current, err = parseSelectGraphicRenditionEscapeSequence(input[settingsStart:settingsEnd])
		if err != nil {
			return nil, err
		}

		pointer = fullMatchEnd
	}

	// Flush the remaining input string part into the result
	applyToResult(input[pointer:], current)

	// Process optional parser options
	for _, opt := range opts {
		if err = opt(&result); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func parseSelectGraphicRenditionEscapeSequence(escapeSeq string) (uint64, error) {
	values := []uint8{}
	for _, x := range strings.Split(escapeSeq, ";") {
		// Note: This only works, because of the regular expression only matching
		// digits. Therefore, it should be okay to omit the error.
		value, _ := strconv.Atoi(x)
		values = append(values, uint8(value))
	}

	result := uint64(0)

	for i := 0; i < len(values); i++ {
		switch values[i] {
		case 1: // bold
			result |= boldMask

		case 3: // italic
			result |= italicMask

		case 4: // underline
			result |= underlineMask

		case 30: // Black
			result |= fgRGBMask(1, 1, 1)

		case 31: // Red
			result |= fgRGBMask(222, 56, 43)

		case 32: // Green
			result |= fgRGBMask(57, 181, 74)

		case 33: // Yellow
			result |= fgRGBMask(255, 199, 6)

		case 34: // Blue
			result |= fgRGBMask(0, 111, 184)

		case 35: // Magenta
			result |= fgRGBMask(118, 38, 113)

		case 36: // Cyan
			result |= fgRGBMask(44, 181, 233)

		case 37: // White
			result |= fgRGBMask(204, 204, 204)

		case 90: // Bright Black (Gray)
			result |= fgRGBMask(128, 128, 128)

		case 91: // Bright Red
			result |= fgRGBMask(255, 0, 0)

		case 92: // Bright Green
			result |= fgRGBMask(0, 255, 0)

		case 93: // Bright Yellow
			result |= fgRGBMask(255, 255, 0)

		case 94: // Bright Blue
			result |= fgRGBMask(0, 0, 255)

		case 95: // Bright Magenta
			result |= fgRGBMask(255, 0, 255)

		case 96: // Bright Cyan
			result |= fgRGBMask(0, 255, 255)

		case 97: // Bright White
			result |= fgRGBMask(255, 255, 255)

		case 40: // Black
			result |= bgRGBMask(1, 1, 1)

		case 41: // Red
			result |= bgRGBMask(222, 56, 43)

		case 42: // Green
			result |= bgRGBMask(57, 181, 74)

		case 43: // Yellow
			result |= bgRGBMask(255, 199, 6)

		case 44: // Blue
			result |= bgRGBMask(0, 111, 184)

		case 45: // Magenta
			result |= bgRGBMask(118, 38, 113)

		case 46: // Cyan
			result |= bgRGBMask(44, 181, 233)

		case 47: // White
			result |= bgRGBMask(204, 204, 204)

		case 100: // Bright Black (Gray)
			result |= bgRGBMask(128, 128, 128)

		case 101: // Bright Red
			result |= bgRGBMask(255, 0, 0)

		case 102: // Bright Green
			result |= bgRGBMask(0, 255, 0)

		case 103: // Bright Yellow
			result |= bgRGBMask(255, 255, 0)

		case 104: // Bright Blue
			result |= bgRGBMask(0, 0, 255)

		case 105: // Bright Magenta
			result |= bgRGBMask(255, 0, 255)

		case 106: // Bright Cyan
			result |= bgRGBMask(0, 255, 255)

		case 107: // Bright White
			result |= bgRGBMask(255, 255, 255)

		case 38: // foreground color
			switch {
			case len(values) > 4 && values[i+1] == 2:
				result |= fgRGBMask(uint64(values[i+2]), uint64(values[i+3]), uint64(values[i+4]))
				i += 4

			case len(values) > 2 && values[i+1] == 5:
				r, g, b := lookUp8bitColor(values[i+2])
				result |= fgRGBMask(uint64(r), uint64(g), uint64(b))
				i += 2

			default:
				return 0, fmt.Errorf("unsupported foreground color selection '%v'", values)
			}

		case 48: // background color
			switch {
			case len(values) > 4 && values[i+1] == 2:
				result |= bgRGBMask(uint64(values[i+2]), uint64(values[i+3]), uint64(values[i+4]))
				i += 4

			case len(values) > 2 && values[i+1] == 5:
				r, g, b := lookUp8bitColor(values[i+2])
				result |= bgRGBMask(uint64(r), uint64(g), uint64(b))
				i += 2

			default:
				return 0, fmt.Errorf("unsupported background color selection '%v'", values)

			}
		}
	}

	return result, nil
}

func processTextAnnotations(text *String) error {
	var buffer bytes.Buffer
	for _, coloredRune := range *text {
		_ = buffer.WriteByte(byte(coloredRune.Symbol))
	}

	raw := buffer.String()
	toBeDeleted := []int{}

	// Process text annotation markers for bold, italic and underline
	helperMap := map[uint64]*regexp.Regexp{
		boldMask:      boldMarker,
		italicMask:    italicMarker,
		underlineMask: underlineMarker,
	}

	for mask, regex := range helperMap {
		for _, match := range regex.FindAllStringSubmatchIndex(raw, -1) {
			fullMatchStart, fullMatchEnd := match[0], match[1]
			textStart, textEnd := match[2], match[3]

			for i := textStart; i < textEnd; i++ {
				(*text)[i].Settings |= mask
			}

			toBeDeleted = append(toBeDeleted, fullMatchStart, fullMatchEnd-1)
		}
	}

	// Process text annotation markers that specify a foreground color for a
	// specific part of the text
	for _, match := range colorMarker.FindAllStringSubmatchIndex(raw, -1) {
		fullMatchStart, fullMatchEnd := match[0], match[1]
		colorName := raw[match[2]:match[3]]
		textStart, textEnd := match[4], match[5]

		color := lookupColorByName(colorName)
		if color == nil {
			return fmt.Errorf("unable to find color by name: %s", colorName)
		}

		r, g, b := color.RGB255()
		for i := textStart; i < textEnd; i++ {
			(*text)[i].Settings |= fgMask
			(*text)[i].Settings |= uint64(r) << 8
			(*text)[i].Settings |= uint64(g) << 16
			(*text)[i].Settings |= uint64(b) << 24
		}

		for i := fullMatchStart; i < fullMatchEnd; i++ {
			if i < textStart || i > textEnd-1 {
				toBeDeleted = append(toBeDeleted, i)
			}
		}
	}

	// Finally, sort the runes to be deleted in descending order and delete them
	// one by one to get rid of the text annotation markers
	sort.Slice(toBeDeleted, func(i, j int) bool {
		return toBeDeleted[i] > toBeDeleted[j]
	})

	for _, idx := range toBeDeleted {
		(*text) = append((*text)[:idx], (*text)[idx+1:]...)
	}

	return nil
}

func fgRGBMask(r, g, b uint64) uint64 {
	return fgMask | r<<8 | g<<16 | b<<24
}

func bgRGBMask(r, g, b uint64) uint64 {
	return bgMask | r<<32 | g<<40 | b<<48
}

func lookUp8bitColor(n uint8) (r, g, b uint8) {
	return colorPalette8bit[n].RGB255()
}
