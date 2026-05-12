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

package dyff

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gonvenience/bunt"
	"github.com/gonvenience/neat"
	"github.com/gonvenience/term"
	"github.com/gonvenience/text"
	"github.com/gonvenience/ytbx"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/texttheater/golang-levenshtein/levenshtein"
	yamlv3 "gopkg.in/yaml.v3"
)

// stringWriter is the interface that wraps the WriteString method.
type stringWriter interface {
	WriteString(s string) (int, error)
}

// HumanReport is a reporter with human readable output in mind
type HumanReport struct {
	Report
	MinorChangeThreshold float64
	NoTableStyle         bool
	DoNotInspectCerts    bool
	OmitHeader           bool
	UseGoPatchPaths      bool
}

// WriteReport writes a human readable report to the provided writer
func (report *HumanReport) WriteReport(out io.Writer) error {
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	// Only show the document index if there is more than one document to show
	showPathRoot := len(report.From.Documents) > 1

	// Show banner if enabled
	if !report.OmitHeader {
		var header = fmt.Sprintf(`     _        __  __
   _| |_   _ / _|/ _|  between %s
 / _' | | | | |_| |_       and %s
| (_| | |_| |  _|  _|
 \__,_|\__, |_| |_|   returned %s
        |___/
`,
			ytbx.HumanReadableLocationInformation(report.From),
			ytbx.HumanReadableLocationInformation(report.To),
			bunt.Style(text.Plural(len(report.Diffs), "difference"), bunt.Bold()))

		_, _ = writer.WriteString(bunt.Style(
			header,
			bunt.ForegroundFunc(func(x int, _ int, _ rune) *colorful.Color {
				switch {
				case x < 7:
					return &colorful.Color{R: .45, G: .71, B: .30}

				case x < 13:
					return &colorful.Color{R: .79, G: .76, B: .38}

				case x < 21:
					return &colorful.Color{R: .65, G: .17, B: .17}
				}

				return nil
			}),
		))
	}

	// Loop over the diff and generate each report into the buffer
	for _, diff := range report.Diffs {
		if err := report.generateHumanDiffOutput(writer, diff, report.UseGoPatchPaths, showPathRoot); err != nil {
			return err
		}
	}

	// Finish with one last newline so that we do not end next to the prompt
	_, _ = writer.WriteString("\n")
	return nil
}

// generateHumanDiffOutput creates a human readable report of the provided diff and writes this into the given bytes buffer. There is an optional flag to indicate whether the document index (which documents of the input file) should be included in the report of the path of the difference.
func (report *HumanReport) generateHumanDiffOutput(output stringWriter, diff Diff, useGoPatchPaths bool, showPathRoot bool) error {
	_, _ = output.WriteString("\n")
	_, _ = output.WriteString(pathToString(diff.Path, useGoPatchPaths, showPathRoot))
	_, _ = output.WriteString("\n")

	blocks := make([]string, len(diff.Details))
	for i, detail := range diff.Details {
		generatedOutput, err := report.generateHumanDetailOutput(detail)
		if err != nil {
			return err
		}

		blocks[i] = generatedOutput
	}

	// For the use case in which only a path-less diff is suppose to be printed,
	// omit the indent in this case since there is only one element to show
	indent := 2
	if diff.Path != nil && len(diff.Path.PathElements) == 0 {
		indent = 0
	}

	report.writeTextBlocks(output, indent, blocks...)
	return nil
}

// generateHumanDetailOutput only serves as a dispatcher to call the correct sub function for the respective type of change
func (report *HumanReport) generateHumanDetailOutput(detail Detail) (string, error) {
	switch detail.Kind {
	case ADDITION:
		return report.generateHumanDetailOutputAddition(detail)

	case REMOVAL:
		return report.generateHumanDetailOutputRemoval(detail)

	case MODIFICATION:
		return report.generateHumanDetailOutputModification(detail)

	case ORDERCHANGE:
		return report.generateHumanDetailOutputOrderchange(detail)
	}

	return "", fmt.Errorf("unsupported detail type %c", detail.Kind)
}

func (report *HumanReport) generateHumanDetailOutputAddition(detail Detail) (string, error) {
	var output bytes.Buffer

	switch detail.To.Kind {
	case yamlv3.SequenceNode:
		_, _ = output.WriteString(yellow("%c %s added:\n",
			ADDITION,
			text.Plural(len(detail.To.Content), "list entry", "list entries"),
		))

	case yamlv3.MappingNode:
		_, _ = output.WriteString(yellow("%c %s added:\n",
			ADDITION,
			text.Plural(len(detail.To.Content)/2, "map entry", "map entries"),
		))
	}

	ytbx.RestructureObject(detail.To)
	yamlOutput, err := yamlStringInGreenishColors(detail.To)
	if err != nil {
		return "", err
	}

	report.writeTextBlocks(&output, 2, yamlOutput)

	return output.String(), nil
}

func (report *HumanReport) generateHumanDetailOutputRemoval(detail Detail) (string, error) {
	var output bytes.Buffer

	switch detail.From.Kind {
	case yamlv3.DocumentNode:
		_, _ = fmt.Fprint(&output, yellow("%c %s removed:\n",
			REMOVAL,
			text.Plural(len(detail.From.Content), "document"),
		))

	case yamlv3.SequenceNode:
		text := text.Plural(len(detail.From.Content), "list entry", "list entries")
		_, _ = output.WriteString(yellow("%c %s removed:\n", REMOVAL, text))

	case yamlv3.MappingNode:
		text := text.Plural(len(detail.From.Content)/2, "map entry", "map entries")
		_, _ = output.WriteString(yellow("%c %s removed:\n", REMOVAL, text))
	}

	ytbx.RestructureObject(detail.From)
	yamlOutput, err := yamlStringInRedishColors(detail.From)
	if err != nil {
		return "", err
	}

	report.writeTextBlocks(&output, 2, yamlOutput)

	return output.String(), nil
}

func (report *HumanReport) generateHumanDetailOutputModification(detail Detail) (string, error) {
	var output bytes.Buffer
	fromType := humanReadableType(detail.From)
	toType := humanReadableType(detail.To)

	switch {
	case fromType == "string" && toType == "string":
		// delegate to special string output
		report.writeStringDiff(
			&output,
			detail.From.Value,
			detail.To.Value,
		)

	case fromType == "binary" && toType == "binary":
		from, err := base64.StdEncoding.DecodeString(detail.From.Value)
		if err != nil {
			return "", err
		}

		to, err := base64.StdEncoding.DecodeString(detail.To.Value)
		if err != nil {
			return "", err
		}

		_, _ = output.WriteString(yellow("%c content change\n", MODIFICATION))
		report.writeTextBlocks(&output, 0,
			red("%s", createStringWithPrefix("  - ", hex.Dump(from))),
			green("%s", createStringWithPrefix("  + ", hex.Dump(to))),
		)

	default:
		if fromType != toType {
			_, _ = output.WriteString(yellow("%c type change from %s to %s\n",
				MODIFICATION,
				italic(fromType),
				italic(toType),
			))

		} else {
			_, _ = output.WriteString(yellow("%c value change\n",
				MODIFICATION,
			))
		}

		from, err := yamlString(detail.From)
		if err != nil {
			return "", err
		}

		to, err := yamlString(detail.To)
		if err != nil {
			return "", err
		}

		_, _ = output.WriteString(red("%s", createStringWithPrefix("  - ", strings.TrimRight(from, "\n"))))
		_, _ = output.WriteString(green("%s", createStringWithPrefix("  + ", strings.TrimRight(to, "\n"))))
	}

	return output.String(), nil
}

func (report *HumanReport) generateHumanDetailOutputOrderchange(detail Detail) (string, error) {
	var output bytes.Buffer

	_, _ = output.WriteString(yellow("%c order changed\n", ORDERCHANGE))
	switch detail.From.Kind {
	case yamlv3.SequenceNode:
		asStringList := func(sequenceNode *yamlv3.Node) ([]string, error) {
			result := make([]string, len(sequenceNode.Content))
			for i, entry := range sequenceNode.Content {
				result[i] = entry.Value
				if entry.Value == "" {
					s, err := yamlString(entry)
					if err != nil {
						return result, err
					}
					result[i] = s
				}
			}

			return result, nil
		}

		from, err := asStringList(detail.From)
		if err != nil {
			return "", err
		}
		to, err := asStringList(detail.To)
		if err != nil {
			return "", err
		}

		const singleLineSeparator = ", "

		threshold := term.GetTerminalWidth() / 2
		fromSingleLineLength := stringArrayLen(from) + ((len(from) - 1) * plainTextLength(singleLineSeparator))
		toStringleLineLength := stringArrayLen(to) + ((len(to) - 1) * plainTextLength(singleLineSeparator))
		if estimatedLength := max(fromSingleLineLength, toStringleLineLength); estimatedLength < threshold {
			_, _ = output.WriteString(red("  - %s\n", strings.Join(from, singleLineSeparator)))
			_, _ = output.WriteString(green("  + %s\n", strings.Join(to, singleLineSeparator)))

		} else {
			_, _ = output.WriteString(CreateTableStyleString(" ", 2,
				red("%s", strings.Join(from, "\n")),
				green("%s", strings.Join(to, "\n"))))
		}
	}

	return output.String(), nil
}

func (report *HumanReport) writeStringDiff(output stringWriter, from string, to string) {
	fromCertText, toCertText, err := report.LoadX509Certs(from, to)

	switch {
	case err == nil:
		_, _ = output.WriteString(yellow("%c certificate change\n", MODIFICATION))
		_, _ = output.WriteString(report.highlightByLine(fromCertText, toCertText))

	case isWhitespaceOnlyChange(from, to):
		_, _ = output.WriteString(yellow("%c whitespace only change\n", MODIFICATION))
		report.writeTextBlocks(output, 0,
			red("%s", createStringWithPrefix("  - ", showWhitespaceCharacters(from))),
			green("%s", createStringWithPrefix("  + ", showWhitespaceCharacters(to))),
		)

	case isMultiLine(from, to):
		if !bunt.UseColors() {
			_, _ = output.WriteString(yellow("%c value change\n", MODIFICATION))
			report.writeTextBlocks(output, 0,
				red("%s", createStringWithPrefix("  - ", from)),
				green("%s", createStringWithPrefix("  + ", to)),
			)

		} else {
			dmp := diffmatchpatch.New()
			diff := dmp.DiffMain(from, to, true)
			diff = dmp.DiffCleanupSemantic(diff)
			diff = dmp.DiffCleanupEfficiency(diff)

			var ins, del int
			var buf bytes.Buffer
			for _, d := range diff {
				switch d.Type {
				case diffmatchpatch.DiffInsert:
					fmt.Fprint(&buf, green("%s", d.Text))
					ins++

				case diffmatchpatch.DiffDelete:
					fmt.Fprint(&buf, red("%s", d.Text))
					del++

				case diffmatchpatch.DiffEqual:
					fmt.Fprint(&buf, dimgray("%s", d.Text))
				}
			}
			fmt.Fprintln(&buf)

			var insDelDetails []string
			if ins > 0 {
				insDelDetails = append(insDelDetails, text.Plural(ins, "insert"))
			}
			if del > 0 {
				insDelDetails = append(insDelDetails, text.Plural(del, "deletion"))
			}

			_, _ = output.WriteString(yellow("%c value change in multiline text (%s)\n", MODIFICATION, strings.Join(insDelDetails, ", ")))
			_, _ = output.WriteString(createStringWithPrefix("    ", buf.String()))
		}

	case isMinorChange(from, to, report.MinorChangeThreshold):
		_, _ = output.WriteString(yellow("%c value change\n", MODIFICATION))
		diffs := diffmatchpatch.New().DiffMain(from, to, false)
		_, _ = output.WriteString(highlightRemovals(diffs))
		_, _ = output.WriteString(highlightAdditions(diffs))

	default:
		_, _ = output.WriteString(yellow("%c value change\n", MODIFICATION))
		_, _ = output.WriteString(red("%s", createStringWithPrefix("  - ", from)))
		_, _ = output.WriteString(green("%s", createStringWithPrefix("  + ", to)))
	}
}

func (report *HumanReport) highlightByLine(from, to string) string {
	fromLines := strings.Split(from, "\n")
	toLines := strings.Split(to, "\n")

	var buf bytes.Buffer

	if len(fromLines) == len(toLines) {
		for i := range fromLines {
			if fromLines[i] != toLines[i] {
				fromLines[i] = red(fromLines[i])
				toLines[i] = green(toLines[i])

			} else {
				fromLines[i] = lightred(fromLines[i])
				toLines[i] = lightgreen(toLines[i])
			}
		}

		report.writeTextBlocks(&buf, 0,
			createStringWithPrefix(red("  - "), strings.Join(fromLines, "\n")),
			createStringWithPrefix(green("  + "), strings.Join(toLines, "\n")))

	} else {
		report.writeTextBlocks(&buf, 0,
			red("%s", createStringWithPrefix("  - ", from)),
			green("%s", createStringWithPrefix("  + ", to)),
		)
	}

	return buf.String()
}

func humanReadableType(node *yamlv3.Node) string {
	switch node.Kind {
	case yamlv3.DocumentNode:
		return "document"

	case yamlv3.MappingNode:
		return "map"

	case yamlv3.SequenceNode:
		return "list"

	case yamlv3.ScalarNode:
		switch node.Tag {
		case "!!str":
			return "string"

		case "!!null":
			return "<nil>"

		default:
			// use the YAML tag name without the exclamation marks
			return node.Tag[2:]
		}

	case yamlv3.AliasNode:
		return humanReadableType(node.Alias)
	}

	panic(fmt.Errorf("unknown and therefore unsupported kind %v", node.Kind))
}

func highlightRemovals(diffs []diffmatchpatch.Diff) string {
	var buf bytes.Buffer

	buf.WriteString(red("  - "))
	for _, part := range diffs {
		switch part.Type {
		case diffmatchpatch.DiffEqual:
			buf.WriteString(lightred("%s", part.Text))

		case diffmatchpatch.DiffDelete:
			buf.WriteString(bold("%s", red("%s", part.Text)))
		}
	}

	buf.WriteString("\n")
	return buf.String()
}

func highlightAdditions(diffs []diffmatchpatch.Diff) string {
	var buf bytes.Buffer

	buf.WriteString(green("  + "))
	for _, part := range diffs {
		switch part.Type {
		case diffmatchpatch.DiffEqual:
			buf.WriteString(lightgreen("%s", part.Text))

		case diffmatchpatch.DiffInsert:
			buf.WriteString(bold("%s", green("%s", part.Text)))
		}
	}

	buf.WriteString("\n")
	return buf.String()
}

// LoadX509Certs tries to load the provided strings as a cert each and returns
// a textual representation of the certs, or an error if the strings are not
// X509 certs
func (report *HumanReport) LoadX509Certs(from, to string) (string, string, error) {
	// Back out quickly if cert inspection is disabled
	if report.DoNotInspectCerts {
		return "", "", fmt.Errorf("certificate inspection is disabled")
	}

	fromDecoded, _ := pem.Decode([]byte(from))
	if fromDecoded == nil {
		return "", "", fmt.Errorf("string '%s' is no PEM string", from)
	}

	toDecoded, _ := pem.Decode([]byte(to))
	if toDecoded == nil {
		return "", "", fmt.Errorf("string '%s' is no PEM string", to)
	}

	fromCert, err := x509.ParseCertificate(fromDecoded.Bytes)
	if err != nil {
		return "", "", err
	}

	toCert, err := x509.ParseCertificate(toDecoded.Bytes)
	if err != nil {
		return "", "", err
	}

	return certificateSummaryAsYAML(fromCert),
		certificateSummaryAsYAML(toCert),
		nil
}

// Create a YAML (hash with key/value) from a certificate to only display a few
// important fields (https://www.sslshopper.com/certificate-decoder.html):
//
//	Common Name: www.example.com
//	Organization: Company Name
//	Organization Unit: Org
//	Locality: Portland
//	State: Oregon
//	Country: US
//	Valid From: April 2, 2018
//	Valid To: April 2, 2019
//	Issuer: www.example.com, Company Name
//	Serial Number: 14581103526614300972 (0xca5a7c67490a792c)
func certificateSummaryAsYAML(cert *x509.Certificate) string {
	const template = `Subject:
  Common Name: %s
  Organization: %s
  Organization Unit: %s
  Locality: %s
  State: %s
  Country: %s
Validity Period:
  NotBefore: %s
  NotAfter: %s
Issuer: %s, %s
Serial Number: %d (%#x)
`

	return fmt.Sprintf(template,
		cert.Subject.CommonName,
		strings.Join(cert.Subject.Organization, " "),
		strings.Join(cert.Subject.OrganizationalUnit, " "),
		strings.Join(cert.Subject.Locality, " "),
		strings.Join(cert.Subject.Province, " "),
		strings.Join(cert.Subject.Country, " "),
		cert.NotBefore.Format("Jan 2 15:04:05 2006 MST"),
		cert.NotAfter.Format("Jan 2 15:04:05 2006 MST"),
		cert.Issuer.CommonName, strings.Join(cert.Issuer.Organization, " "),
		cert.SerialNumber, cert.SerialNumber,
	)
}

func yamlString(input interface{}) (string, error) {
	if input == nil {
		return "<nil>", nil
	}

	switch node := input.(type) {
	case *yamlv3.Node:
		if node.Tag == "!!null" {
			return "<nil>", nil
		}
	}

	return neat.NewOutputProcessor(false, true, nil).ToYAML(input)
}

func isMinorChange(from string, to string, minorChangeThreshold float64) bool {
	levenshteinDistance := levenshtein.DistanceForStrings([]rune(from), []rune(to), levenshtein.DefaultOptions)

	// Special case: Consider it a minor change if only two runes/characters were
	// changed, which results in a default distance of four, two removals and two
	// additions each.
	if levenshteinDistance <= 4 {
		return true
	}

	referenceLength := min(len(from), len(to))
	return float64(levenshteinDistance)/float64(referenceLength) < minorChangeThreshold
}

func isMultiLine(from string, to string) bool {
	return strings.Contains(from, "\n") || strings.Contains(to, "\n")
}

func isWhitespaceOnlyChange(from string, to string) bool {
	return strings.Trim(from, " \n") == strings.Trim(to, " \n")
}

func showWhitespaceCharacters(text string) string {
	return strings.Replace(strings.Replace(text, "\n", bold("↵\n"), -1), " ", bold("·"), -1)
}

func createStringWithPrefix(prefix string, obj interface{}) string {
	var buf bytes.Buffer
	for i, line := range strings.Split(fmt.Sprintf("%v", obj), "\n") {
		if i == 0 {
			buf.WriteString(prefix)

		} else {
			buf.WriteString(strings.Repeat(" ", plainTextLength(prefix)))
		}

		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return buf.String()
}

func plainTextLength(text string) int {
	return utf8.RuneCountInString(bunt.RemoveAllEscapeSequences(text))
}

func stringArrayLen(list []string) int {
	result := 0
	for _, entry := range list {
		result += plainTextLength(entry)
	}

	return result
}

// writeTextBlocks writes strings into the provided buffer in either a table style (each string a column) or list style (each string a row)
func (report *HumanReport) writeTextBlocks(buf stringWriter, indent int, blocks ...string) {
	const separator = "   "

	// Calcuclate the theoretical maximum line length if blocks would be rendered next to each other
	theoreticalMaxLineLength := indent + ((len(blocks) - 1) * plainTextLength(separator))
	for _, block := range blocks {
		maxLineLengthInBlock := 0
		for _, line := range strings.Split(block, "\n") {
			if lineLength := plainTextLength(line); maxLineLengthInBlock < lineLength {
				maxLineLengthInBlock = lineLength
			}
		}

		theoreticalMaxLineLength += maxLineLengthInBlock
	}

	// In case the line with blocks next to each other would surpass the terminal width, fall back to the no-table-style
	if report.NoTableStyle || theoreticalMaxLineLength > term.GetTerminalWidth() {
		for _, block := range blocks {
			lines := strings.Split(block, "\n")
			for _, line := range lines {
				_, _ = buf.WriteString(strings.Repeat(" ", indent))
				_, _ = buf.WriteString(line)
				_, _ = buf.WriteString("\n")
			}
		}

	} else {
		_, _ = buf.WriteString(CreateTableStyleString(separator, indent, blocks...))
	}
}

// CreateTableStyleString takes the multi-line input strings as columns and arranges an output string to create a table-style output format with proper padding so that the text blocks can be arranged next to each other.
func CreateTableStyleString(separator string, indent int, columns ...string) string {
	cols := len(columns)
	rows := -1
	max := make([]int, cols)

	for i, col := range columns {
		lines := strings.Split(col, "\n")
		if noOfLines := len(lines); noOfLines > rows {
			rows = noOfLines
		}

		for _, line := range lines {
			if length := plainTextLength(line); length > max[i] {
				max[i] = length
			}
		}
	}

	mtrx := make([][]string, 0)
	for x := 0; x < rows; x++ {
		mtrx = append(mtrx, make([]string, cols))
		for y := 0; y < cols; y++ {
			mtrx[x][y] = strings.Repeat(" ", max[y]+indent)
		}
	}

	for i, col := range columns {
		for j, line := range strings.Split(col, "\n") {
			mtrx[j][i] = strings.Repeat(" ", indent) +
				line +
				strings.Repeat(" ", max[i]-plainTextLength(line))
		}
	}

	var buf bytes.Buffer
	for i, row := range mtrx {
		buf.WriteString(strings.TrimRight(strings.Join(row, separator), " "))

		if i < len(mtrx)-1 {
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

func styledGoPatchPath(path *ytbx.Path) string {
	if path == nil {
		return bunt.Sprintf("*(file level)*")
	}

	if path.PathElements == nil {
		return bunt.Sprint("*/*")
	}

	sections := []string{""}

	for _, element := range path.PathElements {
		switch {
		case element.Name != "" && element.Key == "":
			sections = append(sections, bunt.Sprintf("*%s*", element.Name))

		case element.Name != "" && element.Key != "":
			sections = append(sections, bunt.Sprintf("*%s*=_*%s*_", element.Key, element.Name))

		default:
			sections = append(sections, bunt.Sprintf("*%d*", element.Idx))
		}
	}

	return strings.Join(sections, "/")
}

func styledDotStylePath(path *ytbx.Path) string {
	if path == nil {
		return bunt.Sprintf("*(file level)*")
	}

	if path.PathElements == nil {
		return bunt.Sprint("*(root level)*")
	}

	sections := []string{}

	for _, element := range path.PathElements {
		switch {
		case element.Key == "" && element.Name != "":
			sections = append(sections, bunt.Sprintf("*%s*", element.Name))

		case element.Key != "" && element.Name != "":
			sections = append(sections, bunt.Sprintf("_*%s*_", element.Name))

		case element.Idx >= 0:
			sections = append(sections, bunt.Sprintf("*%d*", element.Idx))
		}
	}

	return strings.Join(sections, ".")
}
