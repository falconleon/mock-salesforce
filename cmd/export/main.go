// Command export reads from the SQLite database and writes JSON
// seed files for the Salesforce mock API.
//
// Usage:
//
//	go run ./cmd/export/
//	go run ./cmd/export/ --out ./testdata/seed/ --db data/mock.db
package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/exporter"
)

func main() {
	outDir := flag.String("out", "./testdata/seed/", "Output directory for seed files")
	dbPath := flag.String("db", "data/mock.db", "Path to SQLite database")
	flag.Parse()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	// Open database (read-only)
	store, err := db.Open(*dbPath, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer store.Close()

	logger.Info().Str("out", *outDir).Msg("Exporting Salesforce seed data")

	sfExporter := exporter.NewSalesforceExporter(store.DB(), logger)
	if err := sfExporter.Export(*outDir); err != nil {
		logger.Fatal().Err(err).Msg("Salesforce export failed")
	}

	logger.Info().Str("dir", *outDir).Msg("Export complete")
}
