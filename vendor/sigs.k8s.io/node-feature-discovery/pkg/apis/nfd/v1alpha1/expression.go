/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

var matchOps = map[MatchOp]struct{}{
	MatchAny:          {},
	MatchIn:           {},
	MatchNotIn:        {},
	MatchInRegexp:     {},
	MatchExists:       {},
	MatchDoesNotExist: {},
	MatchGt:           {},
	MatchLt:           {},
	MatchGtLt:         {},
	MatchIsTrue:       {},
	MatchIsFalse:      {},
}

type valueRegexpCache []*regexp.Regexp

// CreateMatchExpression creates a new MatchExpression instance. Returns an
// error if validation fails.
func CreateMatchExpression(op MatchOp, values ...string) (*MatchExpression, error) {
	m := newMatchExpression(op, values...)
	return m, m.Validate()
}

// MustCreateMatchExpression creates a new MatchExpression instance. Panics if
// validation fails.
func MustCreateMatchExpression(op MatchOp, values ...string) *MatchExpression {
	m, err := CreateMatchExpression(op, values...)
	if err != nil {
		panic(err)
	}
	return m
}

// newMatchExpression returns a new MatchExpression instance.
func newMatchExpression(op MatchOp, values ...string) *MatchExpression {
	return &MatchExpression{
		Op:    op,
		Value: values,
	}
}

// Validate validates the expression.
func (m *MatchExpression) Validate() error {
	m.valueRe = nil

	if _, ok := matchOps[m.Op]; !ok {
		return fmt.Errorf("invalid Op %q", m.Op)
	}
	switch m.Op {
	case MatchExists, MatchDoesNotExist, MatchIsTrue, MatchIsFalse, MatchAny:
		if len(m.Value) != 0 {
			return fmt.Errorf("value must be empty for Op %q (have %v)", m.Op, m.Value)
		}
	case MatchGt, MatchLt:
		if len(m.Value) != 1 {
			return fmt.Errorf("value must contain exactly one element for Op %q (have %v)", m.Op, m.Value)
		}
		if _, err := strconv.Atoi(m.Value[0]); err != nil {
			return fmt.Errorf("value must be an integer for Op %q (have %v)", m.Op, m.Value[0])
		}
	case MatchGtLt:
		if len(m.Value) != 2 {
			return fmt.Errorf("value must contain exactly two elements for Op %q (have %v)", m.Op, m.Value)
		}
		var err error
		v := make([]int, 2)
		for i := 0; i < 2; i++ {
			if v[i], err = strconv.Atoi(m.Value[i]); err != nil {
				return fmt.Errorf("value must contain integers for Op %q (have %v)", m.Op, m.Value)
			}
		}
		if v[0] >= v[1] {
			return fmt.Errorf("value[0] must be less than Value[1] for Op %q (have %v)", m.Op, m.Value)
		}
	case MatchInRegexp:
		if len(m.Value) == 0 {
			return fmt.Errorf("value must be non-empty for Op %q", m.Op)
		}
		m.valueRe = make([]*regexp.Regexp, len(m.Value))
		for i, v := range m.Value {
			re, err := regexp.Compile(v)
			if err != nil {
				return fmt.Errorf("value must only contain valid regexps for Op %q (have %v)", m.Op, m.Value)
			}
			m.valueRe[i] = re
		}
	default:
		if len(m.Value) == 0 {
			return fmt.Errorf("value must be non-empty for Op %q", m.Op)
		}
	}
	return nil
}

// Match evaluates the MatchExpression against a single input value.
func (m *MatchExpression) Match(valid bool, value interface{}) (bool, error) {
	switch m.Op {
	case MatchAny:
		return true, nil
	case MatchExists:
		return valid, nil
	case MatchDoesNotExist:
		return !valid, nil
	}

	if valid {
		value := fmt.Sprintf("%v", value)
		switch m.Op {
		case MatchIn:
			for _, v := range m.Value {
				if value == v {
					return true, nil
				}
			}
		case MatchNotIn:
			for _, v := range m.Value {
				if value == v {
					return false, nil
				}
			}
			return true, nil
		case MatchInRegexp:
			if m.valueRe == nil {
				return false, fmt.Errorf("BUG: MatchExpression has not been initialized properly, regexps missing")
			}
			for _, re := range m.valueRe {
				if re.MatchString(value) {
					return true, nil
				}
			}
		case MatchGt, MatchLt:
			l, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("not a number %q", value)
			}
			r, err := strconv.Atoi(m.Value[0])
			if err != nil {
				return false, fmt.Errorf("not a number %q in %v", m.Value[0], m)
			}

			if (l < r && m.Op == MatchLt) || (l > r && m.Op == MatchGt) {
				return true, nil
			}
		case MatchGtLt:
			v, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("not a number %q", value)
			}
			lr := make([]int, 2)
			for i := 0; i < 2; i++ {
				lr[i], err = strconv.Atoi(m.Value[i])
				if err != nil {
					return false, fmt.Errorf("not a number %q in %v", m.Value[i], m)
				}
			}
			return v > lr[0] && v < lr[1], nil
		case MatchIsTrue:
			return value == "true", nil
		case MatchIsFalse:
			return value == "false", nil
		default:
			return false, fmt.Errorf("unsupported Op %q", m.Op)
		}
	}
	return false, nil
}

// MatchKeys evaluates the MatchExpression against a set of keys.
func (m *MatchExpression) MatchKeys(name string, keys map[string]Nil) (bool, error) {
	matched := false

	_, ok := keys[name]
	switch m.Op {
	case MatchAny:
		matched = true
	case MatchExists:
		matched = ok
	case MatchDoesNotExist:
		matched = !ok
	default:
		return false, fmt.Errorf("invalid Op %q when matching keys", m.Op)
	}

	if klog.V(3).Enabled() {
		mString := map[bool]string{false: "no match", true: "match found"}[matched]
		k := make([]string, 0, len(keys))
		for n := range keys {
			k = append(k, n)
		}
		sort.Strings(k)
		if len(keys) < 10 || klog.V(4).Enabled() {
			klog.Infof("%s when matching %q %q against %s", mString, name, m.Op, strings.Join(k, " "))
		} else {
			klog.Infof("%s when matching %q %q against %s... (list truncated)", mString, name, m.Op, strings.Join(k[0:10], ", "))
		}
	}
	return matched, nil
}

// MatchValues evaluates the MatchExpression against a set of key-value pairs.
func (m *MatchExpression) MatchValues(name string, values map[string]string) (bool, error) {
	v, ok := values[name]
	matched, err := m.Match(ok, v)
	if err != nil {
		return false, err
	}

	if klog.V(3).Enabled() {
		mString := map[bool]string{false: "no match", true: "match found"}[matched]

		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		kv := make([]string, len(keys))
		for i, k := range keys {
			kv[i] = k + ":" + values[k]
		}

		if len(values) < 10 || klog.V(4).Enabled() {
			klog.Infof("%s when matching %q %q %v against %s", mString, name, m.Op, m.Value, strings.Join(kv, " "))
		} else {
			klog.Infof("%s when matching %q %q %v against %s... (list truncated)", mString, name, m.Op, m.Value, strings.Join(kv[0:10], " "))
		}
	}

	return matched, nil
}

// matchExpression is a helper type for unmarshalling MatchExpression
type matchExpression MatchExpression

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json"
func (m *MatchExpression) UnmarshalJSON(data []byte) error {
	raw := new(interface{})

	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}

	switch v := (*raw).(type) {
	case string:
		*m = *newMatchExpression(MatchIn, v)
	case bool:
		*m = *newMatchExpression(MatchIn, strconv.FormatBool(v))
	case float64:
		*m = *newMatchExpression(MatchIn, strconv.FormatFloat(v, 'f', -1, 64))
	case []interface{}:
		values := make([]string, len(v))
		for i, value := range v {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid value %v in %v", value, v)
			}
			values[i] = str
		}
		*m = *newMatchExpression(MatchIn, values...)
	case map[string]interface{}:
		helper := &matchExpression{}
		if err := json.Unmarshal(data, &helper); err != nil {
			return err
		}
		*m = *newMatchExpression(helper.Op, helper.Value...)
	default:
		return fmt.Errorf("invalid rule '%v' (%T)", v, v)
	}

	return m.Validate()
}

// MatchKeys evaluates the MatchExpressionSet against a set of keys.
func (m *MatchExpressionSet) MatchKeys(keys map[string]Nil) (bool, error) {
	matched, _, err := m.MatchGetKeys(keys)
	return matched, err
}

// MatchedKey holds one matched key.
type MatchedKey struct {
	Name string
}

// MatchGetKeys evaluates the MatchExpressionSet against a set of keys and
// returns all matched keys or nil if no match was found. Special case of an
// empty MatchExpressionSet returns all existing keys are returned. Note that
// an empty MatchExpressionSet and an empty set of keys returns an empty slice
// which is not nil and is treated as a match.
func (m *MatchExpressionSet) MatchGetKeys(keys map[string]Nil) (bool, []MatchedKey, error) {
	ret := make([]MatchedKey, 0, len(*m))

	for n, e := range *m {
		match, err := e.MatchKeys(n, keys)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
		ret = append(ret, MatchedKey{Name: n})
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i].Name < ret[j].Name })
	return true, ret, nil
}

// MatchValues evaluates the MatchExpressionSet against a set of key-value pairs.
func (m *MatchExpressionSet) MatchValues(values map[string]string) (bool, error) {
	matched, _, err := m.MatchGetValues(values)
	return matched, err
}

// MatchedValue holds one matched key-value pair.
type MatchedValue struct {
	Name  string
	Value string
}

// MatchGetValues evaluates the MatchExpressionSet against a set of key-value
// pairs and returns all matched key-value pairs. Special case of an empty
// MatchExpressionSet returns all existing key-value pairs. Note that an empty
// MatchExpressionSet and an empty set of values returns an empty non-nil map
// which is treated as a match.
func (m *MatchExpressionSet) MatchGetValues(values map[string]string) (bool, []MatchedValue, error) {
	ret := make([]MatchedValue, 0, len(*m))

	for n, e := range *m {
		match, err := e.MatchValues(n, values)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
		ret = append(ret, MatchedValue{Name: n, Value: values[n]})
	}
	// Sort for reproducible output
	sort.Slice(ret, func(i, j int) bool { return ret[i].Name < ret[j].Name })
	return true, ret, nil
}

// MatchInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes).
func (m *MatchExpressionSet) MatchInstances(instances []InstanceFeature) (bool, error) {
	v, err := m.MatchGetInstances(instances)
	return len(v) > 0, err
}

// MatchedInstance holds one matched Instance.
type MatchedInstance map[string]string

// MatchGetInstances evaluates the MatchExpressionSet against a set of instance
// features, each of which is an individual set of key-value pairs
// (attributes). A slice containing all matching instances is returned. An
// empty (non-nil) slice is returned if no matching instances were found.
func (m *MatchExpressionSet) MatchGetInstances(instances []InstanceFeature) ([]MatchedInstance, error) {
	ret := []MatchedInstance{}

	for _, i := range instances {
		if match, err := m.MatchValues(i.Attributes); err != nil {
			return nil, err
		} else if match {
			ret = append(ret, i.Attributes)
		}
	}
	return ret, nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchExpressionSet) UnmarshalJSON(data []byte) error {
	*m = MatchExpressionSet{}

	names := make([]string, 0)
	if err := json.Unmarshal(data, &names); err == nil {
		// Simplified slice form
		for _, name := range names {
			split := strings.SplitN(name, "=", 2)
			if len(split) == 1 {
				(*m)[split[0]] = newMatchExpression(MatchExists)
			} else {
				(*m)[split[0]] = newMatchExpression(MatchIn, split[1])
			}
		}
	} else {
		// Unmarshal the full map form
		expressions := make(map[string]*MatchExpression)
		if err := json.Unmarshal(data, &expressions); err != nil {
			return err
		}
		for k, v := range expressions {
			if v != nil {
				(*m)[k] = v
			} else {
				(*m)[k] = newMatchExpression(MatchExists)
			}
		}
	}

	return nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchOp) UnmarshalJSON(data []byte) error {
	var raw string

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if _, ok := matchOps[MatchOp(raw)]; !ok {
		return fmt.Errorf("invalid Op %q", raw)
	}
	*m = MatchOp(raw)
	return nil
}

// UnmarshalJSON implements the Unmarshaler interface of "encoding/json".
func (m *MatchValue) UnmarshalJSON(data []byte) error {
	var raw interface{}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case string:
		*m = []string{v}
	case bool:
		*m = []string{strconv.FormatBool(v)}
	case float64:
		*m = []string{strconv.FormatFloat(v, 'f', -1, 64)}
	case []interface{}:
		values := make([]string, len(v))
		for i, value := range v {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("invalid value %v in %v", value, v)
			}
			values[i] = str
		}
		*m = values
	default:
		return fmt.Errorf("invalid values '%v' (%T)", v, v)
	}

	return nil
}

// DeepCopy supplements the auto-generated code
func (in *valueRegexpCache) DeepCopy() *valueRegexpCache {
	if in == nil {
		return nil
	}
	out := new(valueRegexpCache)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a stub to augment the auto-generated code
//
//nolint:staticcheck  // re.Copy is deprecated but we want to use  it here
func (in *valueRegexpCache) DeepCopyInto(out *valueRegexpCache) {
	*out = make(valueRegexpCache, len(*in))
	for i, re := range *in {
		(*out)[i] = re.Copy()
	}
}
