package weather

import (
	"context"
	"testing"
	"time"
)

func TestCodeToCondition(t *testing.T) {
	cond, emoji := CodeToCondition(0, true)
	if cond != "Clear sky" || emoji != "☀️" {
		t.Errorf("expected Clear sky ☀️, got %s %s", cond, emoji)
	}

	cond, emoji = CodeToCondition(0, false)
	if cond != "Clear sky" || emoji != "🌙" {
		t.Errorf("expected Clear sky 🌙, got %s %s", cond, emoji)
	}

	cond, emoji = CodeToCondition(95, true)
	if cond != "Thunderstorm" || emoji != "⛈️" {
		t.Errorf("expected Thunderstorm ⛈️, got %s %s", cond, emoji)
	}
}

func TestParseCoordinates(t *testing.T) {
	lat, lon, ok := ParseCoordinates("-6.2088, 106.8456")
	if !ok {
		t.Fatalf("expected coordinates to be valid")
	}
	if lat < -6.21 || lat > -6.20 || lon < 106.84 || lon > 106.85 {
		t.Errorf("unexpected coordinates: %f, %f", lat, lon)
	}

	_, _, ok = ParseCoordinates("invalid,string")
	if ok {
		t.Errorf("expected invalid coordinates to return false")
	}
}

func TestWeatherServiceCache(t *testing.T) {
	svc := NewService()
	report := &WeatherReport{
		Location: Location{Name: "Test City", Latitude: 10, Longitude: 20},
		Current:  CurrentWeather{Temperature: 25.5, Condition: "Fair"},
		CachedAt: time.Now(),
	}

	svc.mu.Lock()
	svc.cache["q:test city"] = &cacheEntry{
		data:      report,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	svc.mu.Unlock()

	cached, err := svc.GetWeatherByQuery(context.Background(), "test city")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cached.Location.Name != "Test City" {
		t.Errorf("expected Test City, got %s", cached.Location.Name)
	}
}
