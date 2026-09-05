package plugins

import (
	"strings"

	"searxgo/internal/models"
)

// CheckTorAndSelfInfo checks if request is using Tor and generates privacy diagnostics
func CheckTorAndSelfInfo(clientIP string, headers map[string]string) *models.InstantAnswer {
	isTor := false
	if headers != nil {
		if headers["X-Tor-Exit"] == "1" || headers["X-Tor"] == "true" {
			isTor = true
		}
	}

	if strings.Contains(clientIP, ".onion") {
		isTor = true
	}

	if isTor {
		return &models.InstantAnswer{
			Type:        "tools",
			Title:       "Tor Network Detected 🧅",
			Value:       "Connected via Tor",
			Description: "Your search traffic is anonymized through the Tor onion routing network.",
		}
	}

	return nil
}
