package instant

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"searxgo/internal/models"
)

// CheckColorTool checks if query is a color format (HEX, RGB, HSL, or named color)
func CheckColorTool(query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. Hex Color: #fff, #ffffff, #ff5722, or hex #...
	hexClean := strings.TrimPrefix(qLower, "color ")
	hexClean = strings.TrimPrefix(hexClean, "hex ")
	hexClean = strings.TrimSpace(hexClean)

	if strings.HasPrefix(hexClean, "#") && (len(hexClean) == 4 || len(hexClean) == 7 || len(hexClean) == 9) {
		r, g, b, ok := parseHexColor(hexClean)
		if ok {
			return buildColorCard(r, g, b, hexClean)
		}
	}

	// 2. RGB Color: rgb(255, 87, 34)
	if strings.HasPrefix(qLower, "rgb(") || strings.HasPrefix(qLower, "rgba(") {
		r, g, b, ok := parseRGBColor(qLower)
		if ok {
			hexVal := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			return buildColorCard(r, g, b, hexVal)
		}
	}

	// 3. HSL Color: hsl(14, 100%, 57%)
	if strings.HasPrefix(qLower, "hsl(") || strings.HasPrefix(qLower, "hsla(") {
		r, g, b, ok := parseHSLColor(qLower)
		if ok {
			hexVal := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			return buildColorCard(r, g, b, hexVal)
		}
	}

	// 4. Named colors: "color coral", "color cyan", "color rebeccapurple"
	if strings.HasPrefix(qLower, "color ") {
		name := strings.TrimSpace(strings.TrimPrefix(qLower, "color "))
		if hexNamed, ok := namedColors[name]; ok {
			r, g, b, _ := parseHexColor(hexNamed)
			return buildColorCard(r, g, b, hexNamed)
		}
	}

	return nil
}

func parseHexColor(s string) (uint8, uint8, uint8, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	} else if len(s) == 8 {
		s = s[:6]
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(val >> 16), uint8((val >> 8) & 0xFF), uint8(val & 0xFF), true
}

var rxRGB = regexp.MustCompile(`rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})`)

func parseRGBColor(s string) (uint8, uint8, uint8, bool) {
	matches := rxRGB.FindStringSubmatch(s)
	if len(matches) < 4 {
		return 0, 0, 0, false
	}
	r, _ := strconv.Atoi(matches[1])
	g, _ := strconv.Atoi(matches[2])
	b, _ := strconv.Atoi(matches[3])
	if r > 255 || g > 255 || b > 255 || r < 0 || g < 0 || b < 0 {
		return 0, 0, 0, false
	}
	return uint8(r), uint8(g), uint8(b), true
}

var rxHSL = regexp.MustCompile(`hsla?\(\s*([\d.]+)\s*,\s*([\d.]+)%?\s*,\s*([\d.]+)%?`)

func parseHSLColor(s string) (uint8, uint8, uint8, bool) {
	matches := rxHSL.FindStringSubmatch(s)
	if len(matches) < 4 {
		return 0, 0, 0, false
	}
	h, _ := strconv.ParseFloat(matches[1], 64)
	sat, _ := strconv.ParseFloat(matches[2], 64)
	l, _ := strconv.ParseFloat(matches[3], 64)

	sat /= 100.0
	l /= 100.0

	c := (1.0 - math.Abs(2.0*l-1.0)) * sat
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}

	r := uint8(math.Round((r1 + m) * 255))
	g := uint8(math.Round((g1 + m) * 255))
	b := uint8(math.Round((b1 + m) * 255))
	return r, g, b, true
}

func rgbToHSL(r, g, b uint8) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min

	l := (max + min) / 2.0

	if delta == 0 {
		return 0, 0, l * 100.0
	}

	var s float64
	if l > 0.5 {
		s = delta / (2.0 - max - min)
	} else {
		s = delta / (max + min)
	}

	var h float64
	switch max {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6.0
		}
	case gf:
		h = (bf-rf)/delta + 2.0
	case bf:
		h = (rf-gf)/delta + 4.0
	}
	h *= 60.0

	return math.Round(h), math.Round(s * 100.0), math.Round(l * 100.0)
}

func rgbToCMYK(r, g, b uint8) (int, int, int, int) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	k := 1.0 - math.Max(rf, math.Max(gf, bf))
	if k == 1.0 {
		return 0, 0, 0, 100
	}
	c := (1.0 - rf - k) / (1.0 - k)
	m := (1.0 - gf - k) / (1.0 - k)
	y := (1.0 - bf - k) / (1.0 - k)

	return int(math.Round(c * 100)), int(math.Round(m * 100)), int(math.Round(y * 100)), int(math.Round(k * 100))
}

func calculateLuminance(r, g, b uint8) float64 {
	calc := func(v uint8) float64 {
		val := float64(v) / 255.0
		if val <= 0.03928 {
			return val / 12.92
		}
		return math.Pow((val+0.055)/1.055, 2.4)
	}
	return 0.2126*calc(r) + 0.7152*calc(g) + 0.0722*calc(b)
}

func buildColorCard(r, g, b uint8, hexStr string) *models.InstantAnswer {
	if !strings.HasPrefix(hexStr, "#") {
		hexStr = "#" + hexStr
	}
	hexStr = strings.ToUpper(hexStr)
	rgbStr := fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
	h, s, l := rgbToHSL(r, g, b)
	hslStr := fmt.Sprintf("hsl(%.0f, %.0f%%, %.0f%%)", h, s, l)
	c, m, y, k := rgbToCMYK(r, g, b)
	cmykStr := fmt.Sprintf("cmyk(%d%%, %d%%, %d%%, %d%%)", c, m, y, k)

	// Contrast calculation
	lum := calculateLuminance(r, g, b)
	contrastWhite := (1.0 + 0.05) / (lum + 0.05)
	contrastBlack := (lum + 0.05) / (0.0 + 0.05)
	if lum > 1.0 {
		lum = 1.0
	}

	bestText := "#ffffff"
	if contrastBlack > contrastWhite {
		bestText = "#000000"
	}

	desc := fmt.Sprintf(`
	<div style="display:flex; gap:1.25rem; align-items:center; flex-wrap:wrap; margin-top:0.75rem;">
		<!-- Large Swatch -->
		<div style="width:90px; height:90px; background-color:%s; border-radius:var(--radius-md); box-shadow:0 4px 14px rgba(0,0,0,0.3); border:2px solid rgba(255,255,255,0.15); display:flex; align-items:center; justify-content:center; color:%s; font-weight:700; font-size:0.75rem; text-align:center;">
			%s
		</div>

		<!-- Color Details Grid -->
		<div style="flex:1; display:grid; grid-template-columns:repeat(auto-fit, minmax(130px, 1fr)); gap:0.6rem;">
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-sm); padding:0.5rem 0.75rem;">
				<div style="font-size:0.7rem; color:var(--text-muted); text-transform:uppercase;">HEX</div>
				<div style="font-family:monospace; font-weight:700; color:var(--text-primary); margin-top:2px;">%s</div>
			</div>
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-sm); padding:0.5rem 0.75rem;">
				<div style="font-size:0.7rem; color:var(--text-muted); text-transform:uppercase;">RGB</div>
				<div style="font-family:monospace; font-weight:700; color:var(--text-primary); margin-top:2px;">%s</div>
			</div>
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-sm); padding:0.5rem 0.75rem;">
				<div style="font-size:0.7rem; color:var(--text-muted); text-transform:uppercase;">HSL</div>
				<div style="font-family:monospace; font-weight:700; color:var(--text-primary); margin-top:2px;">%s</div>
			</div>
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-sm); padding:0.5rem 0.75rem;">
				<div style="font-size:0.7rem; color:var(--text-muted); text-transform:uppercase;">CMYK</div>
				<div style="font-family:monospace; font-weight:700; color:var(--text-primary); margin-top:2px;">%s</div>
			</div>
		</div>
	</div>
	<div style="margin-top:0.75rem; font-size:0.8rem; color:var(--text-secondary); display:flex; gap:1rem; align-items:center;">
		<span>WCAG Contrast: <strong style="color:var(--text-primary);">%.2f:1</strong> on White &bull; <strong style="color:var(--text-primary);">%.2f:1</strong> on Black</span>
	</div>`, hexStr, bestText, hexStr, hexStr, rgbStr, hslStr, cmykStr, contrastWhite, contrastBlack)

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🎨 Color Inspector: %s", hexStr),
		Value:       fmt.Sprintf("%s &bull; %s", hexStr, rgbStr),
		Description: desc,
	}
}

var namedColors = map[string]string{
	"red": "#ff0000", "green": "#008000", "blue": "#0000ff", "cyan": "#00ffff", "magenta": "#ff00ff",
	"yellow": "#ffff00", "black": "#000000", "white": "#ffffff", "gray": "#808080", "grey": "#808080",
	"orange": "#ffa500", "purple": "#800080", "pink": "#ffc0cb", "brown": "#a52a2a", "coral": "#ff7f50",
	"crimson": "#dc143c", "darkblue": "#00008b", "gold": "#ffd700", "indigo": "#4b0082", "khaki": "#f0e68c",
	"lavender": "#e6e6fa", "lime": "#00ff00", "maroon": "#800000", "navy": "#000080", "olive": "#808000",
	"rebeccapurple": "#663399", "salmon": "#fa8072", "silver": "#c0c0c0", "teal": "#008080", "violet": "#ee82ee",
}
