package apitester

import (
	"context"
	"strings"
	"testing"
)

func TestCodeGenerators(t *testing.T) {
	p := RequestPayload{
		Method: "POST",
		URL:    "https://api.example.com/v1/users",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"Content-Type":  "application/json",
		},
		Body: `{"name":"alice"}`,
	}

	curl := GenerateCurl(p)
	if !strings.Contains(curl, "curl -X POST") || !strings.Contains(curl, "Authorization: Bearer token123") {
		t.Errorf("unexpected curl output: %s", curl)
	}

	goCode := GenerateGo(p)
	if !strings.Contains(goCode, "http.NewRequest(\"POST\"") {
		t.Errorf("unexpected go output: %s", goCode)
	}

	pyCode := GeneratePython(p)
	if !strings.Contains(pyCode, "requests.post") {
		t.Errorf("unexpected python output: %s", pyCode)
	}

	jsCode := GenerateJS(p)
	if !strings.Contains(jsCode, "method: 'POST'") {
		t.Errorf("unexpected js output: %s", jsCode)
	}
}

func TestSSRFProtection(t *testing.T) {
	ctx := context.Background()

	pLocal := RequestPayload{
		URL: "http://127.0.0.1:8080/admin",
	}
	res := ExecuteRequest(ctx, pLocal)
	if res.Error == "" || !strings.Contains(res.Error, "restricted") {
		t.Errorf("expected SSRF error for 127.0.0.1, got: %s", res.Error)
	}

	pLocalhost := RequestPayload{
		URL: "http://localhost:3000",
	}
	res2 := ExecuteRequest(ctx, pLocalhost)
	if res2.Error == "" || !strings.Contains(res2.Error, "restricted") {
		t.Errorf("expected SSRF error for localhost, got: %s", res2.Error)
	}
}
