// Copyright © 2018 The Homeport Team
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

package ytbx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gonvenience/bunt"
	"github.com/gonvenience/text"
	"github.com/gonvenience/wrap"
	ordered "github.com/virtuald/go-ordered-json"
	yamlv3 "gopkg.in/yaml.v3"
)

// PreserveKeyOrderInJSON specifies whether a special library is used to decode
// JSON input to preserve the order of keys in maps even though that is not part
// of the JSON specification.
var PreserveKeyOrderInJSON = false

// DecoderProxy can either be used with the standard JSON Decoder, or the
// specialised JSON library fork that supports preserving key order
type DecoderProxy struct {
	standard *json.Decoder
	ordered  *ordered.Decoder
}

// InputFile represents the actual input file (local, or fetched remotely) that
// needs to be processed. It can contain multiple documents, where a document
// is a map or a list of things.
type InputFile struct {
	Location  string
	Note      string
	Documents []*yamlv3.Node
	Names     []string
}

// NewDecoderProxy creates a new decoder proxy which either works in ordered
// mode or standard mode.
func NewDecoderProxy(keepOrder bool, r io.Reader) *DecoderProxy {
	if keepOrder {
		decoder := ordered.NewDecoder(r)
		decoder.UseOrderedObject()
		return &DecoderProxy{ordered: decoder}
	}

	return &DecoderProxy{standard: json.NewDecoder(r)}
}

// Decode is a delegate function that calls JSON Decoder `Decode`
func (d *DecoderProxy) Decode(v interface{}) error {
	if d.ordered != nil {
		return d.ordered.Decode(v)
	}

	return d.standard.Decode(v)
}

// HumanReadableLocationInformation create a nicely decorated information about
// the provided input location. It will output the absolute path of the file
// rather than the possibly relative location, or it will show the URL in the
// usual look-and-feel of URIs.
func HumanReadableLocationInformation(inputFile InputFile) string {
	var buf bytes.Buffer

	// Start with a nice location output
	buf.WriteString(HumanReadableLocation(inputFile.Location))

	// Add additional note if it is set
	if inputFile.Note != "" {
		bunt.Fprintf(&buf, ", Orange{%s}", inputFile.Note)
	}

	// Add an information about how many documents are in the provided input file
	if documents := len(inputFile.Documents); documents > 1 {
		bunt.Fprintf(&buf, ", Aquamarine{*%s*}", text.Plural(documents, "document"))
	}

	return buf.String()
}

// HumanReadableLocation returns a human readable location with proper coloring
func HumanReadableLocation(location string) string {
	if IsStdin(location) {
		return bunt.Sprint("_*stdin*_")
	}

	if _, err := os.Stat(location); err == nil {
		return bunt.Sprintf("*%s*", location)
	}

	if _, err := url.ParseRequestURI(location); err == nil {
		return bunt.Sprintf("CornflowerBlue{~%s~}", location)
	}

	return location
}

// LoadFiles concurrently loads two files from the provided locations
func LoadFiles(locationA string, locationB string) (InputFile, InputFile, error) {
	type resultPair struct {
		result InputFile
		err    error
	}

	fromChan := make(chan resultPair, 1)
	toChan := make(chan resultPair, 1)

	go func() {
		result, err := LoadFile(locationA)
		fromChan <- resultPair{result, err}
	}()

	go func() {
		result, err := LoadFile(locationB)
		toChan <- resultPair{result, err}
	}()

	from := <-fromChan
	if from.err != nil {
		return InputFile{}, InputFile{}, from.err
	}

	to := <-toChan
	if to.err != nil {
		return InputFile{}, InputFile{}, to.err
	}

	return from.result, to.result, nil
}

// LoadFile processes the provided input location to load it as one of the
// supported document formats, or plain text if nothing else works.
func LoadFile(location string) (InputFile, error) {
	if info, err := os.Stat(location); err == nil && info.IsDir() {
		return LoadDirectory(location)
	}

	var (
		documents []*yamlv3.Node
		data      []byte
		err       error
	)

	if data, err = getBytesFromLocation(location); err != nil {
		return InputFile{}, wrap.Errorf(err, "unable to load data from %s", HumanReadableLocation(location))
	}

	if documents, err = LoadDocuments(data); err != nil {
		return InputFile{}, wrap.Errorf(err, "unable to parse data from %s", HumanReadableLocation(location))
	}

	return InputFile{
		Location:  location,
		Documents: documents,
	}, nil
}

// LoadDirectory reads the provided location as a directory and processes all
// files in the directory as documents
func LoadDirectory(location string) (InputFile, error) {
	files, err := ioutil.ReadDir(location)
	if err != nil {
		return InputFile{}, wrap.Errorf(err, "failed to read files in directory %s", location)
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Name(), files[j].Name()) < 0
	})

	var result = InputFile{
		Location: location,
	}

	for _, file := range files {
		bytes, err := getBytesFromLocation(filepath.Join(location, file.Name()))
		if err != nil {
			return InputFile{}, err
		}

		docs, err := LoadDocuments(bytes)
		if err != nil {
			return InputFile{}, err
		}

		result.Documents = append(result.Documents, docs...)
		result.Names = append(result.Names, file.Name())
	}

	return result, nil
}

// LoadDocuments reads the provided input data slice as a YAML, JSON, or TOML
// file with potential multiple documents. It only acts as a dispatcher and
// depending on the input will either use `LoadTOMLDocuments`,
// `LoadJSONDocuments`, or `LoadYAMLDocuments`.
func LoadDocuments(input []byte) ([]*yamlv3.Node, error) {
	// There is no easy check whether the input data is TOML format, this is
	// why there is currently no other option than simply trying to parse it.
	if toml, err := LoadTOMLDocuments(input); err == nil {
		return toml, err
	}

	// In case the input data set starts with either a map or list start
	// symbol, it is assumed to be a JSON document. In any other case, use
	// the YAML parser function which also covers plain text documents.
	switch input[0] {
	case '{', '[':
		return LoadJSONDocuments(input)

	default:
		return LoadYAMLDocuments(input)
	}
}

// LoadJSONDocuments reads the provided input data slice as a JSON file with
// potential multiple documents. Each document in the JSON stream results in an
// entry of the result slice.
func LoadJSONDocuments(input []byte) ([]*yamlv3.Node, error) {
	values := []*yamlv3.Node{}

	decoder := NewDecoderProxy(PreserveKeyOrderInJSON, bytes.NewReader(input))
	for {
		var value interface{}
		err := decoder.Decode(&value)
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		node, err := asYAMLNode(value)
		if err != nil {
			return nil, err
		}

		values = append(values, node)
	}

	return values, nil
}

// LoadYAMLDocuments reads the provided input data slice as a YAML file with
// potential multiple documents. Each document in the YAML stream results in an
// entry of the result slice.
func LoadYAMLDocuments(input []byte) ([]*yamlv3.Node, error) {
	documents := []*yamlv3.Node{}

	decoder := yamlv3.NewDecoder(bytes.NewReader(input))
	for {
		var node yamlv3.Node

		err := decoder.Decode(&node)
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		documents = append(documents, &node)
	}

	return documents, nil
}

// LoadTOMLDocuments reads the provided input data slice as a TOML file, which
// can only have one document. For the sake of having similar sounding
// functions and the same signatures, the function uses the plural in its name
// and returns a list of results even though it will only contain one entry.
// All map entries inside the result document are converted into Go-YAMLv3 Node
// types to make it compatible with the rest of the package.
func LoadTOMLDocuments(input []byte) ([]*yamlv3.Node, error) {
	var data interface{}
	if err := toml.Unmarshal(input, &data); err != nil {
		return nil, err
	}

	node, err := asYAMLNode(data)
	if err != nil {
		return nil, err
	}

	return []*yamlv3.Node{node}, nil
}

func getBytesFromLocation(location string) ([]byte, error) {
	// Handle special location "-" which refers to STDIN stream
	if IsStdin(location) {
		return ioutil.ReadAll(os.Stdin)
	}

	// Handle location as local file if there is a file at that location
	if _, err := os.Stat(location); err == nil {
		return ioutil.ReadFile(location)
	}

	// Handle location as a URI if it looks like one
	if _, err := url.ParseRequestURI(location); err == nil {
		response, err := http.Get(location)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		data, err := ioutil.ReadAll(response.Body)
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to retrieve data from location %s: %s", location, string(data))
		}

		return data, err
	}

	// In any other case, bail out ...
	return nil, fmt.Errorf("unable to get any content using location %s: it is not a file or usable URI", location)
}

// IsStdin checks whether the provided input location refers to the dash
// character which usually serves as the replacement to point to STDIN rather
// than a file.
func IsStdin(location string) bool {
	return strings.TrimSpace(location) == "-"
}
