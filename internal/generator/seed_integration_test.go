package generator_test

import (
	"strings"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
)

// TestPersonSeedDiversity verifies that generating multiple seeds produces diverse output.
func TestPersonSeedDiversity(t *testing.T) {
	const sampleSize = 10

	seeds := make([]seed.PersonSeed, sampleSize)
	for i := range seeds {
		seeds[i] = seed.NewPersonSeed(seed.RoleSupportL1)
	}

	// Count unique names
	names := make(map[string]bool)
	cities := make(map[string]bool)
	states := make(map[string]bool)

	for _, s := range seeds {
		names[s.FirstName+" "+s.LastName] = true
		cities[s.City] = true
		states[s.State] = true
	}

	// Should have significant diversity
	if len(names) < 8 {
		t.Errorf("Insufficient name diversity: %d unique names out of %d (want at least 8)", len(names), sampleSize)
		t.Logf("Names: %v", names)
	}
	if len(cities) < 3 {
		t.Errorf("Insufficient city diversity: %d unique cities out of %d (want at least 3)", len(cities), sampleSize)
	}
	if len(states) < 3 {
		t.Errorf("Insufficient state diversity: %d unique states out of %d (want at least 3)", len(states), sampleSize)
	}

	t.Logf("Diversity metrics: %d names, %d cities, %d states", len(names), len(cities), len(states))
}

// TestHairColorEthnicityCorrelation verifies hair colors correlate with ethnicity.
func TestHairColorEthnicityCorrelation(t *testing.T) {
	const sampleSize = 20

	// Track hair colors by ethnicity
	hairByEthnicity := make(map[string]map[string]int)
	for _, eth := range []string{"Asian", "Black", "Hispanic", "White"} {
		hairByEthnicity[eth] = make(map[string]int)
	}

	for i := 0; i < sampleSize; i++ {
		s := seed.NewPersonSeed(seed.RoleSupportL1)
		if _, ok := hairByEthnicity[s.Ethnicity]; ok {
			hairByEthnicity[s.Ethnicity][s.HairColor]++
		}
	}

	// Verify Asian ethnicity has predominantly dark hair (Black/Brown/Gray)
	asianHair := hairByEthnicity["Asian"]
	darkHairCount := asianHair["Black"] + asianHair["Brown"] + asianHair["Gray"]
	if len(asianHair) > 0 {
		for color := range asianHair {
			if color == "Blonde" || color == "Red" {
				t.Errorf("Asian ethnicity should not have %s hair", color)
			}
		}
		t.Logf("Asian hair distribution: %v (dark hair: %d)", asianHair, darkHairCount)
	}

	// Verify Black ethnicity has predominantly dark hair
	blackHair := hairByEthnicity["Black"]
	if len(blackHair) > 0 {
		for color := range blackHair {
			if color == "Blonde" || color == "Red" {
				t.Errorf("Black ethnicity should not have %s hair", color)
			}
		}
		t.Logf("Black ethnicity hair distribution: %v", blackHair)
	}

	// White ethnicity can have any hair color (verify diversity)
	whiteHair := hairByEthnicity["White"]
	if len(whiteHair) > 0 {
		t.Logf("White ethnicity hair distribution: %v", whiteHair)
	}
}

// TestAgeDistributionByRole verifies role levels produce appropriate age ranges.
func TestAgeDistributionByRole(t *testing.T) {
	testCases := []struct {
		role   seed.RoleLevel
		minAge int
		maxAge int
		name   string
	}{
		{seed.RoleSupportL1, 22, 35, "SupportL1"},
		{seed.RoleManager, 35, 55, "Manager"},
		{seed.RoleDirector, 40, 60, "Director"},
		{seed.RoleExecutive, 48, 65, "Executive"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ages []int
			for i := 0; i < 20; i++ {
				s := seed.NewPersonSeed(tc.role)
				ages = append(ages, s.Age)

				if s.Age < tc.minAge || s.Age > tc.maxAge {
					t.Errorf("Age %d outside expected range [%d, %d] for %s",
						s.Age, tc.minAge, tc.maxAge, tc.name)
				}
			}

			// Calculate average age
			sum := 0
			for _, a := range ages {
				sum += a
			}
			avgAge := float64(sum) / float64(len(ages))

			// Average should be roughly in the middle of the range
			expectedAvg := float64(tc.minAge+tc.maxAge) / 2
			if avgAge < float64(tc.minAge) || avgAge > float64(tc.maxAge) {
				t.Errorf("Average age %.1f outside range [%d, %d]", avgAge, tc.minAge, tc.maxAge)
			}
			t.Logf("%s average age: %.1f (expected ~%.1f)", tc.name, avgAge, expectedAvg)
		})
	}
}

// TestCompanySeedDiversity verifies company seeds have diverse locations.
func TestCompanySeedDiversity(t *testing.T) {
	const sampleSize = 10

	seeds := make([]seed.CompanySeed, sampleSize)
	for i := range seeds {
		seeds[i] = seed.NewCompanySeed()
	}

	cities := make(map[string]bool)
	states := make(map[string]bool)
	suffixes := make(map[string]bool)

	for _, s := range seeds {
		cities[s.City] = true
		states[s.State] = true
		suffixes[s.Suffix] = true
	}

	if len(cities) < 3 {
		t.Errorf("Insufficient city diversity: %d unique cities out of %d", len(cities), sampleSize)
	}
	if len(states) < 3 {
		t.Errorf("Insufficient state diversity: %d unique states out of %d", len(states), sampleSize)
	}
	if len(suffixes) < 2 {
		t.Errorf("Insufficient suffix diversity: %d unique suffixes out of %d", len(suffixes), sampleSize)
	}

	t.Logf("Company diversity: %d cities, %d states, %d suffixes", len(cities), len(states), len(suffixes))
}

// TestCompanySeedRanges verifies company seed values are within expected ranges.
func TestCompanySeedRanges(t *testing.T) {
	for i := 0; i < 20; i++ {
		s := seed.NewCompanySeed()

		if s.FoundedYear < 1970 || s.FoundedYear > 2015 {
			t.Errorf("FoundedYear %d outside range [1970, 2015]", s.FoundedYear)
		}
		if s.EmployeeCount < 50 || s.EmployeeCount > 10000 {
			t.Errorf("EmployeeCount %d outside range [50, 10000]", s.EmployeeCount)
		}
	}
}

// TestEthnicityDistribution verifies ethnicities are distributed across the population.
func TestEthnicityDistribution(t *testing.T) {
	const sampleSize = 100

	ethnicities := make(map[string]int)
	for i := 0; i < sampleSize; i++ {
		s := seed.NewPersonSeed(seed.RoleSupportL1)
		ethnicities[s.Ethnicity]++
	}

	// Should have all 4 ethnicities represented
	expectedEthnicities := []string{"Asian", "Black", "Hispanic", "White"}
	for _, eth := range expectedEthnicities {
		count, ok := ethnicities[eth]
		if !ok || count == 0 {
			t.Errorf("Missing ethnicity: %s", eth)
		}
	}

	t.Logf("Ethnicity distribution (n=%d): %v", sampleSize, ethnicities)
}

// TestGenderDistribution verifies genders are distributed.
func TestGenderDistribution(t *testing.T) {
	const sampleSize = 100

	genders := make(map[string]int)
	for i := 0; i < sampleSize; i++ {
		s := seed.NewPersonSeed(seed.RoleSupportL1)
		genders[strings.ToLower(s.Gender)]++
	}

	// Should have both male and female represented (gofakeit returns lowercase)
	maleCount := genders["male"]
	femaleCount := genders["female"]

	if maleCount == 0 {
		t.Error("No male seeds generated")
	}
	if femaleCount == 0 {
		t.Error("No female seeds generated")
	}

	// Expect roughly even distribution (allow 30-70% range)
	malePercent := float64(maleCount) / float64(sampleSize) * 100
	if malePercent < 30 || malePercent > 70 {
		t.Errorf("Uneven gender distribution: %.1f%% male (expected 30-70%%)", malePercent)
	}

	t.Logf("Gender distribution (n=%d): %v", sampleSize, genders)
}

