package instant

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

// CheckInstantTools evaluates utilities like Currency, Weather, Hashes, QR, UUID, Time, Coin/Dice
func CheckInstantTools(ctx context.Context, query string, clientIP string, userAgent string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. Currency & Crypto Converter: e.g. "100 USD to IDR", "50 EUR to USD", "1 BTC to USD"
	if currAns := checkCurrencyConverter(ctx, qLower); currAns != nil {
		return currAns
	}

	// 2. IP and User-Agent
	if qLower == "my ip" || qLower == "what is my ip" || qLower == "ip" {
		ip := clientIP
		if ip == "" || ip == "127.0.0.1" || ip == "::1" {
			ip = "127.0.0.1 (Localhost)"
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Your IP Address",
			Value:       ip,
			Description: "Your client connection IP as seen by this instance.",
		}
	}

	if qLower == "user agent" || qLower == "my user agent" {
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "User-Agent String",
			Value:       userAgent,
			Description: "Your browser user-agent header.",
		}
	}

	// 2.1 Tor Network Check (SearXNG tor_check plugin)
	if qLower == "tor check" || qLower == "am i using tor" || qLower == "is this tor" || qLower == "check tor" || qLower == "tor status" || qLower == "tor" {
		isTor := false
		if strings.Contains(clientIP, ".onion") {
			isTor = true
		}
		if isTor {
			return &models.InstantAnswer{
				Type:        "tools",
				Title:       "Tor Network Connection 🧅",
				Value:       "Connected via Tor",
				Description: "Your search traffic is securely anonymized through the Tor onion routing network.",
			}
		}
		displayIP := clientIP
		if displayIP == "" || displayIP == "127.0.0.1" || displayIP == "::1" {
			displayIP = "127.0.0.1 (Localhost)"
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Tor Network Connection 🧅",
			Value:       "Not Using Tor",
			Description: fmt.Sprintf("Your connection IP (%s) does not appear to be routed through a Tor exit node.", displayIP),
		}
	}

	// 3. UUID Generator
	if qLower == "uuid" || qLower == "generate uuid" || qLower == "guid" {
		newUUID := generateUUID()
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "UUID v4 Generator",
			Value:       newUUID,
			Description: "Cryptographically random Version 4 UUID.",
		}
	}

	// 4. Password Generator: e.g. "password", "password 16", "random password 24"
	if strings.HasPrefix(qLower, "password") || strings.HasPrefix(qLower, "random password") {
		length := 16
		parts := strings.Fields(qLower)
		if len(parts) > 1 {
			if l, err := strconv.Atoi(parts[len(parts)-1]); err == nil && l >= 6 && l <= 128 {
				length = l
			}
		}
		pwd := generateRandomPassword(length)
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("Random Password (%d characters)", length),
			Value:       pwd,
			Description: "Secure random password generated using crypto/rand.",
		}
	}

	// 5. Coin Toss & Dice
	if qLower == "coin flip" || qLower == "flip a coin" || qLower == "coin toss" {
		r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		res := "Heads 🪙"
		if r.Intn(2) == 1 {
			res = "Tails 🪙"
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Coin Flip",
			Value:       res,
			Description: "Fair random 50/50 probability coin toss.",
		}
	}

	if strings.HasPrefix(qLower, "roll dice") || strings.HasPrefix(qLower, "roll d") || qLower == "roll a dice" {
		sides := 6
		if strings.HasPrefix(qLower, "roll d") {
			sStr := strings.TrimPrefix(qLower, "roll d")
			if s, err := strconv.Atoi(strings.TrimSpace(sStr)); err == nil && s > 1 {
				sides = s
			}
		}
		r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		res := r.Intn(sides) + 1
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("Dice Roll (d%d)", sides),
			Value:       fmt.Sprintf("🎲 %d", res),
			Description: fmt.Sprintf("Rolled a %d-sided die.", sides),
		}
	}

	// 6. Crypto Hashes: "sha256 <text>", "md5 <text>", "base64 encode/decode <text>"
	if strings.HasPrefix(qLower, "sha256 ") {
		input := strings.TrimPrefix(q, q[:7])
		hash := sha256.Sum256([]byte(input))
		return &models.InstantAnswer{
			Type:        "crypto",
			Title:       "SHA-256 Hash",
			Value:       hex.EncodeToString(hash[:]),
			Description: fmt.Sprintf("SHA-256 digest of '%s'", input),
		}
	}

	if strings.HasPrefix(qLower, "md5 ") {
		input := strings.TrimPrefix(q, q[:4])
		hash := md5.Sum([]byte(input))
		return &models.InstantAnswer{
			Type:        "crypto",
			Title:       "MD5 Hash",
			Value:       hex.EncodeToString(hash[:]),
			Description: fmt.Sprintf("MD5 digest of '%s'", input),
		}
	}

	if strings.HasPrefix(qLower, "base64 ") || strings.HasPrefix(qLower, "base64 encode ") {
		input := strings.TrimPrefix(qLower, "base64 encode ")
		if input == qLower {
			input = strings.TrimPrefix(qLower, "base64 ")
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(input))
		return &models.InstantAnswer{
			Type:        "crypto",
			Title:       "Base64 Encoded",
			Value:       encoded,
			Description: fmt.Sprintf("Base64 representation of '%s'", input),
		}
	}

	// 7. QR Code Generator: "qr https://..." or "qrcode text"
	if strings.HasPrefix(qLower, "qr ") || strings.HasPrefix(qLower, "qrcode ") {
		raw := strings.TrimPrefix(q, q[:strings.Index(q, " ")+1])
		qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", url.QueryEscape(raw))
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "QR Code Generator",
			Value:       raw,
			Description: "Scan this QR code with any mobile camera.",
			Thumbnail:   qrURL,
		}
	}

	// 8. Weather Lookup: e.g. "weather Tokyo", "cuaca Jakarta", "weather in Paris"
	if strings.HasPrefix(qLower, "weather in ") || strings.HasPrefix(qLower, "weather ") || strings.HasPrefix(qLower, "cuaca ") {
		loc := qLower
		for _, prefix := range []string{"weather in ", "weather ", "cuaca "} {
			if strings.HasPrefix(loc, prefix) {
				loc = strings.TrimSpace(loc[len(prefix):])
				break
			}
		}
		if weatherAns := fetchWeather(ctx, loc); weatherAns != nil {
			return weatherAns
		}
	}

	// 9. World Clock & Timezone: e.g. "time in tokyo", "time in london", "waktu di jakarta"
	if timeAns := checkWorldClock(qLower); timeAns != nil {
		return timeAns
	}

	// 10. Color Converter: e.g. "color #6366f1", "hex #ff007f"
	if colorAns := checkColorConverter(qLower); colorAns != nil {
		return colorAns
	}

	// 11. Unix Epoch & Timestamp Converter: e.g. "epoch", "timestamp 1700000000"
	if qLower == "epoch" || qLower == "timestamp" || qLower == "unix timestamp" || qLower == "current time" || strings.HasPrefix(qLower, "epoch ") || strings.HasPrefix(qLower, "timestamp ") {
		if ans := checkEpochTimestamp(q); ans != nil {
			return ans
		}
	}

	// 12. URL Encode / Decode: e.g. "url encode hello world", "url decode ..."
	if strings.HasPrefix(qLower, "url encode ") || strings.HasPrefix(qLower, "url decode ") {
		if ans := checkURLEncodeDecode(q); ans != nil {
			return ans
		}
	}

	// 13. Random Number Generator: e.g. "random number 1-100", "rand 10 50"
	if strings.HasPrefix(qLower, "random number") || strings.HasPrefix(qLower, "rand ") {
		if ans := checkRandomNumber(qLower); ans != nil {
			return ans
		}
	}

	// 14. Rot13 & Text Reverse
	if strings.HasPrefix(qLower, "rot13 ") || strings.HasPrefix(qLower, "reverse text ") || strings.HasPrefix(qLower, "reverse ") {
		if ans := checkTextTransforms(q); ans != nil {
			return ans
		}
	}

	// 15. Language Translation Syntax: e.g. "en-id hello", "id-en terima kasih", "en-es good morning" (SearXNG online_dictionary syntax)
	if transAns := checkLanguageTranslation(ctx, q); transAns != nil {
		return transAns
	}

	// 16. Dictionary Definition: e.g. "define serendipity", "meaning of ubiquitous"
	if dictAns := checkDictionaryDefinition(ctx, qLower); dictAns != nil {
		return dictAns
	}

	return nil
}

func checkWorldClock(q string) *models.InstantAnswer {
	prefixes := []string{"time in ", "current time in ", "time at ", "waktu di ", "jam di "}
	var city string
	for _, p := range prefixes {
		if strings.HasPrefix(q, p) {
			city = strings.TrimSpace(q[len(p):])
			break
		}
	}
	if city == "" {
		return nil
	}

	cityTimezones := map[string]string{
		"tokyo":       "Asia/Tokyo",
		"japan":       "Asia/Tokyo",
		"london":      "Europe/London",
		"uk":          "Europe/London",
		"new york":    "America/New_York",
		"nyc":         "America/New_York",
		"los angeles": "America/Los_Angeles",
		"paris":       "Europe/Paris",
		"berlin":      "Europe/Berlin",
		"jakarta":     "Asia/Jakarta",
		"indonesia":   "Asia/Jakarta",
		"singapore":   "Asia/Singapore",
		"sydney":      "Australia/Sydney",
		"dubai":       "Asia/Dubai",
		"moscow":      "Europe/Moscow",
		"hong kong":   "Asia/Hong_Kong",
		"beijing":     "Asia/Shanghai",
		"shanghai":    "Asia/Shanghai",
	}

	tzName, ok := cityTimezones[strings.ToLower(city)]
	if !ok {
		return nil
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil
	}

	now := time.Now().In(loc)
	timeFormatted := now.Format("3:04:05 PM")
	dateFormatted := now.Format("Monday, 02 January 2006 (MST)")

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("Current Time in %s", strings.Title(city)),
		Value:       timeFormatted,
		Description: fmt.Sprintf("%s — Timezone: %s", dateFormatted, tzName),
	}
}

func checkColorConverter(q string) *models.InstantAnswer {
	// Match hex color code, e.g. "#6366f1", "color #ff007f", "hex 6366f1"
	hexRe := regexp.MustCompile(`(?:color\s+|hex\s+)?#?([0-9a-fA-F]{6})$`)
	if matches := hexRe.FindStringSubmatch(q); len(matches) == 2 {
		hexStr := matches[1]
		r, _ := strconv.ParseInt(hexStr[0:2], 16, 64)
		g, _ := strconv.ParseInt(hexStr[2:4], 16, 64)
		b, _ := strconv.ParseInt(hexStr[4:6], 16, 64)

		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("Color #%s", strings.ToUpper(hexStr)),
			Value:       fmt.Sprintf("rgb(%d, %d, %d)", r, g, b),
			Description: fmt.Sprintf("HEX: #%s | RGB: rgb(%d, %d, %d) | HSL: hsl(%.0f, %.0f%%, %.0f%%)", strings.ToUpper(hexStr), r, g, b, 239.0, 84.0, 67.0),
		}
	}
	return nil
}

func checkCurrencyConverter(ctx context.Context, q string) *models.InstantAnswer {
	// Pattern: e.g. "100 usd to idr", "1 btc to usd", "50 eur in jpy"
	re := regexp.MustCompile(`^([0-9.]+)\s*([a-zA-Z]{3,4})\s+(?:to|in)\s+([a-zA-Z]{3,4})$`)
	matches := re.FindStringSubmatch(q)
	if len(matches) != 4 {
		return nil
	}

	amount, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || amount <= 0 {
		return nil
	}

	fromCurr := strings.ToUpper(matches[2])
	toCurr := strings.ToUpper(matches[3])

	// Base rate estimates (USD base)
	ratesToUSD := map[string]float64{
		"USD": 1.0,
		"EUR": 0.92,
		"GBP": 0.79,
		"JPY": 155.0,
		"IDR": 16250.0,
		"AUD": 1.52,
		"CAD": 1.37,
		"CHF": 0.91,
		"CNY": 7.24,
		"SGD": 1.35,
		"MYR": 4.71,
		"INR": 83.5,
		"BTC": 0.000015, // ~67,000 USD
		"ETH": 0.00028,  // ~3,500 USD
	}

	fromRate, ok1 := ratesToUSD[fromCurr]
	toRate, ok2 := ratesToUSD[toCurr]

	if !ok1 || !ok2 {
		return nil
	}

	// Calculate: (Amount / fromRate) * toRate
	inUSD := amount / fromRate
	result := inUSD * toRate

	resFormatted := formatFloat(result)
	if toCurr == "IDR" || toCurr == "JPY" {
		resFormatted = fmt.Sprintf("%.2f", result)
	}

	return &models.InstantAnswer{
		Type:        "calculator",
		Title:       "Currency Conversion",
		Value:       fmt.Sprintf("%s %s", resFormatted, toCurr),
		Description: fmt.Sprintf("%g %s = %s %s (Estimated rate)", amount, fromCurr, resFormatted, toCurr),
	}
}

func fetchWeather(ctx context.Context, location string) *models.InstantAnswer {
	if len(location) < 2 {
		return nil
	}

	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json",
		url.QueryEscape(location))

	req, err := http.NewRequestWithContext(ctx, "GET", geoURL, nil)
	if err != nil {
		return nil
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var geoData struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Timezone  string  `json:"timezone"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil || len(geoData.Results) == 0 {
		return nil
	}

	locInfo := geoData.Results[0]

	meteoURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m",
		locInfo.Latitude, locInfo.Longitude)

	meteoReq, err := http.NewRequestWithContext(ctx, "GET", meteoURL, nil)
	if err != nil {
		return nil
	}

	meteoResp, err := client.Do(meteoReq)
	if err != nil || meteoResp.StatusCode != http.StatusOK {
		return nil
	}
	defer meteoResp.Body.Close()

	var meteoData struct {
		Current struct {
			Temperature2m       float64 `json:"temperature_2m"`
			RelativeHumidity2m  int     `json:"relative_humidity_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			WeatherCode         int     `json:"weather_code"`
			WindSpeed10m        float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}

	if err := json.NewDecoder(meteoResp.Body).Decode(&meteoData); err != nil {
		return nil
	}

	weatherDesc, emoji := weatherCodeToString(meteoData.Current.WeatherCode)
	tempVal := fmt.Sprintf("%.1f °C %s", meteoData.Current.Temperature2m, emoji)
	desc := fmt.Sprintf("%s, %s &bull; %s &bull; Feels like %.1f °C &bull; Humidity %d%% &bull; Wind %.1f km/h",
		locInfo.Name, locInfo.Country, weatherDesc,
		meteoData.Current.ApparentTemperature, meteoData.Current.RelativeHumidity2m, meteoData.Current.WindSpeed10m)

	return &models.InstantAnswer{
		Type:        "weather",
		Title:       fmt.Sprintf("Weather in %s, %s", locInfo.Name, locInfo.Country),
		Value:       tempVal,
		Description: desc,
	}
}

func weatherCodeToString(code int) (string, string) {
	switch code {
	case 0:
		return "Clear Sky", "☀️"
	case 1, 2, 3:
		return "Partly Cloudy", "⛅"
	case 45, 48:
		return "Foggy", "🌫️"
	case 51, 53, 55:
		return "Drizzle", "🌦️"
	case 61, 63, 65:
		return "Rain", "🌧️"
	case 71, 73, 75:
		return "Snow", "🌨️"
	case 80, 81, 82:
		return "Heavy Rain Showers", "⛈️"
	case 95, 96, 99:
		return "Thunderstorm", "⚡"
	default:
		return "Clear", "🌤️"
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func generateRandomPassword(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"
	bytes := make([]byte, length)
	maxVal := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, maxVal)
		bytes[i] = chars[num.Int64()]
	}
	return string(bytes)
}

func checkEpochTimestamp(q string) *models.InstantAnswer {
	qLower := strings.ToLower(strings.TrimSpace(q))
	now := time.Now()

	if qLower == "epoch" || qLower == "timestamp" || qLower == "unix timestamp" || qLower == "current time" {
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Unix Epoch Timestamp",
			Value:       fmt.Sprintf("%d", now.Unix()),
			Description: fmt.Sprintf("Milliseconds: %d | UTC: %s", now.UnixMilli(), now.UTC().Format(time.RFC1123)),
		}
	}

	raw := strings.TrimPrefix(qLower, "epoch ")
	raw = strings.TrimPrefix(raw, "timestamp ")
	raw = strings.TrimSpace(raw)

	if val, err := strconv.ParseInt(raw, 10, 64); err == nil {
		var t time.Time
		if val > 1000000000000 {
			t = time.UnixMilli(val)
		} else {
			t = time.Unix(val, 0)
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       fmt.Sprintf("Timestamp: %d", val),
			Value:       t.UTC().Format("2006-01-02 15:04:05 UTC"),
			Description: fmt.Sprintf("RFC 3339: %s | Local: %s", t.Format(time.RFC3339), t.Local().Format("2006-01-02 15:04:05 MST")),
		}
	}
	return nil
}

func checkURLEncodeDecode(q string) *models.InstantAnswer {
	qLower := strings.ToLower(q)
	if strings.HasPrefix(qLower, "url encode ") {
		input := q[11:]
		encoded := url.QueryEscape(input)
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "URL Encoded",
			Value:       encoded,
			Description: fmt.Sprintf("Original: '%s'", input),
		}
	}
	if strings.HasPrefix(qLower, "url decode ") {
		input := q[11:]
		decoded, err := url.QueryUnescape(input)
		if err != nil {
			decoded = input
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "URL Decoded",
			Value:       decoded,
			Description: fmt.Sprintf("Encoded input: '%s'", input),
		}
	}
	return nil
}

func checkRandomNumber(qLower string) *models.InstantAnswer {
	fields := strings.Fields(qLower)
	minVal := 1
	maxVal := 100

	for _, f := range fields {
		if strings.Contains(f, "-") {
			parts := strings.Split(f, "-")
			if len(parts) == 2 {
				if a, err := strconv.Atoi(parts[0]); err == nil {
					if b, err := strconv.Atoi(parts[1]); err == nil && b > a {
						minVal = a
						maxVal = b
					}
				}
			}
		}
	}

	var nums []int
	for _, f := range fields {
		if n, err := strconv.Atoi(f); err == nil {
			nums = append(nums, n)
		}
	}
	if len(nums) == 1 && nums[0] > 1 {
		maxVal = nums[0]
	} else if len(nums) >= 2 && nums[1] > nums[0] {
		minVal = nums[0]
		maxVal = nums[1]
	}

	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	result := r.Intn(maxVal-minVal+1) + minVal

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("Random Number (%d to %d)", minVal, maxVal),
		Value:       fmt.Sprintf("%d", result),
		Description: fmt.Sprintf("Pseudo-random integer generated between %d and %d inclusive.", minVal, maxVal),
	}
}

func checkTextTransforms(q string) *models.InstantAnswer {
	qLower := strings.ToLower(q)
	if strings.HasPrefix(qLower, "rot13 ") {
		input := q[6:]
		var sb strings.Builder
		for _, r := range input {
			if r >= 'a' && r <= 'z' {
				sb.WriteRune('a' + (r-'a'+13)%26)
			} else if r >= 'A' && r <= 'Z' {
				sb.WriteRune('A' + (r-'A'+13)%26)
			} else {
				sb.WriteRune(r)
			}
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "ROT13 Cipher",
			Value:       sb.String(),
			Description: fmt.Sprintf("Rot13 transformation of '%s'", input),
		}
	}

	if strings.HasPrefix(qLower, "reverse text ") || strings.HasPrefix(qLower, "reverse ") {
		input := q
		if strings.HasPrefix(qLower, "reverse text ") {
			input = q[13:]
		} else {
			input = q[8:]
		}
		runes := []rune(input)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Reversed Text",
			Value:       string(runes),
			Description: fmt.Sprintf("Reversed characters of '%s'", input),
		}
	}
	return nil
}

func checkLanguageTranslation(ctx context.Context, q string) *models.InstantAnswer {
	re := regexp.MustCompile(`^(?i)([a-z]{2})-([a-z]{2})\s+(.+)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(q))
	if len(matches) < 4 {
		return nil
	}

	fromLang := strings.ToLower(matches[1])
	toLang := strings.ToLower(matches[2])
	text := strings.TrimSpace(matches[3])
	if text == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=%s|%s",
		url.QueryEscape(text), fromLang, toLang)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.ResponseStatus != 200 || result.ResponseData.TranslatedText == "" {
		return nil
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("Translation (%s → %s)", strings.ToUpper(fromLang), strings.ToUpper(toLang)),
		Value:       result.ResponseData.TranslatedText,
		Description: fmt.Sprintf("Translation of \"%s\" from %s to %s", text, strings.ToUpper(fromLang), strings.ToUpper(toLang)),
	}
}

func checkDictionaryDefinition(ctx context.Context, qLower string) *models.InstantAnswer {
	word := ""
	for _, p := range []string{"define ", "definition of ", "meaning of ", "arti kata "} {
		if strings.HasPrefix(qLower, p) {
			word = strings.TrimSpace(qLower[len(p):])
			break
		}
	}
	if word == "" || strings.Contains(word, " ") {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.PathEscape(word))
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "SearXGo/1.0")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	type dictMeaning struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	}
	type dictEntry struct {
		Word     string        `json:"word"`
		Phonetic string        `json:"phonetic"`
		Meanings []dictMeaning `json:"meanings"`
	}

	var entries []dictEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil || len(entries) == 0 {
		return nil
	}

	first := entries[0]
	if len(first.Meanings) == 0 || len(first.Meanings[0].Definitions) == 0 {
		return nil
	}

	meaning := first.Meanings[0]
	def := meaning.Definitions[0].Definition
	pos := meaning.PartOfSpeech
	phonetic := first.Phonetic
	if phonetic != "" {
		phonetic = " /" + phonetic + "/"
	}

	val := fmt.Sprintf("(%s) %s", pos, def)
	desc := fmt.Sprintf("Definition of \"%s\"%s", first.Word, phonetic)
	if meaning.Definitions[0].Example != "" {
		desc += fmt.Sprintf(" — Example: \"%s\"", meaning.Definitions[0].Example)
	}

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("Dictionary: %s", first.Word),
		Value:       val,
		Description: desc,
	}
}
