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

package utils

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RegexpVal is a wrapper for regexp command line flags
type RegexpVal struct {
	regexp.Regexp
}

// Set implements the flag.Value interface
func (a *RegexpVal) Set(val string) error {
	r, err := regexp.Compile(val)
	if err == nil {
		a.Regexp = *r
	}
	return err
}

// UnmarshalJSON implements the Unmarshaler interface from "encoding/json"
func (a *RegexpVal) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case string:
		if r, err := regexp.Compile(string(val)); err != nil {
			return err
		} else {
			*a = RegexpVal{*r}
		}
	default:
		return fmt.Errorf("invalid regexp %s", data)
	}
	return nil
}

// StringSetVal is a Value encapsulating a set of comma-separated strings
type StringSetVal map[string]struct{}

// Set implements the flag.Value interface
func (a *StringSetVal) Set(val string) error {
	m := map[string]struct{}{}
	for _, n := range strings.Split(val, ",") {
		m[n] = struct{}{}
	}
	*a = m
	return nil
}

// String implements the flag.Value interface
func (a *StringSetVal) String() string {
	if *a == nil {
		return ""
	}
	vals := make([]string, 0, len(*a))
	for val := range *a {
		vals = append(vals, val)
	}
	sort.Strings(vals)
	return strings.Join(vals, ",")
}

// UnmarshalJSON implements the Unmarshaler interface from "encoding/json"
func (a *StringSetVal) UnmarshalJSON(data []byte) error {
	var tmp []string
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	for _, v := range tmp {
		(*a)[v] = struct{}{}
	}
	return nil
}

// StringSliceVal is a Value encapsulating a slice of comma-separated strings
type StringSliceVal []string

// Set implements the regexp.Value interface
func (a *StringSliceVal) Set(val string) error {
	*a = strings.Split(val, ",")
	return nil
}

// String implements the regexp.Value interface
func (a *StringSliceVal) String() string {
	if *a == nil {
		return ""
	}
	return strings.Join(*a, ",")
}

// KlogFlagVal is a wrapper to allow dynamic control of klog from the config file
type KlogFlagVal struct {
	flag             *flag.Flag
	isSetFromCmdLine bool
}

// Set implements flag.Value interface
func (k *KlogFlagVal) Set(value string) error {
	k.isSetFromCmdLine = true
	return k.flag.Value.Set(value)
}

// String implements flag.Value interface
func (k *KlogFlagVal) String() string {
	if k.flag == nil {
		return ""
	}
	// Need to handle "log_backtrace_at" in a special way
	s := k.flag.Value.String()
	if k.flag.Name == "log_backtrace_at" && s == ":0" {
		s = ""
	}
	return s
}

// DefValue returns the default value of KlogFlagVal as string
func (k *KlogFlagVal) DefValue() string {
	// Need to handle "log_backtrace_at" in a special way
	d := k.flag.DefValue
	if k.flag.Name == "log_backtrace_at" && d == ":0" {
		d = ""
	}
	return d
}

// SetFromConfig sets the value without marking it as set from the cmdline
func (k *KlogFlagVal) SetFromConfig(value string) error {
	return k.flag.Value.Set(value)
}

// IsSetFromCmdline returns true if the value has been set via Set()
func (k *KlogFlagVal) IsSetFromCmdline() bool { return k.isSetFromCmdLine }

// IsBoolFlag implements flag.boolFlag.IsBoolFlag() for wrapped klog flags.
func (k *KlogFlagVal) IsBoolFlag() bool {
	if ba, ok := k.flag.Value.(boolFlag); ok {
		return ba.IsBoolFlag()
	}
	return false
}

// NewKlogFlagVal wraps a klog flag into KlogFlagVal type
func NewKlogFlagVal(f *flag.Flag) *KlogFlagVal {
	return &KlogFlagVal{flag: f}
}

// boolFlag replicates boolFlag interface internal to the flag package
type boolFlag interface {
	IsBoolFlag() bool
}
