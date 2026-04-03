package seed

import (
	"testing"
)

func TestNewCompanySeed_FieldsPopulated(t *testing.T) {
	seed := NewCompanySeed()

	if seed.Suffix == "" {
		t.Error("Suffix should not be empty")
	}
	if seed.City == "" {
		t.Error("City should not be empty")
	}
	if seed.State == "" {
		t.Error("State should not be empty")
	}
	if seed.Zip == "" {
		t.Error("Zip should not be empty")
	}
}

func TestNewCompanySeed_FoundedYearRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		seed := NewCompanySeed()
		if seed.FoundedYear < 1970 || seed.FoundedYear > 2015 {
			t.Errorf("FoundedYear %d outside expected range [1970, 2015]", seed.FoundedYear)
		}
	}
}

func TestNewCompanySeed_EmployeeCountRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		seed := NewCompanySeed()
		if seed.EmployeeCount < 50 || seed.EmployeeCount > 10000 {
			t.Errorf("EmployeeCount %d outside expected range [50, 10000]", seed.EmployeeCount)
		}
	}
}

func TestNewCompanySeed_ValidSuffixes(t *testing.T) {
	validSuffixes := map[string]bool{
		"Inc":   true,
		"LLC":   true,
		"Corp":  true,
		"Ltd":   true,
		"Co":    true,
		"Group": true,
	}

	for i := 0; i < 100; i++ {
		seed := NewCompanySeed()
		if !validSuffixes[seed.Suffix] {
			t.Errorf("Invalid suffix: %s", seed.Suffix)
		}
	}
}

func TestNewCompanySeed_Diversity(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		seed := NewCompanySeed()
		key := seed.City + "|" + seed.State + "|" + seed.Suffix
		seen[key] = true
	}

	// Should have significant diversity (at least 50 unique combinations out of 100)
	if len(seen) < 50 {
		t.Errorf("Insufficient diversity: only %d unique combinations out of 100", len(seen))
	}
}

