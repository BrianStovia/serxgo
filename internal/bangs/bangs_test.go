package bangs

import (
	"testing"

	"searxgo/internal/models"
)

func TestParseBangs(t *testing.T) {
	tests := []struct {
		input       string
		defaultCat  models.Category
		expectedQ   string
		expectedCat models.Category
		expectedEng string
		expectedLang string
		isDirect    bool
	}{
		{
			input:       "!gh golang web framework",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "golang web framework",
			expectedCat: models.CategoryGeneral,
			expectedEng: "github",
		},
		{
			input:       "!images cute cats",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "cute cats",
			expectedCat: models.CategoryImages,
		},
		{
			input:       "!vid lo-fi hip hop",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "lo-fi hip hop",
			expectedCat: models.CategoryVideos,
		},
		{
			input:       ":id resep nasi goreng",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "resep nasi goreng",
			expectedCat: models.CategoryGeneral,
			expectedLang: "id",
		},
		{
			input:       ":images cute puppies",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "cute puppies",
			expectedCat: models.CategoryImages,
		},
		{
			input:       ":it rust memory safety",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "rust memory safety",
			expectedCat: models.CategoryIT,
		},
		{
			input:       "golang concurrency filetype:pdf",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "golang concurrency",
			expectedCat: models.CategoryGeneral,
		},
		{
			input:       "intitle:guide inurl:docs kubernetes",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "kubernetes",
			expectedCat: models.CategoryGeneral,
		},
		{
			input:       "!yt! lo-fi chill",
			defaultCat:  models.CategoryGeneral,
			expectedQ:   "lo-fi chill",
			expectedCat: models.CategoryGeneral,
			isDirect:    true,
		},
	}

	for _, tt := range tests {
		res := ParseBangs(tt.input, tt.defaultCat)
		if res.CleanQuery != tt.expectedQ {
			t.Errorf("ParseBangs(%q) CleanQuery = %q; want %q", tt.input, res.CleanQuery, tt.expectedQ)
		}
		if res.Category != tt.expectedCat {
			t.Errorf("ParseBangs(%q) Category = %q; want %q", tt.input, res.Category, tt.expectedCat)
		}
		if tt.expectedEng != "" {
			found := false
			for _, e := range res.Engines {
				if e == tt.expectedEng {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("ParseBangs(%q) expected engine %q in %v", tt.input, tt.expectedEng, res.Engines)
			}
		}
		if tt.expectedLang != "" && res.Language != tt.expectedLang {
			t.Errorf("ParseBangs(%q) Language = %q; want %q", tt.input, res.Language, tt.expectedLang)
		}
		if tt.isDirect && res.DirectRedirectURL == "" {
			t.Errorf("ParseBangs(%q) expected direct redirect URL", tt.input)
		}
	}
}
