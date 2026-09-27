package qr

import (
	"fmt"
	"strings"
)

type WiFiConfig struct {
	SSID       string `json:"ssid"`
	Password   string `json:"password"`
	Encryption string `json:"encryption"` // "WPA", "WEP", "nopass"
	Hidden     bool   `json:"hidden"`
}

// FormatWiFiPayload returns the standard Wi-Fi QR code payload
func FormatWiFiPayload(cfg WiFiConfig) string {
	enc := strings.ToUpper(cfg.Encryption)
	if enc == "" || enc == "WPA2" || enc == "WPA3" {
		enc = "WPA"
	} else if enc == "NONE" || enc == "OPEN" {
		enc = "nopass"
	}

	h := "false"
	if cfg.Hidden {
		h = "true"
	}

	return fmt.Sprintf("WIFI:S:%s;T:%s;P:%s;H:%s;;",
		escapeWiFi(cfg.SSID),
		enc,
		escapeWiFi(cfg.Password),
		h,
	)
}

func escapeWiFi(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, ":", "\\:")
	return s
}
