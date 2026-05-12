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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/gonvenience/bunt"
)

// ToJSONString marshals the provided object into JSON with text decorations
// and is basically just a convenience function to create the output processor
// and call its `ToJSON` function.
func ToJSONString(obj interface{}) (string, error) {
	return NewOutputProcessor(true, true, &DefaultColorSchema).ToCompactJSON(obj)
}

// ToJSON processes the provided input object and tries to neatly output it as
// human readable JSON honoring the preferences provided to the output processor
func (p *OutputProcessor) ToJSON(obj interface{}) (string, error) {
	return p.neatJSON("", obj)
}

// ToCompactJSON processed the provided input object and tries to create a as
// compact as possible output
func (p *OutputProcessor) ToCompactJSON(obj interface{}) (string, error) {
	switch tobj := obj.(type) {
	case *yamlv3.Node:
		return p.ToCompactJSON(*tobj)

	case yamlv3.Node:
		switch tobj.Kind {
		case yamlv3.DocumentNode:
			return p.ToCompactJSON(tobj.Content[0])

		case yamlv3.MappingNode:
			tmp := []string{}
			for i := 0; i < len(tobj.Content); i += 2 {
				k, v := tobj.Content[i], tobj.Content[i+1]

				key, err := p.ToCompactJSON(k)
				if err != nil {
					return "", err
				}

				value, err := p.ToCompactJSON(v)
				if err != nil {
					return "", err
				}

				tmp = append(tmp, fmt.Sprintf("%s: %s", key, value))
			}

			return fmt.Sprintf("{%s}", strings.Join(tmp, ", ")), nil

		case yamlv3.SequenceNode:
			tmp := []string{}
			for _, e := range tobj.Content {
				entry, err := p.ToCompactJSON(e)
				if err != nil {
					return "", err
				}

				tmp = append(tmp, entry)
			}

			return fmt.Sprintf("[%s]", strings.Join(tmp, ", ")), nil

		case yamlv3.ScalarNode:
			obj, err := cast(tobj)
			if err != nil {
				return "", err
			}

			bytes, err := json.Marshal(obj)
			if err != nil {
				return "", err
			}

			return string(bytes), nil
		}

	case []interface{}:
		result := make([]string, 0)
		for _, i := range tobj {
			value, err := p.ToCompactJSON(i)
			if err != nil {
				return "", err
			}
			result = append(result, value)
		}

		return fmt.Sprintf("[%s]", strings.Join(result, ", ")), nil

	case yamlv2.MapSlice:
		result := make([]string, 0)
		for _, i := range tobj {
			value, err := p.ToCompactJSON(i)
			if err != nil {
				return "", err
			}
			result = append(result, value)
		}

		return fmt.Sprintf("{%s}", strings.Join(result, ", ")), nil

	case yamlv2.MapItem:
		key, keyError := p.ToCompactJSON(tobj.Key)
		if keyError != nil {
			return "", keyError
		}

		value, valueError := p.ToCompactJSON(tobj.Value)
		if valueError != nil {
			return "", valueError
		}

		return fmt.Sprintf("%s: %s", key, value), nil
	}

	bytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (p *OutputProcessor) neatJSON(prefix string, obj interface{}) (string, error) {
	var err error

	switch t := obj.(type) {
	case yamlv3.Node:
		err = p.neatJSONofNode(prefix, &t)

	case *yamlv3.Node:
		err = p.neatJSONofNode(prefix, t)

	case yamlv2.MapSlice:
		err = p.neatJSONofYAMLMapSlice(prefix, t)

	case []interface{}:
		err = p.neatJSONofSlice(prefix, t)

	default:
		err = p.neatJSONofScalar(prefix, obj)
	}

	if err != nil {
		return "", err
	}

	p.out.Flush()
	return p.data.String(), nil
}

func (p *OutputProcessor) neatJSONofNode(prefix string, node *yamlv3.Node) error {
	var (
		optionalLineBreak = func() string {
			switch node.Style {
			case yamlv3.FlowStyle:
				return ""
			}

			return "\n"
		}

		optionalIndentPrefix = func() string {
			switch node.Style {
			case yamlv3.FlowStyle:
				return ""
			}

			return prefix + p.prefixAdd()
		}

		optionalPrefixBeforeEnd = func() string {
			switch node.Style {
			case yamlv3.FlowStyle:
				return ""
			}

			return prefix
		}
	)

	switch node.Kind {
	case yamlv3.DocumentNode:
		return p.neatJSONofNode(prefix, node.Content[0])

	case yamlv3.MappingNode:
		if len(node.Content) == 0 {
			fmt.Fprint(p.out, p.colorize("{}", "emptyStructures"))
			return nil
		}

		bunt.Fprint(p.out, "*{*", optionalLineBreak())
		for i := 0; i < len(node.Content); i += 2 {
			k, v := followAlias(node.Content[i]), followAlias(node.Content[i+1])

			fmt.Fprint(p.out,
				optionalIndentPrefix(),
				p.colorize(`"`+k.Value+`"`, "keyColor"), ": ",
			)

			if p.isScalar(v) {
				if _, err := p.neatJSON("", v); err != nil {
					return err
				}

			} else {
				if _, err := p.neatJSON(prefix+p.prefixAdd(), v); err != nil {
					return err
				}
			}

			if i < len(node.Content)-2 {
				fmt.Fprint(p.out, ",")
			}

			fmt.Fprint(p.out, optionalLineBreak())
		}
		bunt.Fprint(p.out, optionalPrefixBeforeEnd(), "*}*")

	case yamlv3.SequenceNode:
		if len(node.Content) == 0 {
			fmt.Fprint(p.out, p.colorize("[]", "emptyStructures"))
			return nil
		}

		bunt.Fprint(p.out, "*[*", optionalLineBreak())
		for i := range node.Content {
			entry := followAlias(node.Content[i])

			if p.isScalar(entry) {
				if _, err := p.neatJSON(optionalIndentPrefix(), entry); err != nil {
					return err
				}

			} else {
				fmt.Fprint(p.out, prefix, p.prefixAdd())
				if _, err := p.neatJSON(prefix+p.prefixAdd(), entry); err != nil {
					return err
				}
			}

			if i < len(node.Content)-1 {
				fmt.Fprint(p.out, ",")
			}

			fmt.Fprint(p.out, optionalLineBreak())
		}
		bunt.Fprint(p.out, optionalPrefixBeforeEnd(), "*]*")

	case yamlv3.ScalarNode:
		obj, err := cast(*node)
		if err != nil {
			return err
		}

		bytes, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		fmt.Fprint(p.out,
			prefix,
			p.colorize(
				string(bytes),
				p.determineColorByType(node),
			))
	}

	return nil
}

func (p *OutputProcessor) neatJSONofYAMLMapSlice(prefix string, mapslice yamlv2.MapSlice) error {
	if len(mapslice) == 0 {
		_, _ = p.out.WriteString(p.colorize("{}", "emptyStructures"))
		return nil
	}

	_, _ = p.out.WriteString(bunt.Style("{", bunt.Bold()))
	_, _ = p.out.WriteString("\n")

	for idx, mapitem := range mapslice {
		keyString := fmt.Sprintf("\"%v\": ", mapitem.Key)

		_, _ = p.out.WriteString(prefix + p.prefixAdd())
		_, _ = p.out.WriteString(p.colorize(keyString, "keyColor"))

		if p.isScalar(mapitem.Value) {
			if err := p.neatJSONofScalar("", mapitem.Value); err != nil {
				return err
			}

		} else {
			if _, err := p.neatJSON(prefix+p.prefixAdd(), mapitem.Value); err != nil {
				return err
			}
		}

		if idx < len(mapslice)-1 {
			_, _ = p.out.WriteString(",")
		}

		_, _ = p.out.WriteString("\n")
	}

	_, _ = p.out.WriteString(prefix)
	_, _ = p.out.WriteString(bunt.Style("}", bunt.Bold()))

	return nil
}

func (p *OutputProcessor) neatJSONofSlice(prefix string, list []interface{}) error {
	if len(list) == 0 {
		_, _ = p.out.WriteString(p.colorize("[]", "emptyStructures"))
		return nil
	}

	_, _ = p.out.WriteString(bunt.Style("[", bunt.Bold()))
	_, _ = p.out.WriteString("\n")

	for idx, value := range list {
		if p.isScalar(value) {
			if err := p.neatJSONofScalar(prefix+p.prefixAdd(), value); err != nil {
				return err
			}

		} else {
			_, _ = p.out.WriteString(prefix + p.prefixAdd())
			if _, err := p.neatJSON(prefix+p.prefixAdd(), value); err != nil {
				return err
			}
		}

		if idx < len(list)-1 {
			_, _ = p.out.WriteString(",")
		}

		_, _ = p.out.WriteString("\n")
	}

	_, _ = p.out.WriteString(prefix)
	_, _ = p.out.WriteString(bunt.Style("]", bunt.Bold()))

	return nil
}

func (p *OutputProcessor) neatJSONofScalar(prefix string, obj interface{}) error {
	if obj == nil {
		_, _ = p.out.WriteString(p.colorize("null", "nullColor"))
		return nil
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	color := p.determineColorByType(obj)

	_, _ = p.out.WriteString(prefix)
	parts := strings.Split(string(data), "\\n")
	for idx, part := range parts {
		_, _ = p.out.WriteString(p.colorize(part, color))

		if idx < len(parts)-1 {
			_, _ = p.out.WriteString(p.colorize("\\n", "emptyStructures"))
		}
	}

	return nil
}

func cast(node yamlv3.Node) (interface{}, error) {
	if node.Kind != yamlv3.ScalarNode {
		return nil, fmt.Errorf("invalid node kind to cast, must be a scalar node")
	}

	switch node.Tag {
	case "!!str":
		return node.Value, nil

	case "!!timestamp":
		return parseTime(node.Value)

	case "!!int":
		return strconv.Atoi(node.Value)

	case "!!float":
		return strconv.ParseFloat(node.Value, 64)

	case "!!bool":
		return strconv.ParseBool(node.Value)

	case "!!null":
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown tag %s", node.Tag)
	}
}

func parseTime(value string) (time.Time, error) {
	// YAML Spec regarding timestamp: https://yaml.org/type/timestamp.html
	var layouts = [...]string{
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02t15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if result, err := time.Parse(layout, value); err == nil {
			return result, nil
		}
	}

	return time.Time{}, fmt.Errorf("value %q cannot be parsed as a timestamp", value)
}
