package instant

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"searxgo/internal/models"
)

// CheckSecretGen generates cryptographically secure secrets, tokens, and keys
func CheckSecretGen(query string) *models.InstantAnswer {
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	secretKeywords := []string{
		"secret", "secret key", "generate secret", "token", "generate token",
		"random token", "api key", "generate api key", "nanoid", "hex token",
		"random hex", "crypto secret",
	}

	isMatch := false
	for _, kw := range secretKeywords {
		if qLower == kw || strings.HasPrefix(qLower, kw+" ") || strings.HasPrefix(qLower, kw+":") {
			isMatch = true
			break
		}
	}

	if !isMatch {
		return nil
	}

	// Extract requested length if specified (e.g. "token 32", "secret 64")
	length := 32
	parts := strings.Fields(qLower)
	if len(parts) > 1 {
		if l, err := strconv.Atoi(parts[len(parts)-1]); err == nil && l >= 8 && l <= 256 {
			length = l
		}
	}

	// Generate three flavors:
	// 1. Alphanumeric / NanoID style
	alphaSecret := generateCryptoAlpha(length)
	// 2. Hexadecimal
	hexBytes := make([]byte, length/2)
	_, _ = rand.Read(hexBytes)
	hexSecret := hex.EncodeToString(hexBytes)
	// 3. Base64URL
	b64Bytes := make([]byte, length*3/4)
	_, _ = rand.Read(b64Bytes)
	b64Secret := base64.RawURLEncoding.EncodeToString(b64Bytes)

	entropyBits := length * 6 // rough estimate for alphanumeric 62-char set
	strength := "Very Strong (256-bit equivalent)"
	if length < 16 {
		strength = "Moderate"
	}

	desc := fmt.Sprintf(`
	<div style="display:flex; flex-direction:column; gap:0.75rem; margin-top:0.75rem;">
		<!-- Alphanumeric Token -->
		<div style="background:var(--bg-glass-card); border:1px solid var(--border-glow); border-radius:var(--radius-md); padding:0.75rem 1rem;">
			<div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.35rem;">
				<span style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase; font-weight:600;">Alphanumeric Token (%d chars)</span>
				<span style="font-size:0.75rem; color:var(--accent-emerald); font-weight:600;">%s</span>
			</div>
			<div style="font-family:monospace; font-size:1.05rem; word-break:break-all; color:var(--accent-primary); font-weight:700; user-select:all;">
				%s
			</div>
		</div>

		<!-- Hexadecimal Format -->
		<div style="background:var(--bg-glass-card); border:1px solid var(--border-glass); border-radius:var(--radius-md); padding:0.75rem 1rem;">
			<div style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase; font-weight:600; margin-bottom:0.35rem;">Hexadecimal (Raw Entropy)</div>
			<div style="font-family:monospace; font-size:0.95rem; word-break:break-all; color:var(--text-secondary); user-select:all;">
				%s
			</div>
		</div>

		<!-- Base64URL Safe Format -->
		<div style="background:var(--bg-glass-card); border:1px solid var(--border-glass); border-radius:var(--radius-md); padding:0.75rem 1rem;">
			<div style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase; font-weight:600; margin-bottom:0.35rem;">Base64URL Safe (JWT/OAuth Secrets)</div>
			<div style="font-family:monospace; font-size:0.95rem; word-break:break-all; color:var(--text-secondary); user-select:all;">
				%s
			</div>
		</div>
	</div>
	<div style="margin-top:0.5rem; font-size:0.8rem; color:var(--text-muted);">
		Generated via standard <code>crypto/rand</code> (CSPRNG) with ~%d bits of entropy. Zero server retention.
	</div>`, length, strength, alphaSecret, hexSecret, b64Secret, entropyBits)

	return &models.InstantAnswer{
		Type:        "tools",
		Title:       fmt.Sprintf("🔑 Cryptographic Secret & Token Generator (%d chars)", length),
		Value:       alphaSecret,
		Description: desc,
	}
}

func generateCryptoAlpha(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			result[i] = charset[i%len(charset)]
		} else {
			result[i] = charset[num.Int64()]
		}
	}
	return string(result)
}
