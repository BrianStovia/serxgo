package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"searxgo/internal/aggregator"
	"searxgo/internal/config"
	"searxgo/internal/engine"
	"searxgo/internal/handler"
)

func main() {
	cfg := config.LoadConfig()

	portFlag := flag.Int("port", cfg.Port, "HTTP server port")
	hostFlag := flag.String("host", cfg.Host, "HTTP server host")
	timeoutFlag := flag.Int("timeout", int(cfg.Timeout.Milliseconds()), "Search timeout in milliseconds")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("SearXGo v1.0.0 (SearXNG Complete Parity in Golang)")
		fmt.Println("Engines: 260 providers registered across 9 categories")
		return
	}

	cfg.Port = *portFlag
	cfg.Host = *hostFlag
	cfg.Timeout = time.Duration(*timeoutFlag) * time.Millisecond

	// 1. Register Full Suite of Search Engines (SearXNG Parity)
	// General & News
	engine.DefaultRegistry.Register(engine.NewDuckDuckGoEngine())
	engine.DefaultRegistry.Register(engine.NewGoogleEngine())
	engine.DefaultRegistry.Register(engine.NewBingEngine())
	engine.DefaultRegistry.Register(engine.NewBraveEngine())
	engine.DefaultRegistry.Register(engine.NewWikipediaEngine())

	// Videos & Media
	engine.DefaultRegistry.Register(engine.NewYouTubeEngine())

	// IT & Code
	engine.DefaultRegistry.Register(engine.NewGitHubEngine())
	engine.DefaultRegistry.Register(engine.NewStackOverflowEngine())
	engine.DefaultRegistry.Register(engine.NewNPMEngine())
	engine.DefaultRegistry.Register(engine.NewPyPIEngine())
	engine.DefaultRegistry.Register(engine.NewHackerNewsEngine())

	// Science & Academic
	engine.DefaultRegistry.Register(engine.NewArxivEngine())
	engine.DefaultRegistry.Register(engine.NewPubMedEngine())

	// Social
	engine.DefaultRegistry.Register(engine.NewRedditEngine())

	// Files & Torrents
	engine.DefaultRegistry.Register(engine.NewPirateBayEngine())
	engine.DefaultRegistry.Register(engine.NewInternetArchiveEngine())

	// Music & Lyrics
	engine.DefaultRegistry.Register(engine.NewGeniusEngine())

	// Maps & Geolocation
	engine.DefaultRegistry.Register(engine.NewOpenStreetMapEngine())

	// 1.1 Register All 260 Engines from priv.au / SearXNG Catalog
	engine.RegisterAllCatalogEngines(engine.DefaultRegistry)

	// 2. Initialize Aggregator
	agg := aggregator.NewAggregator(engine.DefaultRegistry, cfg.Timeout)

	// 3. Initialize HTTP Handlers
	h, err := handler.NewHandler(cfg, agg)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize handlers: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      h.WrapMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("==================================================================")
		fmt.Println("🪐 SearXGo Pro - Complete Privacy Metasearch Aggregator")
		fmt.Println("==================================================================")
		fmt.Printf("➜ Web Interface:   http://localhost:%d\n", cfg.Port)
		fmt.Printf("➜ REST API:        http://localhost:%d/api/search?q=golang\n", cfg.Port)
		fmt.Printf("➜ RSS Feed:        http://localhost:%d/search?q=golang&format=rss\n", cfg.Port)
		fmt.Printf("➜ CSV Export:      http://localhost:%d/search?q=golang&format=csv\n", cfg.Port)
		fmt.Printf("➜ OpenSearch:      http://localhost:%d/opensearch.xml\n", cfg.Port)
		fmt.Printf("➜ Telemetry Stats: http://localhost:%d/stats\n", cfg.Port)
		fmt.Printf("➜ Active Engines:  %d providers registered across 9 categories\n", len(engine.DefaultRegistry.GetAll()))
		fmt.Println("==================================================================")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stopChan
	fmt.Println("\nShutting down SearXGo Pro gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}

	fmt.Println("Server stopped.")
}
