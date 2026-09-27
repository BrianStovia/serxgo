package instant

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

// CheckConverters evaluates timezone and extended digital storage / speed conversions
func CheckConverters(query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. Timezone Check
	if tzAns := checkTimezoneConversion(qLower); tzAns != nil {
		return tzAns
	}

	// 2. Digital Storage & Speed Conversion
	if storageAns := checkExtendedStorageConversion(qLower); storageAns != nil {
		return storageAns
	}

	return nil
}

// -------------------------------------------------------------
// Timezone Converter & Global Time Card
// -------------------------------------------------------------

var tzCityMap = map[string]string{
	"tokyo":       "Asia/Tokyo",
	"jakarta":     "Asia/Jakarta",
	"bali":        "Asia/Makassar",
	"surabaya":    "Asia/Jakarta",
	"singapore":   "Asia/Singapore",
	"london":      "Europe/London",
	"new york":    "America/New_York",
	"ny":          "America/New_York",
	"nyc":         "America/New_York",
	"los angeles": "America/Los_Angeles",
	"la":          "America/Los_Angeles",
	"san francisco": "America/Los_Angeles",
	"chicago":     "America/Chicago",
	"paris":       "Europe/Paris",
	"berlin":      "Europe/Berlin",
	"amsterdam":   "Europe/Amsterdam",
	"dubai":       "Asia/Dubai",
	"sydney":      "Australia/Sydney",
	"melbourne":   "Australia/Melbourne",
	"seoul":       "Asia/Seoul",
	"hong kong":   "Asia/Hong_Kong",
	"taipei":      "Asia/Taipei",
	"mumbai":      "Asia/Kolkata",
	"delhi":       "Asia/Kolkata",
	"bangkok":     "Asia/Bangkok",
	"toronto":     "America/Toronto",
	"vancouver":   "America/Vancouver",
	"auckland":    "Pacific/Auckland",
	"moscow":      "Europe/Moscow",
	"sao paulo":   "America/Sao_Paulo",
}

var tzAbbrOffset = map[string]int{
	"utc":   0,
	"gmt":   0,
	"wib":   7 * 3600,
	"wita":  8 * 3600,
	"wit":   9 * 3600,
	"est":   -5 * 3600,
	"edt":   -4 * 3600,
	"cst":   -6 * 3600,
	"cdt":   -5 * 3600,
	"mst":   -7 * 3600,
	"mdt":   -6 * 3600,
	"pst":   -8 * 3600,
	"pdt":   -7 * 3600,
	"cet":   1 * 3600,
	"cest":  2 * 3600,
	"bst":   1 * 3600,
	"eet":   2 * 3600,
	"eest":  3 * 3600,
	"jst":   9 * 3600,
	"kst":   9 * 3600,
	"ist":   5*3600 + 1800, // UTC+5:30
	"sgt":   8 * 3600,
	"hkt":   8 * 3600,
	"aest":  10 * 3600,
	"aedt":  11 * 3600,
	"nzst":  12 * 3600,
}

func parseTimezoneLocation(name string) (*time.Location, string, bool) {
	clean := strings.TrimSpace(name)
	if locName, ok := tzCityMap[clean]; ok {
		loc, err := time.LoadLocation(locName)
		if err == nil {
			return loc, strings.Title(clean), true
		}
	}
	if offset, ok := tzAbbrOffset[clean]; ok {
		loc := time.FixedZone(strings.ToUpper(clean), offset)
		return loc, strings.ToUpper(clean), true
	}
	loc, err := time.LoadLocation(clean)
	if err == nil {
		return loc, clean, true
	}
	return nil, "", false
}

// Regex to match "time in <city>", "current time in <city>", "what time is it in <city>"
var rxTimeIn = regexp.MustCompile(`^(?:current\s+)?time\s+in\s+([a-zA-Z\s/_]+)$|^(?:what\s+time\s+is\s+it\s+in\s+)([a-zA-Z\s/_]+)$`)

// Regex to match "<time> <from_tz> to/in <to_tz>", e.g. "10am utc to wib", "14:30 est in tokyo"
var rxTzConvert = regexp.MustCompile(`^(\d{1,2}(?::\d{2})?\s*(?:am|pm)?)\s+([a-zA-Z\s/_]+?)\s+(?:to|in)\s+([a-zA-Z\s/_]+)$`)

func checkTimezoneConversion(q string) *models.InstantAnswer {
	// Case 1: "time in <place>"
	if matches := rxTimeIn.FindStringSubmatch(q); len(matches) > 0 {
		targetPlace := strings.TrimSpace(matches[1])
		if targetPlace == "" {
			targetPlace = strings.TrimSpace(matches[2])
		}
		loc, displayName, ok := parseTimezoneLocation(targetPlace)
		if !ok {
			return nil
		}
		nowInLoc := time.Now().In(loc)
		timeStr := nowInLoc.Format("15:04:05 (3:04 PM)")
		dateStr := nowInLoc.Format("Monday, 02 Jan 2006")
		zoneName, offsetSec := nowInLoc.Zone()
		offsetHours := float64(offsetSec) / 3600.0

		desc := fmt.Sprintf(`
		<div style="display:flex; align-items:center; gap:1.25rem; flex-wrap:wrap; margin-top:0.5rem;">
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-md); padding:0.75rem 1.25rem;">
				<div style="font-size:0.8rem; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.05em;">Current Date</div>
				<div style="font-size:1.05rem; font-weight:600; color:var(--text-primary);">%s</div>
			</div>
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-md); padding:0.75rem 1.25rem;">
				<div style="font-size:0.8rem; color:var(--text-muted); text-transform:uppercase; letter-spacing:0.05em;">Offset</div>
				<div style="font-size:1.05rem; font-weight:600; color:var(--accent-primary);">UTC%+g (%s)</div>
			</div>
		</div>`, dateStr, offsetHours, zoneName)

		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("🕒 Current Time in %s", displayName),
			Value:       timeStr,
			Description: desc,
		}
	}

	// Case 2: "<time> <from_tz> to <to_tz>"
	if matches := rxTzConvert.FindStringSubmatch(q); len(matches) == 4 {
		timePart := strings.TrimSpace(matches[1])
		fromPart := strings.TrimSpace(matches[2])
		toPart := strings.TrimSpace(matches[3])

		fromLoc, fromName, okFrom := parseTimezoneLocation(fromPart)
		toLoc, toName, okTo := parseTimezoneLocation(toPart)
		if !okFrom || !okTo {
			return nil
		}

		parsedHour, parsedMin, okTime := parseHourMinute(timePart)
		if !okTime {
			return nil
		}

		now := time.Now()
		tSource := time.Date(now.Year(), now.Month(), now.Day(), parsedHour, parsedMin, 0, 0, fromLoc)
		tDest := tSource.In(toLoc)

		sourceDisplay := tSource.Format("15:04 (3:04 PM)")
		destDisplay := tDest.Format("15:04 (3:04 PM)")
		destFullDate := tDest.Format("Monday, 02 Jan 2006")

		_, fromOffset := tSource.Zone()
		_, toOffset := tDest.Zone()
		diffHours := float64(toOffset-fromOffset) / 3600.0
		diffStr := fmt.Sprintf("%+g hours", diffHours)
		if diffHours == 0 {
			diffStr = "Same time"
		} else if diffHours > 0 {
			diffStr = fmt.Sprintf("+%g hours ahead", diffHours)
		} else {
			diffStr = fmt.Sprintf("%g hours behind", diffHours)
		}

		desc := fmt.Sprintf(`
		<div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap:1rem; margin-top:0.5rem;">
			<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-md); padding:0.75rem 1rem;">
				<div style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase;">Source Time (%s)</div>
				<div style="font-size:1.2rem; font-weight:700; color:var(--text-secondary); margin-top:0.25rem;">%s</div>
				<div style="font-size:0.8rem; color:var(--text-muted); margin-top:0.25rem;">%s</div>
			</div>
			<div style="background:var(--bg-glass-card); border:1px solid var(--accent-primary); border-radius:var(--radius-md); padding:0.75rem 1rem;">
				<div style="font-size:0.75rem; color:var(--accent-primary); font-weight:700; text-transform:uppercase;">Converted Time (%s)</div>
				<div style="font-size:1.3rem; font-weight:800; color:var(--accent-primary); margin-top:0.25rem;">%s</div>
				<div style="font-size:0.8rem; color:var(--text-secondary); margin-top:0.25rem;">%s &bull; <span style="color:var(--accent-emerald); font-weight:600;">%s</span></div>
			</div>
		</div>`, fromName, sourceDisplay, tSource.Format("02 Jan 2006"), toName, destDisplay, destFullDate, diffStr)

		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("🌐 Timezone Conversion: %s ➔ %s", fromName, toName),
			Value:       fmt.Sprintf("%s in %s", destDisplay, toName),
			Description: desc,
		}
	}

	return nil
}

func parseHourMinute(s string) (int, int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	isPM := strings.HasSuffix(s, "pm")
	isAM := strings.HasSuffix(s, "am")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "pm"), "am")
	s = strings.TrimSpace(s)

	parts := strings.Split(s, ":")
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 24 {
		return 0, 0, false
	}
	minute := 0
	if len(parts) > 1 {
		minute, err = strconv.Atoi(parts[1])
		if err != nil || minute < 0 || minute > 59 {
			return 0, 0, false
		}
	}
	if isPM && hour < 12 {
		hour += 12
	} else if isAM && hour == 12 {
		hour = 0
	}
	return hour, minute, true
}

// -------------------------------------------------------------
// Digital Storage & Speed Converter
// -------------------------------------------------------------

var rxUnitConvert = regexp.MustCompile(`^([\d,.]+)\s*([a-zA-Z°]+)\s+(?:to|in)\s+([a-zA-Z°]+)$`)

func checkExtendedStorageConversion(q string) *models.InstantAnswer {
	matches := rxUnitConvert.FindStringSubmatch(q)
	if len(matches) != 4 {
		return nil
	}

	valStr := strings.ReplaceAll(matches[1], ",", "")
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return nil
	}

	fromU := strings.ToLower(strings.TrimSpace(matches[2]))
	toU := strings.ToLower(strings.TrimSpace(matches[3]))

	// 1. Temperatures
	if isTempUnit(fromU) && isTempUnit(toU) {
		result, formula := convertTemperature(val, fromU, toU)
		return &models.InstantAnswer{
			Type:        "calculator",
			Title:       "🌡️ Temperature Conversion",
			Value:       fmt.Sprintf("%.2f %s = %.2f %s", val, strings.ToUpper(fromU), result, strings.ToUpper(toU)),
			Description: fmt.Sprintf(`<span style="color:var(--text-muted); font-size:0.875rem;">Formula: %s</span>`, formula),
		}
	}

	// 2. Digital Storage
	storageUnits := map[string]float64{
		"b": 1, "byte": 1, "bytes": 1,
		"kb": 1e3, "kib": 1024,
		"mb": 1e6, "mib": 1024 * 1024,
		"gb": 1e9, "gib": 1024 * 1024 * 1024,
		"tb": 1e12, "tib": 1024 * 1024 * 1024 * 1024,
		"pb": 1e15, "pib": 1024 * 1024 * 1024 * 1024 * 1024,
	}
	if fromFactor, okFrom := storageUnits[fromU]; okFrom {
		if toFactor, okTo := storageUnits[toU]; okTo {
			bytes := val * fromFactor
			converted := bytes / toFactor
			return buildUnitCard("💾 Digital Storage Conversion", val, strings.ToUpper(fromU), converted, strings.ToUpper(toU))
		}
	}

	// 3. Length & Distance
	lengthUnits := map[string]float64{
		"m": 1.0, "meter": 1.0, "meters": 1.0,
		"km": 1000.0, "kilometer": 1000.0, "kilometers": 1000.0,
		"cm": 0.01, "centimeter": 0.01, "centimeters": 0.01,
		"mm": 0.001, "millimeter": 0.001,
		"mi": 1609.344, "mile": 1609.344, "miles": 1609.344,
		"ft": 0.3048, "foot": 0.3048, "feet": 0.3048,
		"in": 0.0254, "inch": 0.0254, "inches": 0.0254,
		"yd": 0.9144, "yard": 0.9144, "yards": 0.9144,
	}
	if fromFactor, okFrom := lengthUnits[fromU]; okFrom {
		if toFactor, okTo := lengthUnits[toU]; okTo {
			meters := val * fromFactor
			converted := meters / toFactor
			return buildUnitCard("📏 Length & Distance Conversion", val, fromU, converted, toU)
		}
	}

	// 4. Mass & Weight
	massUnits := map[string]float64{
		"kg": 1.0, "kilogram": 1.0, "kilograms": 1.0,
		"g": 0.001, "gram": 0.001, "grams": 0.001,
		"mg": 1e-6, "milligram": 1e-6,
		"lb": 0.45359237, "lbs": 0.45359237, "pound": 0.45359237, "pounds": 0.45359237,
		"oz": 0.028349523, "ounce": 0.028349523, "ounces": 0.028349523,
		"ton": 1000.0, "tons": 1000.0, "tonne": 1000.0,
	}
	if fromFactor, okFrom := massUnits[fromU]; okFrom {
		if toFactor, okTo := massUnits[toU]; okTo {
			kg := val * fromFactor
			converted := kg / toFactor
			return buildUnitCard("⚖️ Mass & Weight Conversion", val, fromU, converted, toU)
		}
	}

	// 5. Speed
	speedUnits := map[string]float64{
		"mps": 1.0, "m/s": 1.0,
		"kph": 0.27777778, "km/h": 0.27777778, "kmh": 0.27777778,
		"mph": 0.44704,
		"knot": 0.514444, "knots": 0.514444,
	}
	if fromFactor, okFrom := speedUnits[fromU]; okFrom {
		if toFactor, okTo := speedUnits[toU]; okTo {
			mps := val * fromFactor
			converted := mps / toFactor
			return buildUnitCard("⚡ Speed Conversion", val, fromU, converted, toU)
		}
	}

	return nil
}

func isTempUnit(u string) bool {
	u = strings.TrimPrefix(u, "°")
	return u == "c" || u == "celsius" || u == "f" || u == "fahrenheit" || u == "k" || u == "kelvin"
}

func convertTemperature(val float64, fromU, toU string) (float64, string) {
	fromU = strings.TrimPrefix(fromU, "°")
	toU = strings.TrimPrefix(toU, "°")

	var celsius float64
	switch fromU {
	case "c", "celsius":
		celsius = val
	case "f", "fahrenheit":
		celsius = (val - 32) * 5.0 / 9.0
	case "k", "kelvin":
		celsius = val - 273.15
	}

	switch toU {
	case "c", "celsius":
		return celsius, "T(°C) = T(°C)"
	case "f", "fahrenheit":
		return (celsius * 9.0 / 5.0) + 32, "(°C × 9/5) + 32"
	case "k", "kelvin":
		return celsius + 273.15, "°C + 273.15"
	default:
		return celsius, ""
	}
}

func buildUnitCard(title string, valFrom float64, unitFrom string, valTo float64, unitTo string) *models.InstantAnswer {
	formatFloat := func(num float64) string {
		if math.Abs(num) < 0.0001 || math.Abs(num) >= 1e9 {
			return fmt.Sprintf("%.4e", num)
		}
		if num == math.Floor(num) {
			return fmt.Sprintf("%.0f", num)
		}
		return fmt.Sprintf("%.4f", num)
	}

	valueStr := fmt.Sprintf("%s %s = %s %s", formatFloat(valFrom), unitFrom, formatFloat(valTo), unitTo)
	desc := fmt.Sprintf(`
	<div style="margin-top:0.35rem; font-size:0.875rem; color:var(--text-muted);">
		Standard SI / Metric and Imperial unit calculation.
	</div>`)

	return &models.InstantAnswer{
		Type:        "calculator",
		Title:       title,
		Value:       valueStr,
		Description: desc,
	}
}
