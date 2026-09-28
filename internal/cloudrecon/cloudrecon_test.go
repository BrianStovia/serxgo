package cloudrecon

import (
	"context"
	"testing"
	"time"
)

func TestCloudReconShortTarget(t *testing.T) {
	ctx := context.Background()
	report := ScanBuckets(ctx, "a")
	if report.Error == "" {
		t.Errorf("expected error for target too short")
	}
}

func TestCloudReconPermutations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	report := ScanBuckets(ctx, "example-corp-test-xyz")
	if report.TotalScanned == 0 {
		t.Errorf("expected scan permutations > 0")
	}
}
