package cloudrecon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// BucketFinding represents the status of an audited bucket
type BucketFinding struct {
	Provider   string `json:"provider"` // AWS S3, Google Cloud, Azure Blob, DigitalOcean Spaces
	BucketName string `json:"bucket_name"`
	URL        string `json:"url"`
	Status     string `json:"status"` // OPEN, PROTECTED, NOT_FOUND, ERROR
	StatusCode int    `json:"status_code"`
	Details    string `json:"details"`
}

// BucketReport holds the full cloud exposure audit result
type BucketReport struct {
	Target           string          `json:"target"`
	TotalScanned     int             `json:"total_scanned"`
	OpenBuckets      []BucketFinding `json:"open_buckets"`
	ProtectedBuckets []BucketFinding `json:"protected_buckets"`
	NotFoundCount    int             `json:"not_found_count"`
	DurationMs       int64           `json:"duration_ms"`
	Error            string          `json:"error,omitempty"`
}

type providerTemplate struct {
	name    string
	urlFmt  string
	enabled bool
}

var defaultTemplates = []providerTemplate{
	{name: "AWS S3", urlFmt: "https://%s.s3.amazonaws.com"},
	{name: "Google Cloud", urlFmt: "https://storage.googleapis.com/%s"},
	{name: "DigitalOcean Spaces", urlFmt: "https://%s.nyc3.digitaloceanspaces.com"},
	{name: "Azure Blob", urlFmt: "https://%s.blob.core.windows.net/public"},
}

var commonSuffixes = []string{
	"",
	"-backup",
	"-backups",
	"-data",
	"-public",
	"-assets",
	"-static",
	"-dev",
	"-prod",
	"-media",
	"-internal",
	"-staging",
	"-logs",
}

var rxSanitize = regexp.MustCompile(`[^a-z0-9\-]`)

// ScanBuckets audits public cloud storage endpoints for target keyword
func ScanBuckets(ctx context.Context, target string) *BucketReport {
	start := time.Now()
	report := &BucketReport{
		Target:           target,
		OpenBuckets:      []BucketFinding{},
		ProtectedBuckets: []BucketFinding{},
	}

	clean := strings.ToLower(strings.TrimSpace(target))
	clean = strings.TrimPrefix(clean, "http://")
	clean = strings.TrimPrefix(clean, "https://")
	if idx := strings.Index(clean, "."); idx != -1 {
		clean = clean[:idx]
	}
	clean = rxSanitize.ReplaceAllString(clean, "")

	if len(clean) < 2 {
		report.Error = "Target name too short (minimum 2 alphanumeric characters)."
		return report
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	type checkJob struct {
		provider   string
		bucketName string
		url        string
	}

	var jobs []checkJob
	for _, suffix := range commonSuffixes {
		bName := clean + suffix
		for _, tmpl := range defaultTemplates {
			jobs = append(jobs, checkJob{
				provider:   tmpl.name,
				bucketName: bName,
				url:        fmt.Sprintf(tmpl.urlFmt, bName),
			})
		}
	}

	report.TotalScanned = len(jobs)

	jobChan := make(chan checkJob, len(jobs))
	for _, j := range jobs {
		jobChan <- j
	}
	close(jobChan)

	var mu sync.Mutex
	var wg sync.WaitGroup
	workers := 8

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				finding := probeBucket(ctx, client, job.provider, job.bucketName, job.url)
				mu.Lock()
				switch finding.Status {
				case "OPEN":
					report.OpenBuckets = append(report.OpenBuckets, finding)
				case "PROTECTED":
					report.ProtectedBuckets = append(report.ProtectedBuckets, finding)
				case "NOT_FOUND":
					report.NotFoundCount++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	report.DurationMs = time.Since(start).Milliseconds()
	return report
}

func probeBucket(ctx context.Context, client *http.Client, provider string, bName string, targetURL string) BucketFinding {
	finding := BucketFinding{
		Provider:   provider,
		BucketName: bName,
		URL:        targetURL,
		Status:     "NOT_FOUND",
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		finding.Status = "ERROR"
		finding.Details = err.Error()
		return finding
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SearXGo-CloudAudit/1.0")

	resp, err := client.Do(req)
	if err != nil {
		finding.Status = "NOT_FOUND"
		return finding
	}
	defer resp.Body.Close()

	finding.StatusCode = resp.StatusCode

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := string(bodyBytes)

	switch resp.StatusCode {
	case http.StatusOK:
		if strings.Contains(bodyStr, "ListBucketResult") || strings.Contains(bodyStr, "EnumerationResults") {
			finding.Status = "OPEN"
			finding.Details = "CRITICAL: Bucket exists and allows anonymous directory/object listing!"
		} else {
			finding.Status = "OPEN"
			finding.Details = "Bucket responds with 200 OK. Public files or static website may be exposed."
		}
	case http.StatusForbidden:
		finding.Status = "PROTECTED"
		finding.Details = "Bucket exists, but anonymous access is restricted (403 Forbidden / AccessDenied)."
	case http.StatusNotFound:
		finding.Status = "NOT_FOUND"
	default:
		if resp.StatusCode == 301 || resp.StatusCode == 307 {
			finding.Status = "PROTECTED"
			finding.Details = fmt.Sprintf("Bucket exists in another cloud region (Redirected %d).", resp.StatusCode)
		} else {
			finding.Status = "NOT_FOUND"
		}
	}

	return finding
}
