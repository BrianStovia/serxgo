package archive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SnapshotItem struct {
	Timestamp   string `json:"timestamp"`
	DateStr     string `json:"date_str"`
	OriginalURL string `json:"original_url"`
	MimeType    string `json:"mime_type"`
	StatusCode  string `json:"status_code"`
	ArchiveURL  string `json:"archive_url"`
}

type ArchiveReport struct {
	QueryURL       string         `json:"query_url"`
	CleanDomain    string         `json:"clean_domain"`
	TotalSnapshots int            `json:"total_snapshots"`
	FirstSeen      string         `json:"first_seen,omitempty"`
	LastSeen       string         `json:"last_seen,omitempty"`
	SaveURL        string         `json:"save_url"`
	Snapshots      []SnapshotItem `json:"snapshots"`
	Error          string         `json:"error,omitempty"`
}

type ArchiveClient struct {
	client *http.Client
}

func NewArchiveClient() *ArchiveClient {
	return &ArchiveClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func formatTimestamp(ts string) string {
	if len(ts) >= 14 {
		// YYYYMMDDhhmmss
		t, err := time.Parse("20060102150405", ts)
		if err == nil {
			return t.Format("02 Jan 2006, 15:04 UTC")
		}
	} else if len(ts) >= 8 {
		t, err := time.Parse("20060102", ts[:8])
		if err == nil {
			return t.Format("02 Jan 2006")
		}
	}
	return ts
}

// GetSnapshots retrieves historical snapshots from Wayback Machine CDX API
func (c *ArchiveClient) GetSnapshots(ctx context.Context, targetURL string) (*ArchiveReport, error) {
	raw := strings.TrimSpace(targetURL)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	report := &ArchiveReport{
		QueryURL:    raw,
		CleanDomain: parsed.Hostname(),
		SaveURL:     fmt.Sprintf("https://web.archive.org/save/%s", raw),
		Snapshots:   make([]SnapshotItem, 0),
	}

	// 1. Query CDX API for snapshots
	cdxURL := fmt.Sprintf(
		"https://web.archive.org/cdx/search/cdx?url=%s&output=json&limit=60&fl=timestamp,original,mimetype,statuscode&collapse=timestamp:8",
		url.QueryEscape(raw),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", cdxURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo/1.0 (+https://github.com/BrianStovia/serxgo)")

	resp, err := c.client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var rows [][]string
		if jsonErr := json.NewDecoder(resp.Body).Decode(&rows); jsonErr == nil && len(rows) > 1 {
			// Row 0 is header ["timestamp","original","mimetype","statuscode"]
			for i := 1; i < len(rows); i++ {
				r := rows[i]
				if len(r) < 4 {
					continue
				}
				ts := r[0]
				orig := r[1]
				mime := r[2]
				status := r[3]
				archURL := fmt.Sprintf("https://web.archive.org/web/%s/%s", ts, orig)

				report.Snapshots = append(report.Snapshots, SnapshotItem{
					Timestamp:   ts,
					DateStr:     formatTimestamp(ts),
					OriginalURL: orig,
					MimeType:    mime,
					StatusCode:  status,
					ArchiveURL:  archURL,
				})
			}
		}
	}

	// 2. Fallback to Wayback Availability API if CDX was empty
	if len(report.Snapshots) == 0 {
		availURL := fmt.Sprintf("https://archive.org/wayback/available?url=%s", url.QueryEscape(raw))
		reqAvail, errAvail := http.NewRequestWithContext(ctx, "GET", availURL, nil)
		if errAvail == nil {
			reqAvail.Header.Set("User-Agent", "SearXGo/1.0")
			if respAvail, errDo := c.client.Do(reqAvail); errDo == nil && respAvail.StatusCode == http.StatusOK {
				defer respAvail.Body.Close()
				var availResult struct {
					ArchivedSnapshots struct {
						Closest struct {
							Status    string `json:"status"`
							Available bool   `json:"available"`
							URL       string `json:"url"`
							Timestamp string `json:"timestamp"`
						} `json:"closest"`
					} `json:"archived_snapshots"`
				}
				if jsonErr := json.NewDecoder(respAvail.Body).Decode(&availResult); jsonErr == nil {
					closest := availResult.ArchivedSnapshots.Closest
					if closest.Available && closest.URL != "" {
						report.Snapshots = append(report.Snapshots, SnapshotItem{
							Timestamp:   closest.Timestamp,
							DateStr:     formatTimestamp(closest.Timestamp),
							OriginalURL: raw,
							MimeType:    "text/html",
							StatusCode:  closest.Status,
							ArchiveURL:  closest.URL,
						})
					}
				}
			}
		}
	}

	report.TotalSnapshots = len(report.Snapshots)
	if len(report.Snapshots) > 0 {
		report.FirstSeen = report.Snapshots[0].DateStr
		report.LastSeen = report.Snapshots[len(report.Snapshots)-1].DateStr
	}

	return report, nil
}
