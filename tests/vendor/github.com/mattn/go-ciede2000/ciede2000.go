package ciede2000

import (
	"image/color"
	"math"
)

type LAB struct {
	L float64
	A float64
	B float64
}

func ToXYZ(c color.Color) (float64, float64, float64) {
	ta, tg, tb, _ := c.RGBA()
	r := float64(ta) / 65535.0
	g := float64(tg) / 65535.0
	b := float64(tb) / 65535.0

	if r > 0.04045 {
		r = math.Pow(((r + 0.055) / 1.055), 2.4)
	} else {
		r = r / 12.92
	}

	if g > 0.04045 {
		g = math.Pow(((g + 0.055) / 1.055), 2.4)
	} else {
		g = g / 12.92
	}

	if b > 0.04045 {
		b = math.Pow(((b + 0.055) / 1.055), 2.4)
	} else {
		b = b / 12.92
	}

	r *= 100
	g *= 100
	b *= 100
	return r*0.4124 + g*0.3576 + b*0.1805, r*0.2126 + g*0.7152 + b*0.0722, r*0.0193 + g*0.1192 + b*0.9505
}

func ToLAB(c color.Color) *LAB {
	x, y, z := ToXYZ(c)
	x /= 95.047
	y /= 100.000
	z /= 108.883

	if x > 0.008856 {
		x = math.Pow(x, (1.0 / 3.0))
	} else {
		x = (7.787 * x) + (16 / 116)
	}

	if y > 0.008856 {
		y = math.Pow(y, (1.0 / 3.0))
	} else {
		y = (7.787 * y) + (16 / 116)
	}

	if z > 0.008856 {
		z = math.Pow(z, (1.0 / 3.0))
	} else {
		z = (7.787 * z) + (16 / 116)
	}

	l := (116 * y) - 16
	a := 500 * (x - y)
	b := 200 * (y - z)

	if l < 0.0 {
		l = 0.0
	}

	return &LAB{l, a, b}
}

func deg2Rad(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

func rad2Deg(rad float64) float64 {
	return (180.0 / math.Pi) * rad
}

func CIEDE2000(lab1, lab2 *LAB) float64 {
	/*
	 * "For these and all other numerical/graphical 􏰀delta E00 values
	 * reported in this article, we set the parametric weighting factors
	 * to unity(i.e., k_L = k_C = k_H = 1.0)." (Page 27).
	 */
	k_L, k_C, k_H := 1.0, 1.0, 1.0
	deg360InRad := deg2Rad(360.0)
	deg180InRad := deg2Rad(180.0)
	pow25To7 := 6103515625.0 /* pow(25, 7) */

	/*
	 * Step 1
	 */
	/* Equation 2 */
	C1 := math.Sqrt((lab1.A * lab1.A) + (lab1.B * lab1.B))
	C2 := math.Sqrt((lab2.A * lab2.A) + (lab2.B * lab2.B))
	/* Equation 3 */
	barC := (C1 + C2) / 2.0
	/* Equation 4 */
	G := 0.5 * (1 - math.Sqrt(math.Pow(barC, 7)/(math.Pow(barC, 7)+pow25To7)))
	/* Equation 5 */
	a1Prime := (1.0 + G) * lab1.A
	a2Prime := (1.0 + G) * lab2.A
	/* Equation 6 */
	CPrime1 := math.Sqrt((a1Prime * a1Prime) + (lab1.B * lab1.B))
	CPrime2 := math.Sqrt((a2Prime * a2Prime) + (lab2.B * lab2.B))
	/* Equation 7 */
	var hPrime1 float64
	if lab1.B == 0 && a1Prime == 0 {
		hPrime1 = 0.0
	} else {
		hPrime1 = math.Atan2(lab1.B, a1Prime)
		/*
		 * This must be converted to a hue angle in degrees between 0
		 * and 360 by addition of 2􏰏 to negative hue angles.
		 */
		if hPrime1 < 0 {
			hPrime1 += deg360InRad
		}
	}
	var hPrime2 float64
	if lab2.B == 0 && a2Prime == 0 {
		hPrime2 = 0.0
	} else {
		hPrime2 = math.Atan2(lab2.B, a2Prime)
		/*
		 * This must be converted to a hue angle in degrees between 0
		 * and 360 by addition of 2􏰏 to negative hue angles.
		 */
		if hPrime2 < 0 {
			hPrime2 += deg360InRad
		}
	}

	/*
	 * Step 2
	 */
	/* Equation 8 */
	deltaLPrime := lab2.L - lab1.L
	/* Equation 9 */
	deltaCPrime := CPrime2 - CPrime1
	/* Equation 10 */
	var deltahPrime float64
	CPrimeProduct := CPrime1 * CPrime2
	if CPrimeProduct == 0 {
		deltahPrime = 0
	} else {
		/* Avoid the fabs() call */
		deltahPrime = hPrime2 - hPrime1
		if deltahPrime < -deg180InRad {
			deltahPrime += deg360InRad
		} else if deltahPrime > deg180InRad {
			deltahPrime -= deg360InRad
		}
	}
	/* Equation 11 */
	deltaHPrime := 2.0 * math.Sqrt(CPrimeProduct) * math.Sin(deltahPrime/2.0)

	/*
	 * Step 3
	 */
	/* Equation 12 */
	barLPrime := (lab1.L + lab2.L) / 2.0
	/* Equation 13 */
	barCPrime := (CPrime1 + CPrime2) / 2.0
	/* Equation 14 */
	var barhPrime float64
	hPrimeSum := hPrime1 + hPrime2
	if CPrime1*CPrime2 == 0 {
		barhPrime = hPrimeSum
	} else {
		if math.Abs(hPrime1-hPrime2) <= deg180InRad {
			barhPrime = hPrimeSum / 2.0
		} else {
			if hPrimeSum < deg360InRad {
				barhPrime = (hPrimeSum + deg360InRad) / 2.0
			} else {
				barhPrime = (hPrimeSum - deg360InRad) / 2.0
			}
		}
	}
	/* Equation 15 */
	T := 1.0 - (0.17 * math.Cos(barhPrime-deg2Rad(30.0))) +
		(0.24 * math.Cos(2.0*barhPrime)) +
		(0.32 * math.Cos((3.0*barhPrime)+deg2Rad(6.0))) -
		(0.20 * math.Cos((4.0*barhPrime)-deg2Rad(63.0)))
	/* Equation 16 */
	deltaTheta := deg2Rad(30.0) * math.Exp(-math.Pow((barhPrime-deg2Rad(275.0))/deg2Rad(25.0), 2.0))
	/* Equation 17 */
	R_C := 2.0 * math.Sqrt(math.Pow(barCPrime, 7.0)/(math.Pow(barCPrime, 7.0)+pow25To7))
	/* Equation 18 */
	S_L := 1 + ((0.015 * math.Pow(barLPrime-50.0, 2.0)) /
		math.Sqrt(20+math.Pow(barLPrime-50.0, 2.0)))
	/* Equation 19 */
	S_C := 1 + (0.045 * barCPrime)
	/* Equation 20 */
	S_H := 1 + (0.015 * barCPrime * T)
	/* Equation 21 */
	R_T := (-math.Sin(2.0 * deltaTheta)) * R_C

	/* Equation 22 */
	return math.Sqrt(
		math.Pow(deltaLPrime/(k_L*S_L), 2.0) +
			math.Pow(deltaCPrime/(k_C*S_C), 2.0) +
			math.Pow(deltaHPrime/(k_H*S_H), 2.0) +
			(R_T * (deltaCPrime / (k_C * S_C)) * (deltaHPrime / (k_H * S_H))))
}

func Diff(c1, c2 color.Color) float64 {
	return CIEDE2000(ToLAB(c1), ToLAB(c2))
}
