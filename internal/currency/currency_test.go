package currency

import (
	"testing"
)

func TestCurrencyConvert(t *testing.T) {
	svc := NewService()

	// Convert 100 USD to IDR
	res, err := svc.Convert("USD", "IDR", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Result < 1000000 {
		t.Errorf("expected 100 USD to be > 1,000,000 IDR, got %f", res.Result)
	}
	if res.FromCode != "USD" || res.ToCode != "IDR" {
		t.Errorf("unexpected from/to codes: %s -> %s", res.FromCode, res.ToCode)
	}

	// Inverse conversion
	invRes, err := svc.Convert("IDR", "USD", res.Result)
	if err != nil {
		t.Fatalf("unexpected error on inverse conversion: %v", err)
	}
	if invRes.Result < 99.0 || invRes.Result > 101.0 {
		t.Errorf("expected ~100 USD, got %f", invRes.Result)
	}
}

func TestUnsupportedCurrency(t *testing.T) {
	svc := NewService()
	_, err := svc.Convert("INVALID_COIN", "USD", 10)
	if err == nil {
		t.Errorf("expected error for unsupported currency")
	}
}

func TestFormatCurrencyNumber(t *testing.T) {
	formatted := formatCurrencyNumber(16250.75, 2)
	if formatted != "16,250.75" {
		t.Errorf("expected 16,250.75, got %s", formatted)
	}

	formattedLarge := formatCurrencyNumber(1500000, 0)
	if formattedLarge != "1,500,000" {
		t.Errorf("expected 1,500,000, got %s", formattedLarge)
	}
}
