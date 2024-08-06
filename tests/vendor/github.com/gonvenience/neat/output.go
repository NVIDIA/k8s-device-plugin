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
	"bufio"
	"bytes"
	"fmt"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/gonvenience/bunt"
)

// DefaultColorSchema is a prepared usable color schema for the neat output
// processor which is loosly based upon the colors used by Atom
var DefaultColorSchema = map[string]colorful.Color{
	"documentStart":      bunt.LightSlateGray,
	"keyColor":           bunt.IndianRed,
	"indentLineColor":    {R: 0.14, G: 0.14, B: 0.14},
	"scalarDefaultColor": bunt.PaleGreen,
	"boolColor":          bunt.Moccasin,
	"floatColor":         bunt.Orange,
	"intColor":           bunt.MediumPurple,
	"multiLineTextColor": bunt.Aquamarine,
	"nullColor":          bunt.DarkOrange,
	"binaryColor":        bunt.Aqua,
	"emptyStructures":    bunt.PaleGoldenrod,
	"commentColor":       bunt.DimGray,
	"anchorColor":        bunt.CornflowerBlue,
}

// OutputProcessor provides the functionality to output neat YAML strings using
// colors and text emphasis
type OutputProcessor struct {
	data           *bytes.Buffer
	out            *bufio.Writer
	colorSchema    *map[string]colorful.Color
	useIndentLines bool
	boldKeys       bool
}

// NewOutputProcessor creates a new output processor including the required
// internals using the provided preferences
func NewOutputProcessor(useIndentLines bool, boldKeys bool, colorSchema *map[string]colorful.Color) *OutputProcessor {
	bytesBuffer := &bytes.Buffer{}
	writer := bufio.NewWriter(bytesBuffer)

	// Only use indent lines in color mode
	if !bunt.UseColors() {
		useIndentLines = false
	}

	return &OutputProcessor{
		data:           bytesBuffer,
		out:            writer,
		useIndentLines: useIndentLines,
		boldKeys:       boldKeys,
		colorSchema:    colorSchema,
	}
}

// Deprecated: Use colorizef instead
func (p *OutputProcessor) colorize(text string, colorName string) string {
	if p.colorSchema != nil {
		if value, ok := (*p.colorSchema)[colorName]; ok {
			return bunt.Style(text, bunt.Foreground(value))
		}
	}

	return text
}

func (p *OutputProcessor) colorizef(colorName string, format string, a ...interface{}) string {
	var text = fmt.Sprintf(format, a...)

	if p.colorSchema != nil {
		if value, ok := (*p.colorSchema)[colorName]; ok {
			return bunt.Style(text, bunt.Foreground(value))
		}
	}

	return text
}

func (p *OutputProcessor) determineColorByType(obj interface{}) string {
	color := "scalarDefaultColor"

	switch t := obj.(type) {
	case *yamlv3.Node:
		switch t.Tag {
		case "!!str":
			if len(strings.Split(strings.TrimSpace(t.Value), "\n")) > 1 {
				color = "multiLineTextColor"
			}

		case "!!int":
			color = "intColor"

		case "!!float":
			color = "floatColor"

		case "!!bool":
			color = "boolColor"

		case "!!null":
			color = "nullColor"
		}

	case bool:
		color = "boolColor"

	case float32, float64:
		color = "floatColor"

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
		color = "intColor"

	case string:
		if len(strings.Split(strings.TrimSpace(t), "\n")) > 1 {
			color = "multiLineTextColor"
		}
	}

	return color
}

func (p *OutputProcessor) isScalar(obj interface{}) bool {
	switch tobj := obj.(type) {
	case *yamlv3.Node:
		return tobj.Kind == yamlv3.ScalarNode

	case yamlv2.MapSlice, []interface{}, []yamlv2.MapSlice:
		return false

	default:
		return true
	}
}

func (p *OutputProcessor) simplify(list []yamlv2.MapSlice) []interface{} {
	result := make([]interface{}, len(list))
	for idx, value := range list {
		result[idx] = value
	}

	return result
}

func (p *OutputProcessor) prefixAdd() string {
	if p.useIndentLines {
		return p.colorize("│ ", "indentLineColor")
	}

	return p.colorize("  ", "indentLineColor")
}

func followAlias(node *yamlv3.Node) *yamlv3.Node {
	if node != nil && node.Alias != nil {
		return followAlias(node.Alias)
	}

	return node
}
