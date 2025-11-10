//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/Test-Alliance-Please-Ignore/goosniffer/internal/clipboardwatcher"
	"github.com/Test-Alliance-Please-Ignore/goosniffer/internal/moonparse"
)

type Options struct {
	APIEndpoint string `long:"api-endpoint" env:"API_ENDPOINT" description:"API endpoint URL"`
	APIToken    string `long:"api-token" env:"API_TOKEN" description:"API Bearer token"`
}

func main() {
	var opts Options

	// Parse CLI flags + environment variables (API_ENDPOINT, API_TOKEN).
	parser := flags.NewParser(&opts, flags.Default)
	if _, err := parser.Parse(); err != nil {
		// If user asked for help, just exit quietly.
		var fErr *flags.Error
		if errors.As(err, &fErr) && errors.Is(fErr.Type, flags.ErrHelp) {
			return
		}
		log.Fatalf("failed to parse flags: %v", err)
	}

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	log.Println("Listening for clipboard text. Press Ctrl+C to exit.")

	watchErr := clipboardwatcher.Watch(func(text string) {
		log.Println("Clipboard changed")

		if looksLikeMoonScan(text) {
			log.Println("Possible moon scan data detected")
			data, parseErr := moonparse.ParseMoons(text)
			if parseErr != nil {
				log.Fatalf("failed to parse moons: %v", parseErr)
			}

			payload, jsErr := json.Marshal(data)
			if jsErr != nil {
				log.Printf("Failed to marshal moon scan data: %v", jsErr)
				return
			}
			log.Println("Moon scan parsed:")

			j, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(j))

			if opts.APIEndpoint != "" {
				log.Println("Uploading data")

				req, reqErr := http.NewRequest("POST", opts.APIEndpoint, bytes.NewReader(payload))
				if reqErr != nil {
					log.Printf("Failed to create POST request: %v", reqErr)
					return
				}
				req.Header.Set("Content-Type", "application/json")

				if opts.APIToken != "" {
					req.Header.Set("Authorization", "Bearer "+opts.APIToken)
				}

				resp, httpErr := httpClient.Do(req)
				if httpErr != nil {
					log.Printf("POST %s failed: %v", opts.APIEndpoint, httpErr)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					log.Printf("Moon scan posted to %s (status %s, %d bytes)", opts.APIEndpoint, resp.Status, len(payload))
				} else {
					log.Printf("Moon scan POST to %s returned status %s", opts.APIEndpoint, resp.Status)
				}
			}
		}
	})

	if watchErr != nil {
		log.Fatal(watchErr)
	}
}

func looksLikeMoonScan(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	if strings.Contains(s, "Moon Product") {
		return true
	}

	return false
}
