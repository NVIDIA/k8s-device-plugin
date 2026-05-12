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
	"reflect"
	"strings"

	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/gonvenience/bunt"
)

// ToYAMLString marshals the provided object into YAML with text decorations
// and is basically just a convenience function to create the output processor
// and call its `ToYAML` function.
func ToYAMLString(obj interface{}) (string, error) {
	return NewOutputProcessor(true, true, &DefaultColorSchema).ToYAML(obj)
}

// ToYAML processes the provided input object and tries to neatly output it as
// human readable YAML honoring the preferences provided to the output processor
func (p *OutputProcessor) ToYAML(obj interface{}) (string, error) {
	if err := p.neatYAML("", false, obj); err != nil {
		return "", err
	}

	p.out.Flush()
	return p.data.String(), nil
}

func (p *OutputProcessor) neatYAML(prefix string, skipIndentOnFirstLine bool, obj interface{}) error {
	switch t := obj.(type) {
	case yamlv2.MapSlice:
		return p.neatYAMLofMapSlice(prefix, skipIndentOnFirstLine, t)

	case []interface{}:
		return p.neatYAMLofSlice(prefix, skipIndentOnFirstLine, t)

	case []yamlv2.MapSlice:
		return p.neatYAMLofSlice(prefix, skipIndentOnFirstLine, p.simplify(t))

	case yamlv3.Node:
		return p.neatYAMLofNode(prefix, skipIndentOnFirstLine, &t)

	default:
		switch reflect.TypeOf(obj).Kind() {
		case reflect.Ptr:
			return p.neatYAML(prefix, skipIndentOnFirstLine, reflect.ValueOf(obj).Elem().Interface())

		case reflect.Struct:
			return p.neatYAMLOfStruct(prefix, skipIndentOnFirstLine, t)

		default:
			return p.neatYAMLofScalar(prefix, skipIndentOnFirstLine, t)
		}
	}
}

func (p *OutputProcessor) neatYAMLofMapSlice(prefix string, skipIndentOnFirstLine bool, mapslice yamlv2.MapSlice) error {
	for i, mapitem := range mapslice {
		if !skipIndentOnFirstLine || i > 0 {
			_, _ = p.out.WriteString(prefix)
		}

		keyString := fmt.Sprintf("%v:", mapitem.Key)
		if p.boldKeys {
			keyString = bunt.Style(keyString, bunt.Bold())
		}

		_, _ = p.out.WriteString(p.colorize(keyString, "keyColor"))

		switch mapitem.Value.(type) {
		case yamlv2.MapSlice:
			if len(mapitem.Value.(yamlv2.MapSlice)) == 0 {
				_, _ = p.out.WriteString(" ")
				_, _ = p.out.WriteString(p.colorize("{}", "emptyStructures"))
				_, _ = p.out.WriteString("\n")

			} else {
				_, _ = p.out.WriteString("\n")
				if err := p.neatYAMLofMapSlice(prefix+p.prefixAdd(), false, mapitem.Value.(yamlv2.MapSlice)); err != nil {
					return err
				}
			}

		case []interface{}:
			if len(mapitem.Value.([]interface{})) == 0 {
				_, _ = p.out.WriteString(" ")
				_, _ = p.out.WriteString(p.colorize("[]", "emptyStructures"))
				_, _ = p.out.WriteString("\n")
			} else {
				_, _ = p.out.WriteString("\n")
				if err := p.neatYAMLofSlice(prefix, false, mapitem.Value.([]interface{})); err != nil {
					return err
				}
			}

		default:
			_, _ = p.out.WriteString(" ")
			if err := p.neatYAMLofScalar(prefix, false, mapitem.Value); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *OutputProcessor) neatYAMLofSlice(prefix string, skipIndentOnFirstLine bool, list []interface{}) error {
	for _, entry := range list {
		_, _ = p.out.WriteString(prefix)
		_, _ = p.out.WriteString(p.colorize("-", "dashColor"))
		_, _ = p.out.WriteString(" ")
		if err := p.neatYAML(prefix+p.prefixAdd(), true, entry); err != nil {
			return err
		}
	}

	return nil
}

func (p *OutputProcessor) neatYAMLofScalar(prefix string, skipIndentOnFirstLine bool, obj interface{}) error {
	// Process nil values immediately and return afterwards
	if obj == nil {
		_, _ = p.out.WriteString(p.colorize("null", "nullColor"))
		_, _ = p.out.WriteString("\n")
		return nil
	}

	// Any other value: Run through Go YAML marshaller and colorize afterwards
	data, err := yamlv2.Marshal(obj)
	if err != nil {
		return err
	}

	// Decide on one color to be used
	color := p.determineColorByType(obj)

	// Cast byte slice to string, remove trailing newlines, split into lines
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if i > 0 {
			_, _ = p.out.WriteString(prefix)
		}

		_, _ = p.out.WriteString(p.colorize(line, color))
		_, _ = p.out.WriteString("\n")
	}

	return nil
}

func (p *OutputProcessor) neatYAMLofNode(prefix string, skipIndentOnFirstLine bool, node *yamlv3.Node) error {
	keyStyles := []bunt.StyleOption{}
	if p.boldKeys {
		keyStyles = append(keyStyles, bunt.Bold())
	}

	switch node.Kind {
	case yamlv3.DocumentNode:
		bunt.Fprint(p.out, p.colorize("---", "documentStart"), "\n")
		for _, content := range node.Content {
			if err := p.neatYAML(prefix, false, content); err != nil {
				return err
			}
		}

		if len(node.FootComment) > 0 {
			fmt.Fprint(p.out, p.colorize(node.FootComment, "commentColor"), "\n")
		}

	case yamlv3.SequenceNode:
		for i, entry := range node.Content {
			if i == 0 {
				if !skipIndentOnFirstLine {
					fmt.Fprint(p.out, prefix)
				}
			} else {
				fmt.Fprint(p.out, prefix)
			}

			fmt.Fprint(p.out, p.colorize("-", "dashColor"), " ")

			if err := p.neatYAMLofNode(prefix+p.prefixAdd(), true, entry); err != nil {
				return err
			}
		}

	case yamlv3.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if !skipIndentOnFirstLine || i > 0 {
				fmt.Fprint(p.out, prefix)
			}

			key := node.Content[i]
			if len(key.HeadComment) > 0 {
				fmt.Fprint(p.out, p.colorize(key.HeadComment, "commentColor"), "\n")
			}
			fmt.Fprint(p.out,
				bunt.Style(p.colorize(fmt.Sprintf("%s:", key.Value), "keyColor"), keyStyles...),
			)

			value := node.Content[i+1]
			switch value.Kind {
			case yamlv3.MappingNode:
				if len(value.Content) == 0 {
					fmt.Fprint(p.out, p.createAnchorDefinition(value), " ", p.colorize("{}", "emptyStructures"), "\n")
				} else {
					fmt.Fprint(p.out, p.createAnchorDefinition(value), "\n")
					if err := p.neatYAMLofNode(prefix+p.prefixAdd(), false, value); err != nil {
						return err
					}
				}

			case yamlv3.SequenceNode:
				if len(value.Content) == 0 {
					fmt.Fprint(p.out, p.createAnchorDefinition(value), " ", p.colorize("[]", "emptyStructures"), "\n")
				} else {
					fmt.Fprint(p.out, p.createAnchorDefinition(value), "\n")
					if err := p.neatYAMLofNode(prefix, false, value); err != nil {
						return err
					}
				}

			case yamlv3.ScalarNode:
				fmt.Fprint(p.out, p.createAnchorDefinition(value), " ")
				if err := p.neatYAMLofNode(prefix+p.prefixAdd(), false, value); err != nil {
					return err
				}

			case yamlv3.AliasNode:
				fmt.Fprintf(p.out, " %s\n", p.colorize("*"+value.Value, "anchorColor"))
			}

			if len(key.FootComment) > 0 {
				fmt.Fprint(p.out, p.colorize(key.FootComment, "commentColor"), "\n")
			}
		}

	case yamlv3.ScalarNode:
		var colorName = "scalarDefaultColor"
		switch node.Tag {
		case "!!binary":
			colorName = "binaryColor"

		case "!!str":
			colorName = "scalarDefaultColor"

		case "!!float":
			colorName = "floatColor"

		case "!!int":
			colorName = "intColor"

		case "!!bool":
			colorName = "boolColor"

		case "!!null":
			colorName = "nullColor"
		}

		lines := strings.Split(node.Value, "\n")
		switch len(lines) {
		case 1:
			if needsQuotes(node) {
				fmt.Fprint(p.out, p.colorizef(colorName, `"%s"`, node.Value))
			} else {
				fmt.Fprint(p.out, p.colorizef(colorName, node.Value))
			}

		default:
			colorName = "multiLineTextColor"
			fmt.Fprint(p.out, p.colorize("|", colorName), "\n")
			for i, line := range lines {
				fmt.Fprint(p.out,
					prefix,
					p.colorize(line, colorName),
				)

				if i != len(lines)-1 {
					fmt.Fprint(p.out, "\n")
				}
			}
		}

		if len(node.LineComment) > 0 {
			fmt.Fprint(p.out, " ", p.colorize(node.LineComment, "commentColor"))
		}

		fmt.Fprint(p.out, "\n")

		if len(node.FootComment) > 0 {
			fmt.Fprint(p.out, p.colorize(node.FootComment, "commentColor"), "\n")
		}

	case yamlv3.AliasNode:
		if err := p.neatYAMLofNode(prefix, skipIndentOnFirstLine, node.Alias); err != nil {
			return err
		}
	}

	return nil
}

func (p *OutputProcessor) neatYAMLOfStruct(prefix string, skipIndentOnFirstLine bool, obj interface{}) error {
	// There might be better ways to do it. With generic struct objects, the
	// only option is to do a roundtrip marshal and unmarshal to get the
	// object into a universal Go YAML library version 3 node object and
	// to render the node instead.

	data, err := yamlv3.Marshal(obj)
	if err != nil {
		return err
	}

	var tmp yamlv3.Node
	if err := yamlv3.Unmarshal(data, &tmp); err != nil {
		return err
	}

	return p.neatYAML(prefix, skipIndentOnFirstLine, tmp)
}

func (p *OutputProcessor) createAnchorDefinition(node *yamlv3.Node) string {
	if len(node.Anchor) != 0 {
		return fmt.Sprint(" ", p.colorize("&"+node.Anchor, "anchorColor"))
	}

	return ""
}

func needsQuotes(node *yamlv3.Node) bool {
	// skip all non string nodes
	if node.Tag != "!!str" {
		return false
	}

	// check if string matches one of the known reserved keywords
	for _, chk := range []string{"true", "false", "null"} {
		if node.Value == chk {
			return true
		}
	}

	// check if string contains special characters
	return strings.ContainsAny(node.Value, " *&:,")
}
