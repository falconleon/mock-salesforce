// Command generate_photos generates profile pictures for contacts and users.
//
// Usage:
//
//	go run ./cmd/generate_photos --db data/mock.db --type contacts --dry-run
//	go run ./cmd/generate_photos --db data/mock.db --type users
//	go run ./cmd/generate_photos --db data/mock.db --type all
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/rs/zerolog"

	config "github.com/falconleon/mock-salesforce/internal/config/config"
	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/generator/photo"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

const (
	costPerImage = 0.01 // USD per CogView-4 image
)

type persona struct {
	ID        string
	Type      string // "contact" or "user"
	FirstName string
	LastName  string
	Title     string
}

func main() {
	dbPath := flag.String("db", "data/mock.db", "Path to the mock database")
	personaType := flag.String("type", "all", "Persona type: contacts, users, or all")
	dryRun := flag.Bool("dry-run", false, "Show what would be generated without making API calls")
	limit := flag.Int("limit", 0, "Max number of photos to generate (0 = no limit)")
	outputDir := flag.String("output-dir", "", "Output directory (default: assets/profile_images)")
	flag.Parse()

	// Load .env from repo root
	config.LoadEnvFromRepoRoot()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	// Determine output directory
	outDir := *outputDir
	if outDir == "" {
		// Default: relative to db path
		dbDir := filepath.Dir(*dbPath)
		outDir = filepath.Join(dbDir, "..", "assets", "profile_images", "images")
	}

	logger.Info().
		Str("db", *dbPath).
		Str("type", *personaType).
		Bool("dry-run", *dryRun).
		Int("limit", *limit).
		Str("output-dir", outDir).
		Msg("Starting photo generation")

	// Validate persona type
	*personaType = strings.ToLower(*personaType)
	if *personaType != "contacts" && *personaType != "users" && *personaType != "all" {
		logger.Fatal().Str("type", *personaType).Msg("Invalid persona type. Use: contacts, users, or all")
	}

	// Open database
	store, err := db.Open(*dbPath, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer store.Close()

	// Ensure profile_images table exists (migration)
	if err := store.MigrateProfileImages(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to migrate profile_images table")
	}

	// Query personas
	personas, err := queryPersonas(store, *personaType)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to query personas")
	}

	if len(personas) == 0 {
		logger.Warn().Msg("No personas found in database")
		return
	}

	// Apply limit
	if *limit > 0 && len(personas) > *limit {
		personas = personas[:*limit]
	}

	// Show cost estimation
	cost := float64(len(personas)) * costPerImage
	logger.Info().
		Int("count", len(personas)).
		Float64("cost_usd", cost).
		Msgf("Would generate %d photos (cost: $%.2f)", len(personas), cost)

	if *dryRun {
		// Print details in dry-run mode
		for i, p := range personas {
			logger.Info().
				Int("num", i+1).
				Str("type", p.Type).
				Str("name", p.FirstName+" "+p.LastName).
				Str("title", p.Title).
				Msg("Would generate photo")
		}
		logger.Info().Msg("Dry run complete - no photos generated")
		return
	}

	// Generate photos
	if err := generatePhotos(logger, store, personas, outDir); err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate photos")
	}

	logger.Info().Int("count", len(personas)).Msg("Photo generation complete")
}

func queryPersonas(store *db.Store, personaType string) ([]persona, error) {
	var personas []persona

	// Query contacts
	if personaType == "contacts" || personaType == "all" {
		contacts, err := store.QueryContacts()
		if err != nil {
			return nil, fmt.Errorf("query contacts: %w", err)
		}
		for _, c := range contacts {
			personas = append(personas, persona{
				ID:        c.ID,
				Type:      "contact",
				FirstName: c.FirstName,
				LastName:  c.LastName,
				Title:     c.Title,
			})
		}
	}

	// Query users
	if personaType == "users" || personaType == "all" {
		users, err := store.QueryUsers()
		if err != nil {
			return nil, fmt.Errorf("query users: %w", err)
		}
		for _, u := range users {
			personas = append(personas, persona{
				ID:        u.ID,
				Type:      "user",
				FirstName: u.FirstName,
				LastName:  u.LastName,
				Title:     u.Title,
			})
		}
	}

	return personas, nil
}

func generatePhotos(logger zerolog.Logger, store *db.Store, personas []persona, outputDir string) error {
	// Check for API key
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ZAI_API_KEY environment variable not set")
	}

	// Create photo generator
	generator := photo.NewPhotoGenerator(apiKey, outputDir)

	// Create metadata store
	metadataPath := filepath.Join(filepath.Dir(outputDir), "metadata.json")
	metadataStore := photo.NewMetadataStore(metadataPath)

	ctx := context.Background()
	total := len(personas)
	generated := 0
	failed := 0

	for i, p := range personas {
		// Generate random traits for this persona
		// Use a deterministic seed based on persona ID for reproducibility
		gofakeit.Seed(hashStringToInt64(p.ID))
		personSeed := generatePersonSeed(p)

		// Check for existing match
		if match := metadataStore.FindMatch(personSeed); match != nil {
			logger.Info().
				Int("num", i+1).
				Int("total", total).
				Str("name", p.FirstName+" "+p.LastName).
				Str("match_id", match.ID).
				Msg("Found existing photo match")
			// Mark photo as in use and save the update
			if err := metadataStore.MarkInUse(match.ID, p.ID); err != nil {
				logger.Error().Err(err).Str("photo_id", match.ID).Msg("Failed to mark photo in use")
			}
			continue
		}

		// Generate new photo
		logger.Info().
			Int("num", i+1).
			Int("total", total).
			Str("name", p.FirstName+" "+p.LastName).
			Str("type", p.Type).
			Msg("Generating photo")

		metadata, imagePath, err := photo.GenerateAndSave(ctx, generator, metadataStore, personSeed, p.Type, p.ID)
		if err != nil {
			logger.Error().
				Err(err).
				Str("name", p.FirstName+" "+p.LastName).
				Msg("Failed to generate photo")
			failed++
			continue
		}

		// Insert record into database
		profileImage := &db.ProfileImage{
			ID:          metadata.ID,
			PersonaType: p.Type,
			PersonaID:   p.ID,
			ImagePath:   imagePath,
			FirstName:   p.FirstName,
			LastName:    p.LastName,
			Age:         personSeed.Age,
			Gender:      metadata.Gender,
			Ethnicity:   metadata.Ethnicity,
			HairColor:   metadata.HairColor,
			HairStyle:   metadata.HairStyle,
			EyeColor:    metadata.EyeColor,
			Glasses:     metadata.Glasses,
			GeneratedAt: metadata.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if err := store.InsertProfileImage(profileImage); err != nil {
			logger.Error().Err(err).Str("id", metadata.ID).Msg("Failed to insert profile image record")
		}

		generated++
		logger.Info().
			Int("num", i+1).
			Int("total", total).
			Str("filename", metadata.Filename).
			Str("path", imagePath).
			Msg("Photo generated")
	}

	logger.Info().
		Int("generated", generated).
		Int("reused", total-generated-failed).
		Int("failed", failed).
		Msg("Photo generation summary")

	return nil
}

// generatePersonSeed creates a PersonSeed with random traits for a persona.
func generatePersonSeed(p persona) seed.PersonSeed {
	// Determine role level from title
	role := roleFromTitle(p.Title)

	// Use the seed package to generate consistent traits
	ps := seed.NewPersonSeed(role)

	// Override with actual name from persona
	ps.FirstName = p.FirstName
	ps.LastName = p.LastName

	return ps
}

// roleFromTitle estimates a role level from a job title.
func roleFromTitle(title string) seed.RoleLevel {
	title = strings.ToLower(title)

	switch {
	case strings.Contains(title, "executive") ||
		strings.Contains(title, "ceo") ||
		strings.Contains(title, "cto") ||
		strings.Contains(title, "cfo") ||
		strings.Contains(title, "president"):
		return seed.RoleExecutive
	case strings.Contains(title, "director") ||
		strings.Contains(title, "vp") ||
		strings.Contains(title, "vice president"):
		return seed.RoleDirector
	case strings.Contains(title, "manager"):
		return seed.RoleManager
	case strings.Contains(title, "lead") ||
		strings.Contains(title, "senior") ||
		strings.Contains(title, "sr."):
		return seed.RoleSupportL2
	default:
		return seed.RoleSupportL1
	}
}

// hashStringToInt64 converts a string to a deterministic int64 for seeding.
func hashStringToInt64(s string) int64 {
	var h int64
	for _, c := range s {
		h = 31*h + int64(c)
	}
	return h
}

