package instant

import (
	"testing"
)

func TestCalculator(t *testing.T) {
	tests := []struct {
		query         string
		expectedValue string
	}{
		{"2 + 2", "4"},
		{"10 * 5 - 2", "48"},
		{"sqrt(144)", "12"},
		{"15% of 200", "30"},
		{"100 km to miles", "62.1371 miles"},
		{"0 celsius to fahrenheit", "32.00 °F"},
		{"100 celsius to fahrenheit", "212.00 °F"},
		{"1024 mb to gb", "1 GB"},
		{"mean 10, 20, 30", "20"},
		{"median 1, 3, 3, 6, 7, 8, 9", "6"},
		{"sum 10, 20, 30, 40", "100"},
		{"min 5, 2, 9, -1", "-1"},
		{"max 5, 2, 9, -1", "9"},
		{"prod 2, 3, 4", "24"},
		{"range 10, 50, 100", "90"},
	}

	for _, tt := range tests {
		ans := CheckCalculator(tt.query)
		if ans == nil {
			t.Errorf("CheckCalculator(%q) returned nil; want value %q", tt.query, tt.expectedValue)
			continue
		}
		if ans.Value != tt.expectedValue {
			t.Errorf("CheckCalculator(%q) = %q; want %q", tt.query, ans.Value, tt.expectedValue)
		}
	}
}
