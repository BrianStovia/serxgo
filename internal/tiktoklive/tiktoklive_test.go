package tiktoklive

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
)

func TestCleanUsername(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@jkt48.official", "jkt48.official"},
		{"jkt48.official", "jkt48.official"},
		{"https://www.tiktok.com/@mrbeast/live", "mrbeast"},
		{"https://tiktok.com/@scout2015", "scout2015"},
		{"7345678901234567890", "7345678901234567890"},
	}

	for _, tc := range tests {
		got := CleanUsername(tc.input)
		if got != tc.expected {
			t.Errorf("CleanUsername(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSolveSlardarWAF(t *testing.T) {
	// Synthesize a valid challenge
	aSeed := "test_seed_12345"
	targetI := 42
	aBytes := []byte(aSeed)
	h := sha256.Sum256(append(aBytes, []byte(strconv.Itoa(targetI))...))

	challengeObj := map[string]interface{}{
		"v": map[string]interface{}{
			"a": base64.StdEncoding.EncodeToString(aBytes),
			"b": 1720000000,
			"c": base64.StdEncoding.EncodeToString(h[:]),
		},
		"s": "test_signature",
	}

	raw, err := json.Marshal(challengeObj)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	csB64 := base64.StdEncoding.EncodeToString(raw)

	solvedB64, err := SolveSlardarWAF(csB64)
	if err != nil {
		t.Fatalf("SolveSlardarWAF failed: %v", err)
	}

	solvedBytes, err := base64.StdEncoding.DecodeString(solvedB64)
	if err != nil {
		t.Fatalf("Failed to decode solved string: %v", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(solvedBytes, &res); err != nil {
		t.Fatalf("Unmarshal solved error: %v", err)
	}

	vMap := res["v"].(map[string]interface{})
	dVal := int(vMap["d"].(float64))
	if dVal != targetI {
		t.Errorf("Expected solution %d, got %d", targetI, dVal)
	}
}

func TestExtractLiveRoom(t *testing.T) {
	svc := NewService()
	info, err := svc.Extract(context.Background(), "mrbeast")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if info == nil {
		t.Fatalf("Extract returned nil info")
	}
	t.Logf("mrbeast: IsLive=%v, Nickname=%q, Avatar=%q", info.IsLive, info.Nickname, info.Avatar)
	if info.Username != "mrbeast" {
		t.Errorf("Expected username mrbeast, got %q", info.Username)
	}

	liveInfo, err := svc.Extract(context.Background(), "skynews")
	if err == nil && liveInfo != nil {
		t.Logf("skynews: IsLive=%v, Title=%q, Viewers=%d, Qualities=%d, StreamURL=%q", 
			liveInfo.IsLive, liveInfo.Title, liveInfo.ViewerCount, len(liveInfo.Qualities), liveInfo.StreamURL)
	}
}
