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
	"github.com/gonvenience/bunt"
	"github.com/gonvenience/neat"
	"github.com/lucasb-eyer/go-colorful"
)

func yamlStringInRedishColors(input interface{}) (string, error) {
	return neat.NewOutputProcessor(true, true, &map[string]colorful.Color{
		"keyColor":           bunt.FireBrick,
		"indentLineColor":    {R: 0.2, G: 0, B: 0},
		"scalarDefaultColor": bunt.LightCoral,
		"boolColor":          bunt.LightCoral,
		"floatColor":         bunt.LightCoral,
		"intColor":           bunt.LightCoral,
		"multiLineTextColor": bunt.DarkSalmon,
		"nullColor":          bunt.Salmon,
		"emptyStructures":    bunt.LightSalmon,
		"dashColor":          bunt.FireBrick,
	}).ToYAML(input)
}

func yamlStringInGreenishColors(input interface{}) (string, error) {
	return neat.NewOutputProcessor(true, true, &map[string]colorful.Color{
		"keyColor":           bunt.Green,
		"indentLineColor":    {R: 0, G: 0.2, B: 0},
		"scalarDefaultColor": bunt.LimeGreen,
		"boolColor":          bunt.LimeGreen,
		"floatColor":         bunt.LimeGreen,
		"intColor":           bunt.LimeGreen,
		"multiLineTextColor": bunt.OliveDrab,
		"nullColor":          bunt.Olive,
		"emptyStructures":    bunt.DarkOliveGreen,
		"dashColor":          bunt.Green,
	}).ToYAML(input)
}
