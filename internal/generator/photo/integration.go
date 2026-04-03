// Package photo provides profile photo generation with trait extraction and database integration.
package photo

import (
	"context"
	"fmt"
	"strings"

	"github.com/falconleon/mock-salesforce/internal/db"
	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

// TraitExtractor extracts PersonSeed traits from database records.
type TraitExtractor struct {
	// hashSeed is used to generate deterministic random traits from persona ID
	hashSeed func(id string) int64
}

// NewTraitExtractor creates a new trait extractor.
func NewTraitExtractor() *TraitExtractor {
	return &TraitExtractor{
		hashSeed: hashStringToInt64,
	}
}

// ExtractTraitsFromContact extracts PersonSeed traits from a Contact record.
// Uses the contact's ID for deterministic trait generation.
func (e *TraitExtractor) ExtractTraitsFromContact(contact db.Contact) seed.PersonSeed {
	// Determine role level from title
	role := roleFromTitle(contact.Title)

	// Seed for reproducibility
	seed.SetSeed(e.hashSeed(contact.ID))

	// Generate base traits
	ps := seed.NewPersonSeed(role)

	// Override with actual name from contact
	ps.FirstName = contact.FirstName
	ps.LastName = contact.LastName

	return ps
}

// ExtractTraitsFromUser extracts PersonSeed traits from a User record.
// Uses the user's ID for deterministic trait generation.
func (e *TraitExtractor) ExtractTraitsFromUser(user db.User) seed.PersonSeed {
	// Determine role level from title and user_role
	role := roleFromTitleAndRole(user.Title, user.UserRole)

	// Seed for reproducibility
	seed.SetSeed(e.hashSeed(user.ID))

	// Generate base traits
	ps := seed.NewPersonSeed(role)

	// Override with actual name from user
	ps.FirstName = user.FirstName
	ps.LastName = user.LastName

	return ps
}

// roleFromTitle estimates a role level from a job title.
func roleFromTitle(title string) seed.RoleLevel {
	title = strings.ToLower(title)

	// Check more specific patterns first to avoid false matches
	// e.g., "vice president" contains "president" but is Director level
	switch {
	case strings.Contains(title, "director") ||
		strings.Contains(title, "vp") ||
		strings.Contains(title, "vice president"):
		return seed.RoleDirector
	case strings.Contains(title, "executive") ||
		strings.Contains(title, "ceo") ||
		strings.Contains(title, "cto") ||
		strings.Contains(title, "cfo") ||
		strings.Contains(title, "president"):
		return seed.RoleExecutive
	case strings.Contains(title, "manager"):
		return seed.RoleManager
	case strings.Contains(title, "team lead"):
		return seed.RoleSupportL3
	case strings.Contains(title, "lead") ||
		strings.Contains(title, "senior") ||
		strings.Contains(title, "sr."):
		return seed.RoleSupportL2
	default:
		return seed.RoleSupportL1
	}
}

// roleFromTitleAndRole combines title and user role for better role estimation.
func roleFromTitleAndRole(title, userRole string) seed.RoleLevel {
	// First check title
	role := roleFromTitle(title)
	if role != seed.RoleSupportL1 {
		return role
	}

	// If title didn't give us a clear signal, check userRole
	userRole = strings.ToLower(userRole)
	switch {
	case strings.Contains(userRole, "admin"):
		return seed.RoleManager
	case strings.Contains(userRole, "support"):
		return seed.RoleSupportL1
	}

	return role
}

// hashStringToInt64 converts a string to a deterministic int64 for seeding.
func hashStringToInt64(s string) int64 {
	var h int64
	for _, c := range s {
		h = 31*h + int64(c)
	}
	return h
}

// GenerateAndSave generates a photo for a persona and saves it with metadata tracking.
// Returns the photo metadata and the path to the saved image.
func GenerateAndSave(
	ctx context.Context,
	generator *PhotoGenerator,
	metadataStore *MetadataStore,
	personSeed seed.PersonSeed,
	personaType string,
	personaID string,
) (*PhotoMetadata, string, error) {
	// Check for existing match to enable reuse
	if match := metadataStore.FindMatch(personSeed); match != nil {
		// Reuse existing photo
		if err := metadataStore.MarkInUse(match.ID, personaID); err != nil {
			return nil, "", fmt.Errorf("mark in use: %w", err)
		}
		imagePath := generator.outputDir + "/" + match.Filename
		return match, imagePath, nil
	}

	// Generate new photo
	metadata, err := generator.Generate(ctx, personSeed)
	if err != nil {
		return nil, "", fmt.Errorf("generate photo: %w", err)
	}

	// Mark as in use
	metadata.InUseBy = append(metadata.InUseBy, personaID)

	// Save metadata
	if err := metadataStore.Add(*metadata); err != nil {
		return nil, "", fmt.Errorf("save metadata: %w", err)
	}

	imagePath := generator.outputDir + "/" + metadata.Filename
	return metadata, imagePath, nil
}

