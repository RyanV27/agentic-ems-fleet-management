// Command extractcli is a dev harness for internal/extract: transcript in
// on stdin, extraction JSON out on stdout (ROADMAP.md S4 c7).
//
//	go run ./services/fleet/cmd/extractcli < testdata/transcripts/cardiac.txt
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/config"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/extract"
)

// seedZoneIDs mirrors internal/store/seed.go's seedZoneIDs. This CLI has no
// store dependency (S4's scope is the extraction client only), so the zone
// list is duplicated here for the dev harness rather than imported.
var seedZoneIDs = []string{"zone-1", "zone-2", "zone-3", "zone-4", "zone-5", "zone-6"}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "extractcli:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	transcript, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	client := extract.NewClient(cfg, seedZoneIDs)
	result, err := client.Extract(context.Background(), string(transcript))
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
