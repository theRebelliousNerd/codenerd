package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-rod/rod/lib/launcher"
)

// Chrome Launcher Helper
// Starts Chrome with remote debugging enabled and keeps it running
// Usage: go run chrome_launcher.go --port 9222 --headless=false

func main() {
	port := flag.String("port", "9222", "Remote debugging port")
	headless := flag.Bool("headless", false, "Run in headless mode")
	userDataDir := flag.String("user-data-dir", "", "Chrome user data directory (empty = temp)")
	noSandbox := flag.Bool("no-sandbox", false, "Disable sandbox (for Docker/CI)")

	flag.Parse()

	// Configure launcher
	l := launcher.New().
		Set("remote-debugging-port", *port).
		Headless(*headless)

	if *userDataDir != "" {
		l = l.UserDataDir(*userDataDir)
	}

	if *noSandbox {
		l = l.NoSandbox(true)
	}

	// Launch Chrome
	url, err := l.Launch()
	if err != nil {
		log.Fatalf("Failed to launch Chrome: %v", err)
	}

	fmt.Printf("Chrome launched successfully!\n")
	fmt.Printf("Remote debugging URL: %s\n", url)
	fmt.Printf("Connect with: rod.New().ControlURL(\"%s\").MustConnect()\n", url)
	fmt.Printf("\nPress Ctrl+C to stop...\n")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down Chrome...")
	l.Cleanup()
}
