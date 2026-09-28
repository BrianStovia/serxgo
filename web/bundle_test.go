package web_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModularCSSStructure(t *testing.T) {
	cssDir := filepath.Join(".", "static", "css")
	requiredFiles := []string{
		"main.css",
		"variables.css",
		"components.css",
		"results.css",
		"tools.css",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(cssDir, file)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Required modular CSS file %s missing: %v", file, err)
		}
		if info.Size() == 0 {
			t.Fatalf("Modular CSS file %s is empty", file)
		}
	}

	mainContent, err := os.ReadFile(filepath.Join(cssDir, "main.css"))
	if err != nil {
		t.Fatalf("Failed to read main.css: %v", err)
	}

	str := string(mainContent)
	imports := []string{"variables.css", "components.css", "results.css", "tools.css"}
	for _, imp := range imports {
		if !strings.Contains(str, imp) {
			t.Errorf("main.css does not import %s", imp)
		}
	}
}

func TestModularJSStructure(t *testing.T) {
	jsDir := filepath.Join(".", "static", "js")
	modulesDir := filepath.Join(jsDir, "modules")

	requiredModules := []string{
		"theme.js",
		"search.js",
		"navigation.js",
		"settings.js",
		"features.js",
		"core.js",
	}

	for _, file := range requiredModules {
		path := filepath.Join(modulesDir, file)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Required modular JS file %s missing: %v", file, err)
		}
		if info.Size() == 0 {
			t.Fatalf("Modular JS file %s is empty", file)
		}
	}

	appContent, err := os.ReadFile(filepath.Join(jsDir, "app.js"))
	if err != nil {
		t.Fatalf("Failed to read app.js: %v", err)
	}

	appStr := string(appContent)
	if !strings.Contains(appStr, "window.SearXGo") {
		t.Errorf("app.js is missing window.SearXGo export")
	}
	if !strings.Contains(appStr, "initAll()") {
		t.Errorf("app.js is missing initAll() call")
	}
}
