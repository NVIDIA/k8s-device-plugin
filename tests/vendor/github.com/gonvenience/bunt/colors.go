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
	"image/color"
	"math"
	"math/rand"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// The named colors are based upon https://en.wikipedia.org/wiki/Web_colors
var (
	Pink            = hexColor("#FFC0CB")
	LightPink       = hexColor("#FFB6C1")
	HotPink         = hexColor("#FF69B4")
	DeepPink        = hexColor("#FF1493")
	PaleVioletRed   = hexColor("#DB7093")
	MediumVioletRed = hexColor("#C71585")

	LightSalmon = hexColor("#FFA07A")
	Salmon      = hexColor("#FA8072")
	DarkSalmon  = hexColor("#E9967A")
	LightCoral  = hexColor("#F08080")
	IndianRed   = hexColor("#CD5C5C")
	Crimson     = hexColor("#DC143C")
	FireBrick   = hexColor("#B22222")
	DarkRed     = hexColor("#8B0000")
	Red         = hexColor("#FF0000")

	OrangeRed  = hexColor("#FF4500")
	Tomato     = hexColor("#FF6347")
	Coral      = hexColor("#FF7F50")
	DarkOrange = hexColor("#FF8C00")
	Orange     = hexColor("#FFA500")

	Yellow               = hexColor("#FFFF00")
	LightYellow          = hexColor("#FFFFE0")
	LemonChiffon         = hexColor("#FFFACD")
	LightGoldenrodYellow = hexColor("#FAFAD2")
	PapayaWhip           = hexColor("#FFEFD5")
	Moccasin             = hexColor("#FFE4B5")
	PeachPuff            = hexColor("#FFDAB9")
	PaleGoldenrod        = hexColor("#EEE8AA")
	Khaki                = hexColor("#F0E68C")
	DarkKhaki            = hexColor("#BDB76B")
	Gold                 = hexColor("#FFD700")

	Cornsilk       = hexColor("#FFF8DC")
	BlanchedAlmond = hexColor("#FFEBCD")
	Bisque         = hexColor("#FFE4C4")
	NavajoWhite    = hexColor("#FFDEAD")
	Wheat          = hexColor("#F5DEB3")
	BurlyWood      = hexColor("#DEB887")
	Tan            = hexColor("#D2B48C")
	RosyBrown      = hexColor("#BC8F8F")
	SandyBrown     = hexColor("#F4A460")
	Goldenrod      = hexColor("#DAA520")
	DarkGoldenrod  = hexColor("#B8860B")
	Peru           = hexColor("#CD853F")
	Chocolate      = hexColor("#D2691E")
	SaddleBrown    = hexColor("#8B4513")
	Sienna         = hexColor("#A0522D")
	Brown          = hexColor("#A52A2A")
	Maroon         = hexColor("#800000")

	DarkOliveGreen    = hexColor("#556B2F")
	Olive             = hexColor("#808000")
	OliveDrab         = hexColor("#6B8E23")
	YellowGreen       = hexColor("#9ACD32")
	LimeGreen         = hexColor("#32CD32")
	Lime              = hexColor("#00FF00")
	LawnGreen         = hexColor("#7CFC00")
	Chartreuse        = hexColor("#7FFF00")
	GreenYellow       = hexColor("#ADFF2F")
	SpringGreen       = hexColor("#00FF7F")
	MediumSpringGreen = hexColor("#00FA9A")
	LightGreen        = hexColor("#90EE90")
	PaleGreen         = hexColor("#98FB98")
	DarkSeaGreen      = hexColor("#8FBC8F")
	MediumAquamarine  = hexColor("#66CDAA")
	MediumSeaGreen    = hexColor("#3CB371")
	SeaGreen          = hexColor("#2E8B57")
	ForestGreen       = hexColor("#228B22")
	Green             = hexColor("#008000")
	DarkGreen         = hexColor("#006400")

	Aqua            = hexColor("#00FFFF")
	Cyan            = hexColor("#00FFFF")
	LightCyan       = hexColor("#E0FFFF")
	PaleTurquoise   = hexColor("#AFEEEE")
	Aquamarine      = hexColor("#7FFFD4")
	Turquoise       = hexColor("#40E0D0")
	MediumTurquoise = hexColor("#48D1CC")
	DarkTurquoise   = hexColor("#00CED1")
	LightSeaGreen   = hexColor("#20B2AA")
	CadetBlue       = hexColor("#5F9EA0")
	DarkCyan        = hexColor("#008B8B")
	Teal            = hexColor("#008080")

	LightSteelBlue = hexColor("#B0C4DE")
	PowderBlue     = hexColor("#B0E0E6")
	LightBlue      = hexColor("#ADD8E6")
	SkyBlue        = hexColor("#87CEEB")
	LightSkyBlue   = hexColor("#87CEFA")
	DeepSkyBlue    = hexColor("#00BFFF")
	DodgerBlue     = hexColor("#1E90FF")
	CornflowerBlue = hexColor("#6495ED")
	SteelBlue      = hexColor("#4682B4")
	RoyalBlue      = hexColor("#4169E1")
	Blue           = hexColor("#0000FF")
	MediumBlue     = hexColor("#0000CD")
	DarkBlue       = hexColor("#00008B")
	Navy           = hexColor("#000080")
	MidnightBlue   = hexColor("#191970")

	Lavender        = hexColor("#E6E6FA")
	Thistle         = hexColor("#D8BFD8")
	Plum            = hexColor("#DDA0DD")
	Violet          = hexColor("#EE82EE")
	Orchid          = hexColor("#DA70D6")
	Fuchsia         = hexColor("#FF00FF")
	Magenta         = hexColor("#FF00FF")
	MediumOrchid    = hexColor("#BA55D3")
	MediumPurple    = hexColor("#9370DB")
	BlueViolet      = hexColor("#8A2BE2")
	DarkViolet      = hexColor("#9400D3")
	DarkOrchid      = hexColor("#9932CC")
	DarkMagenta     = hexColor("#8B008B")
	Purple          = hexColor("#800080")
	Indigo          = hexColor("#4B0082")
	DarkSlateBlue   = hexColor("#483D8B")
	SlateBlue       = hexColor("#6A5ACD")
	MediumSlateBlue = hexColor("#7B68EE")

	White         = hexColor("#FFFFFF")
	Snow          = hexColor("#FFFAFA")
	Honeydew      = hexColor("#F0FFF0")
	MintCream     = hexColor("#F5FFFA")
	Azure         = hexColor("#F0FFFF")
	AliceBlue     = hexColor("#F0F8FF")
	GhostWhite    = hexColor("#F8F8FF")
	WhiteSmoke    = hexColor("#F5F5F5")
	Seashell      = hexColor("#FFF5EE")
	Beige         = hexColor("#F5F5DC")
	OldLace       = hexColor("#FDF5E6")
	FloralWhite   = hexColor("#FFFAF0")
	Ivory         = hexColor("#FFFFF0")
	AntiqueWhite  = hexColor("#FAEBD7")
	Linen         = hexColor("#FAF0E6")
	LavenderBlush = hexColor("#FFF0F5")
	MistyRose     = hexColor("#FFE4E1")

	Gainsboro      = hexColor("#DCDCDC")
	LightGray      = hexColor("#D3D3D3")
	Silver         = hexColor("#C0C0C0")
	DarkGray       = hexColor("#A9A9A9")
	Gray           = hexColor("#808080")
	DimGray        = hexColor("#696969")
	LightSlateGray = hexColor("#778899")
	SlateGray      = hexColor("#708090")
	DarkSlateGray  = hexColor("#2F4F4F")
	Black          = hexColor("#000000")
)

var colorByNameMap = map[string]colorful.Color{
	"Pink":                 Pink,
	"LightPink":            LightPink,
	"HotPink":              HotPink,
	"DeepPink":             DeepPink,
	"PaleVioletRed":        PaleVioletRed,
	"MediumVioletRed":      MediumVioletRed,
	"LightSalmon":          LightSalmon,
	"Salmon":               Salmon,
	"DarkSalmon":           DarkSalmon,
	"LightCoral":           LightCoral,
	"IndianRed":            IndianRed,
	"Crimson":              Crimson,
	"FireBrick":            FireBrick,
	"DarkRed":              DarkRed,
	"Red":                  Red,
	"OrangeRed":            OrangeRed,
	"Tomato":               Tomato,
	"Coral":                Coral,
	"DarkOrange":           DarkOrange,
	"Orange":               Orange,
	"Yellow":               Yellow,
	"LightYellow":          LightYellow,
	"LemonChiffon":         LemonChiffon,
	"LightGoldenrodYellow": LightGoldenrodYellow,
	"PapayaWhip":           PapayaWhip,
	"Moccasin":             Moccasin,
	"PeachPuff":            PeachPuff,
	"PaleGoldenrod":        PaleGoldenrod,
	"Khaki":                Khaki,
	"DarkKhaki":            DarkKhaki,
	"Gold":                 Gold,
	"Cornsilk":             Cornsilk,
	"BlanchedAlmond":       BlanchedAlmond,
	"Bisque":               Bisque,
	"NavajoWhite":          NavajoWhite,
	"Wheat":                Wheat,
	"BurlyWood":            BurlyWood,
	"Tan":                  Tan,
	"RosyBrown":            RosyBrown,
	"SandyBrown":           SandyBrown,
	"Goldenrod":            Goldenrod,
	"DarkGoldenrod":        DarkGoldenrod,
	"Peru":                 Peru,
	"Chocolate":            Chocolate,
	"SaddleBrown":          SaddleBrown,
	"Sienna":               Sienna,
	"Brown":                Brown,
	"Maroon":               Maroon,
	"DarkOliveGreen":       DarkOliveGreen,
	"Olive":                Olive,
	"OliveDrab":            OliveDrab,
	"YellowGreen":          YellowGreen,
	"LimeGreen":            LimeGreen,
	"Lime":                 Lime,
	"LawnGreen":            LawnGreen,
	"Chartreuse":           Chartreuse,
	"GreenYellow":          GreenYellow,
	"SpringGreen":          SpringGreen,
	"MediumSpringGreen":    MediumSpringGreen,
	"LightGreen":           LightGreen,
	"PaleGreen":            PaleGreen,
	"DarkSeaGreen":         DarkSeaGreen,
	"MediumAquamarine":     MediumAquamarine,
	"MediumSeaGreen":       MediumSeaGreen,
	"SeaGreen":             SeaGreen,
	"ForestGreen":          ForestGreen,
	"Green":                Green,
	"DarkGreen":            DarkGreen,
	"Aqua":                 Aqua,
	"Cyan":                 Cyan,
	"LightCyan":            LightCyan,
	"PaleTurquoise":        PaleTurquoise,
	"Aquamarine":           Aquamarine,
	"Turquoise":            Turquoise,
	"MediumTurquoise":      MediumTurquoise,
	"DarkTurquoise":        DarkTurquoise,
	"LightSeaGreen":        LightSeaGreen,
	"CadetBlue":            CadetBlue,
	"DarkCyan":             DarkCyan,
	"Teal":                 Teal,
	"LightSteelBlue":       LightSteelBlue,
	"PowderBlue":           PowderBlue,
	"LightBlue":            LightBlue,
	"SkyBlue":              SkyBlue,
	"LightSkyBlue":         LightSkyBlue,
	"DeepSkyBlue":          DeepSkyBlue,
	"DodgerBlue":           DodgerBlue,
	"CornflowerBlue":       CornflowerBlue,
	"SteelBlue":            SteelBlue,
	"RoyalBlue":            RoyalBlue,
	"Blue":                 Blue,
	"MediumBlue":           MediumBlue,
	"DarkBlue":             DarkBlue,
	"Navy":                 Navy,
	"MidnightBlue":         MidnightBlue,
	"Lavender":             Lavender,
	"Thistle":              Thistle,
	"Plum":                 Plum,
	"Violet":               Violet,
	"Orchid":               Orchid,
	"Fuchsia":              Fuchsia,
	"Magenta":              Magenta,
	"MediumOrchid":         MediumOrchid,
	"MediumPurple":         MediumPurple,
	"BlueViolet":           BlueViolet,
	"DarkViolet":           DarkViolet,
	"DarkOrchid":           DarkOrchid,
	"DarkMagenta":          DarkMagenta,
	"Purple":               Purple,
	"Indigo":               Indigo,
	"DarkSlateBlue":        DarkSlateBlue,
	"SlateBlue":            SlateBlue,
	"MediumSlateBlue":      MediumSlateBlue,
	"White":                White,
	"Snow":                 Snow,
	"Honeydew":             Honeydew,
	"MintCream":            MintCream,
	"Azure":                Azure,
	"AliceBlue":            AliceBlue,
	"GhostWhite":           GhostWhite,
	"WhiteSmoke":           WhiteSmoke,
	"Seashell":             Seashell,
	"Beige":                Beige,
	"OldLace":              OldLace,
	"FloralWhite":          FloralWhite,
	"Ivory":                Ivory,
	"AntiqueWhite":         AntiqueWhite,
	"Linen":                Linen,
	"LavenderBlush":        LavenderBlush,
	"MistyRose":            MistyRose,
	"Gainsboro":            Gainsboro,
	"LightGray":            LightGray,
	"Silver":               Silver,
	"DarkGray":             DarkGray,
	"Gray":                 Gray,
	"DimGray":              DimGray,
	"LightSlateGray":       LightSlateGray,
	"SlateGray":            SlateGray,
	"DarkSlateGray":        DarkSlateGray,
	"Black":                Black,
}

var colorPalette8bit map[uint8]colorful.Color = func() map[uint8]colorful.Color {
	var rgb = func(r, g, b uint8) colorful.Color {
		return colorful.Color{
			R: float64(r) / 255.0,
			G: float64(g) / 255.0,
			B: float64(b) / 255.0,
		}
	}

	palette := make(map[uint8]colorful.Color, 256)

	// Standard colors
	palette[0] = rgb(0, 0, 0)       // Black
	palette[1] = rgb(170, 0, 0)     // Red
	palette[2] = rgb(0, 170, 0)     // Green
	palette[3] = rgb(229, 229, 16)  // Yellow
	palette[4] = rgb(0, 0, 170)     // Blue
	palette[5] = rgb(170, 0, 170)   // Magenta
	palette[6] = rgb(0, 170, 170)   // Cyan
	palette[7] = rgb(229, 229, 229) // White

	// High-intensity colors
	palette[8] = rgb(85, 85, 85)     // Bright Black (Gray)
	palette[9] = rgb(255, 85, 85)    // Bright Red
	palette[10] = rgb(85, 255, 85)   // Bright Green
	palette[11] = rgb(255, 255, 85)  // Bright Yellow
	palette[12] = rgb(85, 85, 255)   // Bright Blue
	palette[13] = rgb(255, 85, 255)  // Bright Magenta
	palette[14] = rgb(85, 255, 255)  // Bright Cyan
	palette[15] = rgb(255, 255, 255) // Bright White

	// 216 prepared colors
	for b := 0; b <= 5; b++ {
		for g := 0; g <= 5; g++ {
			for r := 0; r <= 5; r++ {
				palette[uint8(16+36*r+6*g+b)] = rgb(uint8(r*51), uint8(g*51), uint8(b*51))
			}
		}
	}

	// 24 grayscale shades
	for i := 232; i < 256; i++ {
		value := uint8(float32(i-232) * (255.0 / 23.0))
		palette[uint8(i)] = rgb(value, value, value)
	}

	return palette
}()

func hexColor(scol string) colorful.Color {
	c, _ := colorful.Hex(scol)
	return c
}

func lookupColorByName(colorName string) *colorful.Color {
	// Try to lookup a color by a supplied hexcode
	if strings.HasPrefix(colorName, "#") && len(colorName) == 7 {
		if color, err := colorful.Hex(colorName); err == nil {
			return &color
		}
	}

	// Try to lookup color by searching in the known colors table
	if color, ok := colorByNameMap[colorName]; ok {
		return &color
	}

	// Give up
	return nil
}

// RandomTerminalFriendlyColors creates a list of random 24 bit colors based on
// the 4 bit colors that most terminals support.
func RandomTerminalFriendlyColors(n int) []colorful.Color {
	if n < 0 {
		panic("size is out of bounds, must be greater than zero")
	}

	f := func(i uint8) uint8 {
		const threshold = 128
		if i < threshold {
			return i
		}

		maxFactor := .5
		randomFactor := 1 + (rand.Float64()*2*maxFactor - maxFactor)

		return uint8(math.Max(
			math.Min(
				randomFactor*float64(i),
				255.0,
			),
			float64(threshold),
		))
	}

	baseColors := [][]uint8{
		{0xAA, 0x00, 0x00},
		{0x00, 0xAA, 0x00},
		{0xFF, 0xFF, 0x00},
		{0x00, 0x00, 0xAA},
		{0xAA, 0x00, 0xAA},
		{0x00, 0xAA, 0xAA},
		{0xAA, 0xAA, 0xAA},
		{0xFF, 0x55, 0x55},
		{0x55, 0xFF, 0x55},
		{0xFF, 0xFF, 0x55},
		{0x55, 0x55, 0xFF},
		{0xFF, 0x55, 0xFF},
		{0x55, 0xFF, 0xFF},
		{0xFF, 0xFF, 0xFF},
	}

	result := make([]colorful.Color, n)
	for i := 0; i < n; i++ {
		baseColorRGB := baseColors[i%len(baseColors)]
		r, g, b := baseColorRGB[0], baseColorRGB[1], baseColorRGB[2]

		color, _ := colorful.MakeColor(color.RGBA{f(r), f(g), f(b), 0xFF})
		result[i] = color
	}

	return result
}
