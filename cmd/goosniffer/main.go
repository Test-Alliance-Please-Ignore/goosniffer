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
		var ferr *flags.Error
		if errors.As(err, &ferr) && errors.Is(ferr.Type, flags.ErrHelp) {
			return
		}
		log.Fatalf("failed to parse flags: %v", err)
	}

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	log.Println("Listening for clipboard text. Press Ctrl+C to exit.")

	err := clipboardwatcher.Watch(func(text string) {
		log.Println("Clipboard changed")

		if looksLikeMoonScan(text) {
			log.Println("Possible moon scan data detected")
			data, err := moonparse.ParseMoons(text)

			payload, err := json.Marshal(data)
			if err != nil {
				log.Printf("Failed to marshal moon scan data: %v", err)
				return
			}
			log.Println("Moon scan parsed:")

			j, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(j))

			if (opts.APIToken != "") && (opts.APIEndpoint != "") {
				log.Println("Uploading data")

				req, err := http.NewRequest("POST", opts.APIEndpoint, bytes.NewReader(payload))
				if err != nil {
					log.Printf("Failed to create POST request: %v", err)
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+opts.APIToken)

				// Send it.
				resp, err := httpClient.Do(req)
				if err != nil {
					log.Printf("POST %s failed: %v", opts.APIEndpoint, err)
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

	if err != nil {
		log.Fatal(err)
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
