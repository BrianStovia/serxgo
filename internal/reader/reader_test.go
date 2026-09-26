package reader

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractContent(t *testing.T) {
	rawHTML := `
	<html>
	<head>
		<title>Sample Tech Article</title>
		<meta name="author" content="Jane Doe">
	</head>
	<body>
		<header><nav><a href="/">Home</a></nav></header>
		<div class="cookie-banner">Accept cookies</div>
		<article>
			<h1>Understanding Golang Concurrency</h1>
			<p>Golang provides lightweight threads called goroutines.</p>
			<p>Channels allow channels to communicate safely.</p>
		</article>
		<footer>Copyright 2026</footer>
	</body>
	</html>
	`

	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	art := &Article{URL: "https://example.com/post"}
	extractMeta(doc, art)
	extractContent(doc, art)

	if art.Title != "Sample Tech Article" {
		t.Errorf("Expected title 'Sample Tech Article', got '%s'", art.Title)
	}

	if art.Author != "Jane Doe" {
		t.Errorf("Expected author 'Jane Doe', got '%s'", art.Author)
	}

	if strings.Contains(art.ContentHTML, "cookie-banner") || strings.Contains(art.ContentHTML, "Copyright 2026") {
		t.Errorf("Expected cookies and footer to be stripped, got: %s", art.ContentHTML)
	}

	if !strings.Contains(art.ContentHTML, "goroutines") {
		t.Errorf("Expected article body content to be preserved, got: %s", art.ContentHTML)
	}
}
