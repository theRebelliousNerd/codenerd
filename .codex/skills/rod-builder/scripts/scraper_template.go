package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Scraper Template
// Customizable web scraper with best practices built-in
// Usage: go run scraper_template.go --url "https://example.com" --output data.json

type ScraperConfig struct {
	URL         string
	OutputFile  string
	Headless    bool
	Timeout     time.Duration
	WaitFor     string
	Selector    string
}

type ScrapedData struct {
	URL       string                   `json:"url"`
	Timestamp string                   `json:"timestamp"`
	Data      []map[string]interface{} `json:"data"`
}

func main() {
	config := parseFlags()

	data, err := scrape(config)
	if err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	if err := saveResults(data, config.OutputFile); err != nil {
		log.Fatalf("Failed to save results: %v", err)
	}

	fmt.Printf("Successfully scraped %d items to %s\n", len(data.Data), config.OutputFile)
}

func parseFlags() ScraperConfig {
	url := flag.String("url", "", "URL to scrape (required)")
	output := flag.String("output", "output.json", "Output file path")
	headless := flag.Bool("headless", true, "Run in headless mode")
	timeout := flag.Duration("timeout", 30*time.Second, "Page load timeout")
	waitFor := flag.String("wait-for", "", "CSS selector to wait for before scraping")
	selector := flag.String("selector", "", "CSS selector for items to scrape")

	flag.Parse()

	if *url == "" {
		fmt.Println("Error: --url is required")
		flag.Usage()
		os.Exit(1)
	}

	return ScraperConfig{
		URL:        *url,
		OutputFile: *output,
		Headless:   *headless,
		Timeout:    *timeout,
		WaitFor:    *waitFor,
		Selector:   *selector,
	}
}

func scrape(config ScraperConfig) (*ScrapedData, error) {
	// Launch browser
	l := launcher.New().Headless(config.Headless)
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	defer l.Cleanup()

	browser := rod.New().ControlURL(url).MustConnect()
	defer browser.MustClose()

	// Create page with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	page := browser.Context(ctx).MustPage(config.URL)
	defer page.MustClose()

	// Wait for page load
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait load: %w", err)
	}

	// Wait for specific element if specified
	if config.WaitFor != "" {
		if _, err := page.Element(config.WaitFor); err != nil {
			return nil, fmt.Errorf("wait for selector: %w", err)
		}
	}

	// Extract data
	var items []map[string]interface{}

	if config.Selector != "" {
		// Use provided selector
		elements, err := page.Elements(config.Selector)
		if err != nil {
			return nil, fmt.Errorf("find elements: %w", err)
		}

		for _, el := range elements {
			item := map[string]interface{}{
				"text": el.MustText(),
				"html": el.MustHTML(),
			}
			items = append(items, item)
		}
	} else {
		// Default: extract page metadata
		title := page.MustInfo().Title
		url := page.MustInfo().URL

		items = append(items, map[string]interface{}{
			"title": title,
			"url":   url,
		})
	}

	return &ScrapedData{
		URL:       config.URL,
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      items,
	}, nil
}

func saveResults(data *ScrapedData, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
