package instant

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"searxgo/internal/models"
)

// CheckCalculator evaluates mathematical queries or unit conversions
func CheckCalculator(query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	// 1. Check unit conversions
	if ans := checkUnitConversion(q); ans != nil {
		return ans
	}

	// 2. Check statistics: e.g. "mean 10, 20, 30", "median 1, 3, 5, 7, 9", "sum 5 10 15", "min 4, 2, 8", "max 4, 2, 8" (SearXNG statistics answerer)
	if statsAns := checkStatistics(q); statsAns != nil {
		return statsAns
	}

	// 3. Check percentage: e.g., "15% of 250" or "20% of 80"
	pctRegex := regexp.MustCompile(`(?i)^([0-9.]+)\s*%\s*(?:of)\s*([0-9.]+)$`)
	if matches := pctRegex.FindStringSubmatch(q); len(matches) == 3 {
		p, err1 := strconv.ParseFloat(matches[1], 64)
		v, err2 := strconv.ParseFloat(matches[2], 64)
		if err1 == nil && err2 == nil {
			res := (p / 100.0) * v
			return &models.InstantAnswer{
				Type:        "calculator",
				Title:       "Percentage Calculation",
				Value:       formatFloat(res),
				Description: fmt.Sprintf("%s%% of %s = %s", matches[1], matches[2], formatFloat(res)),
			}
		}
	}

	// 3. Check general math expression
	// Must contain at least one math operator (+, -, *, /, ^, sqrt, sin, cos, tan, log)
	if !isMathExpression(q) {
		return nil
	}

	res, err := evaluateMath(q)
	if err != nil {
		return nil
	}

	return &models.InstantAnswer{
		Type:        "calculator",
		Title:       "Calculation",
		Value:       formatFloat(res),
		Description: fmt.Sprintf("%s = %s", q, formatFloat(res)),
	}
}

func isMathExpression(s string) bool {
	sLower := strings.ToLower(s)
	// Must have math characters and numbers
	hasDigit := false
	hasOperator := false

	validChars := "0123456789.+-*/^%() sqrtcosinatlpegx "
	for _, ch := range sLower {
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
		if strings.ContainsRune("+-*/^", ch) {
			hasOperator = true
		}
		if !strings.ContainsRune(validChars, ch) {
			return false
		}
	}

	// Also allow functions like sqrt(144)
	if strings.Contains(sLower, "sqrt") || strings.Contains(sLower, "sin") ||
		strings.Contains(sLower, "cos") || strings.Contains(sLower, "tan") ||
		strings.Contains(sLower, "log") {
		hasOperator = true
	}

	return hasDigit && hasOperator
}

func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "Undefined"
	}
	if math.Abs(f-math.Round(f)) < 1e-9 {
		return fmt.Sprintf("%.0f", f)
	}
	return strconv.FormatFloat(f, 'f', 4, 64)
}

// Simple recursive descent parser for mathematical expressions
type mathParser struct {
	s   string
	pos int
}

func evaluateMath(expr string) (float64, error) {
	// Clean string
	expr = strings.ToLower(strings.ReplaceAll(expr, " ", ""))
	expr = strings.ReplaceAll(expr, "pi", fmt.Sprintf("%f", math.Pi))
	expr = strings.ReplaceAll(expr, "e", fmt.Sprintf("%f", math.E))
	expr = strings.ReplaceAll(expr, "x", "*")

	p := &mathParser{s: expr, pos: 0}
	val, err := p.parseExpression()
	if err != nil || p.pos < len(p.s) {
		return 0, fmt.Errorf("invalid math expression")
	}
	return val, nil
}

func (p *mathParser) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *mathParser) get() byte {
	if p.pos < len(p.s) {
		ch := p.s[p.pos]
		p.pos++
		return ch
	}
	return 0
}

func (p *mathParser) parseExpression() (float64, error) {
	val, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for p.peek() == '+' || p.peek() == '-' {
		op := p.get()
		rhs, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			val += rhs
		} else {
			val -= rhs
		}
	}
	return val, nil
}

func (p *mathParser) parseTerm() (float64, error) {
	val, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	for p.peek() == '*' || p.peek() == '/' || p.peek() == '%' {
		op := p.get()
		rhs, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			val *= rhs
		} else if op == '/' {
			if rhs == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			val /= rhs
		} else if op == '%' {
			val = math.Mod(val, rhs)
		}
	}
	return val, nil
}

func (p *mathParser) parseFactor() (float64, error) {
	val, err := p.parsePrimary()
	if err != nil {
		return 0, err
	}

	if p.peek() == '^' {
		p.get()
		rhs, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		val = math.Pow(val, rhs)
	}
	return val, nil
}

func (p *mathParser) parsePrimary() (float64, error) {
	if p.peek() == '+' {
		p.get()
		return p.parsePrimary()
	}
	if p.peek() == '-' {
		p.get()
		val, err := p.parsePrimary()
		return -val, err
	}
	if p.peek() == '(' {
		p.get()
		val, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.get()
		return val, nil
	}

	// Check function calls: sqrt, sin, cos, tan, log
	for _, fn := range []string{"sqrt", "sin", "cos", "tan", "log"} {
		if strings.HasPrefix(p.s[p.pos:], fn) {
			p.pos += len(fn)
			val, err := p.parsePrimary()
			if err != nil {
				return 0, err
			}
			switch fn {
			case "sqrt":
				if val < 0 {
					return 0, fmt.Errorf("negative square root")
				}
				return math.Sqrt(val), nil
			case "sin":
				return math.Sin(val), nil
			case "cos":
				return math.Cos(val), nil
			case "tan":
				return math.Tan(val), nil
			case "log":
				if val <= 0 {
					return 0, fmt.Errorf("non-positive log")
				}
				return math.Log10(val), nil
			}
		}
	}

	// Parse number
	start := p.pos
	hasDot := false
	for p.pos < len(p.s) && (unicode.IsDigit(rune(p.s[p.pos])) || p.s[p.pos] == '.') {
		if p.s[p.pos] == '.' {
			if hasDot {
				break
			}
			hasDot = true
		}
		p.pos++
	}

	if start == p.pos {
		return 0, fmt.Errorf("expected number")
	}

	val, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func checkUnitConversion(q string) *models.InstantAnswer {
	// Pattern: "[value] [unit1] to/in [unit2]" OR "[unit1] to/in [unit2]"
	regex := regexp.MustCompile(`(?i)^(?:([0-9.]+)\s*)?([a-zA-Z°/]+)\s+(?:to|in)\s+([a-zA-Z°/]+)$`)
	matches := regex.FindStringSubmatch(q)
	if len(matches) != 4 {
		return nil
	}

	val := 1.0
	if matches[1] != "" {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			val = v
		} else {
			return nil
		}
	}

	u1 := strings.ToLower(matches[2])
	u2 := strings.ToLower(matches[3])

	// Temperature conversions
	if (u1 == "c" || u1 == "celsius" || u1 == "°c") && (u2 == "f" || u2 == "fahrenheit" || u2 == "°f") {
		res := (val * 9.0 / 5.0) + 32.0
		return &models.InstantAnswer{
			Type:        "calculator",
			Title:       "Temperature Conversion",
			Value:       fmt.Sprintf("%.2f °F", res),
			Description: fmt.Sprintf("%.1f °C = %.2f °F", val, res),
		}
	}
	if (u1 == "f" || u1 == "fahrenheit" || u1 == "°f") && (u2 == "c" || u2 == "celsius" || u2 == "°c") {
		res := (val - 32.0) * 5.0 / 9.0
		return &models.InstantAnswer{
			Type:        "calculator",
			Title:       "Temperature Conversion",
			Value:       fmt.Sprintf("%.2f °C", res),
			Description: fmt.Sprintf("%.1f °F = %.2f °C", val, res),
		}
	}
	if (u1 == "k" || u1 == "kelvin") && (u2 == "c" || u2 == "celsius" || u2 == "°c") {
		res := val - 273.15
		return &models.InstantAnswer{
			Type:        "calculator",
			Title:       "Temperature Conversion",
			Value:       fmt.Sprintf("%.2f °C", res),
			Description: fmt.Sprintf("%.2f K = %.2f °C", val, res),
		}
	}
	if (u1 == "c" || u1 == "celsius" || u1 == "°c") && (u2 == "k" || u2 == "kelvin") {
		res := val + 273.15
		return &models.InstantAnswer{
			Type:        "calculator",
			Title:       "Temperature Conversion",
			Value:       fmt.Sprintf("%.2f K", res),
			Description: fmt.Sprintf("%.2f °C = %.2f K", val, res),
		}
	}

	// Length conversions
	lengthRates := map[string]float64{
		"m": 1.0, "meter": 1.0, "meters": 1.0,
		"km": 1000.0, "kilometer": 1000.0, "kilometers": 1000.0,
		"cm": 0.01, "centimeter": 0.01, "centimeters": 0.01,
		"mm": 0.001, "millimeter": 0.001, "millimeters": 0.001,
		"mi": 1609.344, "mile": 1609.344, "miles": 1609.344,
		"ft": 0.3048, "foot": 0.3048, "feet": 0.3048,
		"in": 0.0254, "inch": 0.0254, "inches": 0.0254,
		"yd": 0.9144, "yard": 0.9144, "yards": 0.9144,
	}

	if r1, ok1 := lengthRates[u1]; ok1 {
		if r2, ok2 := lengthRates[u2]; ok2 {
			inMeters := val * r1
			res := inMeters / r2
			return &models.InstantAnswer{
				Type:        "calculator",
				Title:       "Length Conversion",
				Value:       fmt.Sprintf("%s %s", formatFloat(res), u2),
				Description: fmt.Sprintf("%g %s = %s %s", val, u1, formatFloat(res), u2),
			}
		}
	}

	// Mass / Weight conversions
	weightRates := map[string]float64{
		"kg": 1.0, "kilogram": 1.0, "kilograms": 1.0,
		"g": 0.001, "gram": 0.001, "grams": 0.001,
		"mg": 0.000001, "milligram": 0.000001, "milligrams": 0.000001,
		"lb": 0.45359237, "lbs": 0.45359237, "pound": 0.45359237, "pounds": 0.45359237,
		"oz": 0.028349523, "ounce": 0.028349523, "ounces": 0.028349523,
		"ton": 907.18474, "tons": 907.18474, "tonne": 1000.0, "tonnes": 1000.0,
	}

	if r1, ok1 := weightRates[u1]; ok1 {
		if r2, ok2 := weightRates[u2]; ok2 {
			inKg := val * r1
			res := inKg / r2
			return &models.InstantAnswer{
				Type:        "calculator",
				Title:       "Weight Conversion",
				Value:       fmt.Sprintf("%s %s", formatFloat(res), u2),
				Description: fmt.Sprintf("%g %s = %s %s", val, u1, formatFloat(res), u2),
			}
		}
	}

	// Speed conversions
	speedRates := map[string]float64{
		"m/s": 1.0, "mps": 1.0,
		"km/h": 1.0 / 3.6, "kph": 1.0 / 3.6, "kmh": 1.0 / 3.6,
		"mph": 0.44704, "mi/h": 0.44704,
		"knot": 0.514444, "knots": 0.514444,
	}

	if r1, ok1 := speedRates[u1]; ok1 {
		if r2, ok2 := speedRates[u2]; ok2 {
			inMps := val * r1
			res := inMps / r2
			return &models.InstantAnswer{
				Type:        "calculator",
				Title:       "Speed Conversion",
				Value:       fmt.Sprintf("%s %s", formatFloat(res), u2),
				Description: fmt.Sprintf("%g %s = %s %s", val, u1, formatFloat(res), u2),
			}
		}
	}

	// Data storage conversions
	byteRates := map[string]float64{
		"b": 1.0, "byte": 1.0, "bytes": 1.0,
		"kb": 1024.0, "kilobyte": 1024.0, "kilobytes": 1024.0,
		"mb": 1024.0 * 1024.0, "megabyte": 1024.0 * 1024.0, "megabytes": 1024.0 * 1024.0,
		"gb": 1024.0 * 1024.0 * 1024.0, "gigabyte": 1024.0 * 1024.0 * 1024.0, "gigabytes": 1024.0 * 1024.0 * 1024.0,
		"tb": 1024.0 * 1024.0 * 1024.0 * 1024.0, "terabyte": 1024.0 * 1024.0 * 1024.0 * 1024.0, "terabytes": 1024.0 * 1024.0 * 1024.0 * 1024.0,
	}

	if r1, ok1 := byteRates[u1]; ok1 {
		if r2, ok2 := byteRates[u2]; ok2 {
			inBytes := val * r1
			res := inBytes / r2
			return &models.InstantAnswer{
				Type:        "calculator",
				Title:       "Data Storage Conversion",
				Value:       fmt.Sprintf("%s %s", formatFloat(res), strings.ToUpper(u2)),
				Description: fmt.Sprintf("%g %s = %s %s", val, strings.ToUpper(u1), formatFloat(res), strings.ToUpper(u2)),
			}
		}
	}

	return nil
}

func checkStatistics(query string) *models.InstantAnswer {
	qLower := strings.ToLower(strings.TrimSpace(query))
	prefixes := []string{"mean ", "avg ", "average ", "median ", "sum ", "min ", "max ", "prod ", "product ", "range ", "stddev ", "variance "}
	var op string
	var numsStr string
	for _, p := range prefixes {
		if strings.HasPrefix(qLower, p) {
			op = strings.TrimSpace(p)
			numsStr = qLower[len(p):]
			break
		}
	}
	if op == "" {
		return nil
	}

	// Parse comma or space separated numbers: e.g. "1, 5, 2, 8, -3" or "10 20 30 40"
	numsStr = strings.ReplaceAll(numsStr, ",", " ")
	fields := strings.Fields(numsStr)
	if len(fields) < 2 {
		return nil
	}

	nums := make([]float64, 0, len(fields))
	for _, f := range fields {
		val, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil
		}
		nums = append(nums, val)
	}

	var result float64
	var title string
	var desc string

	switch op {
	case "mean", "avg", "average":
		sum := 0.0
		for _, n := range nums {
			sum += n
		}
		result = sum / float64(len(nums))
		title = "Arithmetic Mean (Average)"
		desc = fmt.Sprintf("Mean of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "median":
		sorted := make([]float64, len(nums))
		copy(sorted, nums)
		sort.Float64s(sorted)
		n := len(sorted)
		if n%2 == 1 {
			result = sorted[n/2]
		} else {
			result = (sorted[n/2-1] + sorted[n/2]) / 2.0
		}
		title = "Median"
		desc = fmt.Sprintf("Median of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "sum":
		for _, n := range nums {
			result += n
		}
		title = "Sum"
		desc = fmt.Sprintf("Sum of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "min":
		result = nums[0]
		for _, n := range nums[1:] {
			if n < result {
				result = n
			}
		}
		title = "Minimum"
		desc = fmt.Sprintf("Minimum of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "max":
		result = nums[0]
		for _, n := range nums[1:] {
			if n > result {
				result = n
			}
		}
		title = "Maximum"
		desc = fmt.Sprintf("Maximum of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "prod", "product":
		result = 1.0
		for _, n := range nums {
			result *= n
		}
		title = "Product"
		desc = fmt.Sprintf("Product of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "range":
		minVal := nums[0]
		maxVal := nums[0]
		for _, n := range nums[1:] {
			if n < minVal {
				minVal = n
			}
			if n > maxVal {
				maxVal = n
			}
		}
		result = maxVal - minVal
		title = "Statistical Range (Max - Min)"
		desc = fmt.Sprintf("Range of [%s] = %s (Max: %s, Min: %s)", strings.Join(fields, ", "), formatFloat(result), formatFloat(maxVal), formatFloat(minVal))

	case "variance":
		sum := 0.0
		for _, n := range nums {
			sum += n
		}
		mean := sum / float64(len(nums))
		varSum := 0.0
		for _, n := range nums {
			diff := n - mean
			varSum += diff * diff
		}
		result = varSum / float64(len(nums))
		title = "Variance (σ²)"
		desc = fmt.Sprintf("Variance of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))

	case "stddev":
		sum := 0.0
		for _, n := range nums {
			sum += n
		}
		mean := sum / float64(len(nums))
		varSum := 0.0
		for _, n := range nums {
			diff := n - mean
			varSum += diff * diff
		}
		variance := varSum / float64(len(nums))
		result = math.Sqrt(variance)
		title = "Standard Deviation (σ)"
		desc = fmt.Sprintf("Standard Deviation of [%s] = %s", strings.Join(fields, ", "), formatFloat(result))
	}

	return &models.InstantAnswer{
		Type:        "calculator",
		Title:       title,
		Value:       formatFloat(result),
		Description: desc,
	}
}
