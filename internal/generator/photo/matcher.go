package photo

import (
	"strings"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

const (
	// MatchThreshold is the minimum score required for a valid match.
	MatchThreshold = 60

	// Weight for required traits (must match).
	weightGender    = 30
	weightEthnicity = 30

	// Weight for important traits (should match).
	weightAgeRange  = 15
	weightHairColor = 10
	weightGlasses   = 5

	// Weight for nice-to-have traits.
	weightHairStyle = 5
	weightBuild     = 5
)

// FindMatch searches for a reusable photo matching the given person traits.
// Returns nil if no suitable match is found (score < threshold).
// Prefers photos with fewer InUseBy entries.
func (s *MetadataStore) FindMatch(personSeed seed.PersonSeed) *PhotoMetadata {
	metadata, err := s.Load()
	if err != nil || len(metadata) == 0 {
		return nil
	}

	var bestMatch *PhotoMetadata
	bestScore := 0

	targetAgeRange := ageToRange(personSeed.Age)

	for i := range metadata {
		photo := &metadata[i]
		score := calculateMatchScore(photo, personSeed, targetAgeRange)

		if score < MatchThreshold {
			continue
		}

		// Prefer photos with fewer users (better diversity)
		// Tie-breaker: use InUseBy count
		if score > bestScore || (score == bestScore && (bestMatch == nil || len(photo.InUseBy) < len(bestMatch.InUseBy))) {
			bestScore = score
			bestMatch = photo
		}
	}

	return bestMatch
}

// calculateMatchScore computes a weighted score for how well a photo matches a person.
func calculateMatchScore(photo *PhotoMetadata, person seed.PersonSeed, targetAgeRange string) int {
	score := 0

	// Required traits (must match for consideration)
	if !strings.EqualFold(photo.Gender, person.Gender) {
		return 0 // Gender must match
	}
	score += weightGender

	if !strings.EqualFold(photo.Ethnicity, person.Ethnicity) {
		return 0 // Ethnicity must match
	}
	score += weightEthnicity

	// Important traits
	if photo.AgeRange == targetAgeRange {
		score += weightAgeRange
	} else if isAdjacentAgeRange(photo.AgeRange, targetAgeRange) {
		score += weightAgeRange / 2 // Partial credit for adjacent range
	}

	if strings.EqualFold(photo.HairColor, person.HairColor) {
		score += weightHairColor
	}

	if photo.Glasses == person.Glasses {
		score += weightGlasses
	}

	// Nice-to-have traits
	if strings.EqualFold(photo.HairStyle, person.HairStyle) {
		score += weightHairStyle
	}

	if strings.EqualFold(photo.Build, person.Build) {
		score += weightBuild
	}

	return score
}

// ageToRange converts an age to a decade range string.
func ageToRange(age int) string {
	switch {
	case age < 20:
		return "under-20"
	case age < 30:
		return "20-30"
	case age < 40:
		return "30-40"
	case age < 50:
		return "40-50"
	case age < 60:
		return "50-60"
	default:
		return "60+"
	}
}

// isAdjacentAgeRange checks if two age ranges are adjacent (within 10 years).
func isAdjacentAgeRange(a, b string) bool {
	ranges := []string{"under-20", "20-30", "30-40", "40-50", "50-60", "60+"}

	indexA, indexB := -1, -1
	for i, r := range ranges {
		if r == a {
			indexA = i
		}
		if r == b {
			indexB = i
		}
	}

	if indexA < 0 || indexB < 0 {
		return false
	}

	diff := indexA - indexB
	return diff == 1 || diff == -1
}

