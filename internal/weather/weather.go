package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Location struct {
	Name        string  `json:"name"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Admin1      string  `json:"admin1,omitempty"` // State / Province
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Elevation   float64 `json:"elevation"`
	Timezone    string  `json:"timezone"`
}

type CurrentWeather struct {
	Temperature         float64 `json:"temperature"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	RelativeHumidity    int     `json:"relative_humidity"`
	WeatherCode         int     `json:"weather_code"`
	Condition           string  `json:"condition"`
	Emoji               string  `json:"emoji"`
	IsDay               bool    `json:"is_day"`
	WindSpeed           float64 `json:"wind_speed"`
	WindDirection       int     `json:"wind_direction"`
	SurfacePressure     float64 `json:"surface_pressure"`
	Precipitation       float64 `json:"precipitation"`
	UVIndex             float64 `json:"uv_index"`
}

type HourlyForecast struct {
	Time          string  `json:"time"`
	HourFormatted string  `json:"hour_formatted"`
	Temperature   float64 `json:"temperature"`
	PrecipProb    int     `json:"precip_probability"`
	WeatherCode   int     `json:"weather_code"`
	Condition     string  `json:"condition"`
	Emoji         string  `json:"emoji"`
	WindSpeed     float64 `json:"wind_speed"`
}

type DailyForecast struct {
	Date          string  `json:"date"`
	DayName       string  `json:"day_name"`
	WeatherCode   int     `json:"weather_code"`
	Condition     string  `json:"condition"`
	Emoji         string  `json:"emoji"`
	TempMax       float64 `json:"temp_max"`
	TempMin       float64 `json:"temp_min"`
	PrecipSum     float64 `json:"precipitation_sum"`
	PrecipProbMax int     `json:"precipitation_probability_max"`
	UVIndexMax    float64 `json:"uv_index_max"`
	Sunrise       string  `json:"sunrise"`
	Sunset        string  `json:"sunset"`
}

type WeatherReport struct {
	Location Location         `json:"location"`
	Current  CurrentWeather   `json:"current"`
	Hourly   []HourlyForecast `json:"hourly"`
	Daily    []DailyForecast  `json:"daily"`
	CachedAt time.Time        `json:"cached_at"`
}

type Service struct {
	client *http.Client
	mu     sync.RWMutex
	cache  map[string]*cacheEntry
}

type cacheEntry struct {
	data      *WeatherReport
	expiresAt time.Time
}

func NewService() *Service {
	return &Service{
		client: &http.Client{Timeout: 8 * time.Second},
		cache:  make(map[string]*cacheEntry),
	}
}

// GetWeatherByQuery searches for the city name, gets coordinates, then gets the weather
func (s *Service) GetWeatherByQuery(ctx context.Context, query string) (*WeatherReport, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = "Jakarta"
	}

	cacheKey := "q:" + strings.ToLower(q)
	s.mu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.data, nil
	}
	s.mu.RUnlock()

	loc, err := s.Geocode(ctx, q)
	if err != nil {
		return nil, err
	}

	report, err := s.GetWeatherByCoords(ctx, loc.Latitude, loc.Longitude, loc)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[cacheKey] = &cacheEntry{
		data:      report,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()

	return report, nil
}

// Geocode resolves a city name to coordinates using Open-Meteo Geocoding API
func (s *Service) Geocode(ctx context.Context, city string) (*Location, error) {
	apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json",
		url.QueryEscape(city))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo-Weather/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API error (HTTP %d)", resp.StatusCode)
	}

	var res struct {
		Results []struct {
			Name        string  `json:"name"`
			Country     string  `json:"country"`
			CountryCode string  `json:"country_code"`
			Admin1      string  `json:"admin1"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
			Elevation   float64 `json:"elevation"`
			Timezone    string  `json:"timezone"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to parse geocoding response: %w", err)
	}

	if len(res.Results) == 0 {
		return nil, fmt.Errorf("location '%s' not found", city)
	}

	first := res.Results[0]
	return &Location{
		Name:        first.Name,
		Country:     first.Country,
		CountryCode: first.CountryCode,
		Admin1:      first.Admin1,
		Latitude:    first.Latitude,
		Longitude:   first.Longitude,
		Elevation:   first.Elevation,
		Timezone:    first.Timezone,
	}, nil
}

// GetWeatherByCoords fetches forecast for given latitude and longitude
func (s *Service) GetWeatherByCoords(ctx context.Context, lat, lon float64, locHint *Location) (*WeatherReport, error) {
	coordKey := fmt.Sprintf("coord:%.3f:%.3f", lat, lon)
	s.mu.RLock()
	if entry, ok := s.cache[coordKey]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.data, nil
	}
	s.mu.RUnlock()

	tz := "auto"
	if locHint != nil && locHint.Timezone != "" {
		tz = locHint.Timezone
	}

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,weather_code,surface_pressure,wind_speed_10m,wind_direction_10m,uv_index"+
			"&hourly=temperature_2m,precipitation_probability,weather_code,wind_speed_10m"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset,precipitation_sum,precipitation_probability_max,uv_index_max"+
			"&timezone=%s",
		lat, lon, url.QueryEscape(tz),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", forecastURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SearXGo-Weather/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather forecast request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API error (HTTP %d)", resp.StatusCode)
	}

	var data struct {
		Timezone string `json:"timezone"`
		Current  struct {
			Temperature         float64 `json:"temperature_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			RelativeHumidity    int     `json:"relative_humidity_2m"`
			WeatherCode         int     `json:"weather_code"`
			IsDay               int     `json:"is_day"`
			WindSpeed           float64 `json:"wind_speed_10m"`
			WindDirection       int     `json:"wind_direction_10m"`
			SurfacePressure     float64 `json:"surface_pressure"`
			Precipitation       float64 `json:"precipitation"`
			UVIndex             float64 `json:"uv_index"`
		} `json:"current"`
		Hourly struct {
			Time             []string  `json:"time"`
			Temperature2m    []float64 `json:"temperature_2m"`
			PrecipProb       []int     `json:"precipitation_probability"`
			WeatherCode      []int     `json:"weather_code"`
			WindSpeed10m     []float64 `json:"wind_speed_10m"`
		} `json:"hourly"`
		Daily struct {
			Time             []string  `json:"time"`
			WeatherCode      []int     `json:"weather_code"`
			TempMax          []float64 `json:"temperature_2m_max"`
			TempMin          []float64 `json:"temperature_2m_min"`
			Sunrise          []string  `json:"sunrise"`
			Sunset           []string  `json:"sunset"`
			PrecipSum        []float64 `json:"precipitation_sum"`
			PrecipProbMax    []int     `json:"precipitation_probability_max"`
			UVIndexMax       []float64 `json:"uv_index_max"`
		} `json:"daily"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode forecast response: %w", err)
	}

	condition, emoji := CodeToCondition(data.Current.WeatherCode, data.Current.IsDay == 1)

	loc := Location{
		Name:      fmt.Sprintf("%.2f, %.2f", lat, lon),
		Latitude:  lat,
		Longitude: lon,
		Timezone:  data.Timezone,
	}
	if locHint != nil {
		loc = *locHint
		if loc.Timezone == "" {
			loc.Timezone = data.Timezone
		}
	}

	current := CurrentWeather{
		Temperature:         data.Current.Temperature,
		ApparentTemperature: data.Current.ApparentTemperature,
		RelativeHumidity:    data.Current.RelativeHumidity,
		WeatherCode:         data.Current.WeatherCode,
		Condition:           condition,
		Emoji:               emoji,
		IsDay:               data.Current.IsDay == 1,
		WindSpeed:           data.Current.WindSpeed,
		WindDirection:       data.Current.WindDirection,
		SurfacePressure:     data.Current.SurfacePressure,
		Precipitation:       data.Current.Precipitation,
		UVIndex:             data.Current.UVIndex,
	}

	// Next 24 hours of hourly forecast
	var hourly []HourlyForecast
	now := time.Now()
	count := 0
	for i, tStr := range data.Hourly.Time {
		if count >= 24 {
			break
		}
		t, err := time.Parse("2006-01-02T15:04", tStr)
		if err == nil && t.Before(now.Add(-1*time.Hour)) {
			continue // skip past hours
		}

		hCond, hEmoji := CodeToCondition(data.Hourly.WeatherCode[i], true)
		hTemp := 0.0
		if i < len(data.Hourly.Temperature2m) {
			hTemp = data.Hourly.Temperature2m[i]
		}
		hPrecip := 0
		if i < len(data.Hourly.PrecipProb) {
			hPrecip = data.Hourly.PrecipProb[i]
		}
		hWind := 0.0
		if i < len(data.Hourly.WindSpeed10m) {
			hWind = data.Hourly.WindSpeed10m[i]
		}

		hourDisplay := tStr
		if err == nil {
			hourDisplay = t.Format("15:04")
		} else if len(tStr) >= 16 {
			hourDisplay = tStr[11:16]
		}

		hourly = append(hourly, HourlyForecast{
			Time:          tStr,
			HourFormatted: hourDisplay,
			Temperature:   hTemp,
			PrecipProb:    hPrecip,
			WeatherCode:   data.Hourly.WeatherCode[i],
			Condition:     hCond,
			Emoji:         hEmoji,
			WindSpeed:     hWind,
		})
		count++
	}

	// 7 Days Daily Forecast
	var daily []DailyForecast
	for i, dStr := range data.Daily.Time {
		dCond, dEmoji := CodeToCondition(data.Daily.WeatherCode[i], true)
		dayName := dStr
		if parsedDate, err := time.Parse("2006-01-02", dStr); err == nil {
			if i == 0 {
				dayName = "Today"
			} else {
				dayName = parsedDate.Format("Mon, Jan 2")
			}
		}

		tMax := 0.0
		if i < len(data.Daily.TempMax) {
			tMax = data.Daily.TempMax[i]
		}
		tMin := 0.0
		if i < len(data.Daily.TempMin) {
			tMin = data.Daily.TempMin[i]
		}
		pSum := 0.0
		if i < len(data.Daily.PrecipSum) {
			pSum = data.Daily.PrecipSum[i]
		}
		pProb := 0
		if i < len(data.Daily.PrecipProbMax) {
			pProb = data.Daily.PrecipProbMax[i]
		}
		uvMax := 0.0
		if i < len(data.Daily.UVIndexMax) {
			uvMax = data.Daily.UVIndexMax[i]
		}
		sunrise := ""
		if i < len(data.Daily.Sunrise) && len(data.Daily.Sunrise[i]) >= 16 {
			sunrise = data.Daily.Sunrise[i][11:16]
		}
		sunset := ""
		if i < len(data.Daily.Sunset) && len(data.Daily.Sunset[i]) >= 16 {
			sunset = data.Daily.Sunset[i][11:16]
		}

		daily = append(daily, DailyForecast{
			Date:          dStr,
			DayName:       dayName,
			WeatherCode:   data.Daily.WeatherCode[i],
			Condition:     dCond,
			Emoji:         dEmoji,
			TempMax:       tMax,
			TempMin:       tMin,
			PrecipSum:     pSum,
			PrecipProbMax: pProb,
			UVIndexMax:    uvMax,
			Sunrise:       sunrise,
			Sunset:        sunset,
		})
	}

	report := &WeatherReport{
		Location: loc,
		Current:  current,
		Hourly:   hourly,
		Daily:    daily,
		CachedAt: time.Now(),
	}

	s.mu.Lock()
	s.cache[coordKey] = &cacheEntry{
		data:      report,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()

	return report, nil
}

// CodeToCondition maps WMO Weather interpretation codes (WW) to human condition and emoji
func CodeToCondition(code int, isDay bool) (string, string) {
	switch code {
	case 0:
		if isDay {
			return "Clear sky", "☀️"
		}
		return "Clear sky", "🌙"
	case 1:
		if isDay {
			return "Mainly clear", "🌤️"
		}
		return "Mainly clear", "🌑"
	case 2:
		return "Partly cloudy", "⛅"
	case 3:
		return "Overcast", "☁️"
	case 45:
		return "Foggy", "🌫️"
	case 48:
		return "Depositing rime fog", "🌫️"
	case 51:
		return "Light drizzle", "🌦️"
	case 53:
		return "Moderate drizzle", "🌦️"
	case 55:
		return "Dense drizzle", "🌧️"
	case 56, 57:
		return "Freezing drizzle", "🌨️"
	case 61:
		return "Slight rain", "🌦️"
	case 63:
		return "Moderate rain", "🌧️"
	case 65:
		return "Heavy rain", "🌧️"
	case 66, 67:
		return "Freezing rain", "🌨️"
	case 71:
		return "Slight snow fall", "🌨️"
	case 73:
		return "Moderate snow fall", "❄️"
	case 75:
		return "Heavy snow fall", "❄️"
	case 77:
		return "Snow grains", "❄️"
	case 80:
		return "Slight rain showers", "🌦️"
	case 81:
		return "Moderate rain showers", "🌧️"
	case 82:
		return "Violent rain showers", "⛈️"
	case 85, 86:
		return "Snow showers", "🌨️"
	case 95:
		return "Thunderstorm", "⛈️"
	case 96, 99:
		return "Thunderstorm with hail", "⛈️"
	default:
		return "Fair", "🌤️"
	}
}

// ParseCoordinates parses comma-separated lat,lon string if user searches coordinates directly
func ParseCoordinates(s string) (float64, float64, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}
