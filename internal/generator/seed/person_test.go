package seed

import (
	"strings"
	"testing"
)

func TestNewPersonSeed_AgeWithinRoleRange(t *testing.T) {
	testCases := []struct {
		role   RoleLevel
		minAge int
		maxAge int
	}{
		{RoleSupportL1, 22, 35},
		{RoleSupportL2, 26, 42},
		{RoleSupportL3, 30, 50},
		{RoleManager, 35, 55},
		{RoleDirector, 40, 60},
		{RoleExecutive, 48, 65},
	}

	for _, tc := range testCases {
		t.Run(tc.role.String(), func(t *testing.T) {
			// Generate multiple samples to verify range
			for i := 0; i < 50; i++ {
				seed := NewPersonSeed(tc.role)
				if seed.Age < tc.minAge || seed.Age > tc.maxAge {
					t.Errorf("Age %d outside expected range [%d, %d] for role %s",
						seed.Age, tc.minAge, tc.maxAge, tc.role)
				}
			}
		})
	}
}

func TestNewPersonSeed_HairColorCorrelatesWithEthnicity(t *testing.T) {
	// Map of valid hair colors per ethnicity
	validHairColors := map[string][]string{
		"Asian":    {"Black", "Brown", "Gray"},
		"Black":    {"Black", "Brown", "Gray"},
		"Hispanic": {"Black", "Brown", "Gray"},
		"White":    {"Blonde", "Brown", "Black", "Red", "Gray"},
	}

	// Generate many samples and verify correlation
	for i := 0; i < 200; i++ {
		seed := NewPersonSeed(RoleSupportL1)

		validColors, ok := validHairColors[seed.Ethnicity]
		if !ok {
			t.Errorf("Unexpected ethnicity: %s", seed.Ethnicity)
			continue
		}

		found := false
		for _, c := range validColors {
			if seed.HairColor == c {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Hair color %q not valid for ethnicity %s (expected one of %v)",
				seed.HairColor, seed.Ethnicity, validColors)
		}
	}
}

func TestNewPersonSeed_BuildExcludesExtremes(t *testing.T) {
	validBuilds := map[string]bool{
		"slim":     true,
		"average":  true,
		"athletic": true,
		"stocky":   true,
	}

	extremeBuilds := []string{"obese", "underweight", "morbidly obese", "anorexic"}

	for i := 0; i < 100; i++ {
		seed := NewPersonSeed(RoleSupportL1)

		if !validBuilds[seed.Build] {
			t.Errorf("Invalid build type: %s", seed.Build)
		}

		for _, extreme := range extremeBuilds {
			if strings.EqualFold(seed.Build, extreme) {
				t.Errorf("Build should not include extreme: %s", seed.Build)
			}
		}
	}
}

func TestNewPersonSeed_FacialHairMaleOnly(t *testing.T) {
	for i := 0; i < 100; i++ {
		seed := NewPersonSeed(RoleSupportL1)

		if strings.EqualFold(seed.Gender, "female") && seed.FacialHair != "" {
			t.Errorf("Female should not have facial hair, got: %s", seed.FacialHair)
		}

		if strings.EqualFold(seed.Gender, "male") {
			validFacialHair := map[string]bool{
				"clean-shaven": true,
				"stubble":      true,
				"goatee":       true,
				"beard":        true,
			}
			if !validFacialHair[seed.FacialHair] {
				t.Errorf("Invalid facial hair for male: %s", seed.FacialHair)
			}
		}
	}
}

func TestNewPersonSeed_Diversity(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		seed := NewPersonSeed(RoleSupportL1)
		key := seed.FirstName + "|" + seed.LastName + "|" + seed.Ethnicity + "|" + seed.HairColor
		seen[key] = true
	}

	// Should have significant diversity (at least 80 unique combinations out of 100)
	if len(seen) < 80 {
		t.Errorf("Insufficient diversity: only %d unique combinations out of 100", len(seen))
	}
}

func TestNewPersonSeed_GlassesProbability(t *testing.T) {
	glassesCount := 0
	samples := 1000

	for i := 0; i < samples; i++ {
		seed := NewPersonSeed(RoleSupportL1)
		if seed.Glasses {
			glassesCount++
		}
	}

	// Expect roughly 30% with glasses (allow 20%-40% range for randomness)
	percentage := float64(glassesCount) / float64(samples) * 100
	if percentage < 20 || percentage > 40 {
		t.Errorf("Glasses probability %.1f%% outside expected range [20%%, 40%%]", percentage)
	}
}

func TestPersonSeed_ToPromptFragment(t *testing.T) {
	seed := PersonSeed{
		FirstName:   "John",
		LastName:    "Smith",
		Gender:      "Male",
		Age:         35,
		City:        "Austin",
		State:       "Texas",
		Ethnicity:   "White",
		HairColor:   "Brown",
		HairStyle:   "short",
		HairTexture: "straight",
		FacialHair:  "beard",
		EyeColor:    "Blue",
		Glasses:     true,
		Build:       "athletic",
	}

	fragment := seed.ToPromptFragment()

	// Verify key elements are present
	expectedParts := []string{
		"John Smith",
		"35-year-old",
		"White",
		"male",
		"Austin, Texas",
		"athletic build",
		"straight short brown hair",
		"blue eyes",
		"wears glasses",
		"has beard",
	}

	for _, part := range expectedParts {
		if !strings.Contains(fragment, part) {
			t.Errorf("ToPromptFragment() missing expected part %q, got: %s", part, fragment)
		}
	}
}

