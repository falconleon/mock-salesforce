// Package seed provides faker-generated random data to seed variety
// in each entity while letting the LLM generate contextual content.
package seed

import (
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// SetSeed sets the global random seed for deterministic trait generation.
// Use this when you need reproducible PersonSeed generation from a known ID.
func SetSeed(seed int64) {
	gofakeit.Seed(seed)
}

// RoleLevel represents the seniority level of a support team member.
type RoleLevel int

const (
	RoleSupportL1 RoleLevel = iota
	RoleSupportL2
	RoleSupportL3
	RoleManager
	RoleDirector
	RoleExecutive
)

// String returns the human-readable name of the role level.
func (r RoleLevel) String() string {
	switch r {
	case RoleSupportL1:
		return "Support L1"
	case RoleSupportL2:
		return "Support L2"
	case RoleSupportL3:
		return "Support L3"
	case RoleManager:
		return "Manager"
	case RoleDirector:
		return "Director"
	case RoleExecutive:
		return "Executive"
	default:
		return "Unknown"
	}
}

// ageRange returns min and max age for a role level.
func (r RoleLevel) ageRange() (min, max int) {
	switch r {
	case RoleSupportL1:
		return 22, 35
	case RoleSupportL2:
		return 26, 42
	case RoleSupportL3:
		return 30, 50
	case RoleManager:
		return 35, 55
	case RoleDirector:
		return 40, 60
	case RoleExecutive:
		return 48, 65
	default:
		return 25, 45
	}
}

// PersonSeed contains faker-generated traits for a person.
type PersonSeed struct {
	FirstName   string
	LastName    string
	Gender      string
	Age         int
	City        string
	State       string
	Ethnicity   string
	HairColor   string
	HairStyle   string
	HairTexture string
	FacialHair  string // male only
	EyeColor    string
	Glasses     bool
	Build       string
}

// weightedOption represents an option with a weight for random selection.
type weightedOption struct {
	Value  string
	Weight int
}

// hairColorByEthnicity defines weighted hair color distributions.
var hairColorByEthnicity = map[string][]weightedOption{
	"Asian":    {{"Black", 80}, {"Brown", 15}, {"Gray", 5}},
	"Black":    {{"Black", 90}, {"Brown", 5}, {"Gray", 5}},
	"Hispanic": {{"Black", 70}, {"Brown", 25}, {"Gray", 5}},
	"White":    {{"Blonde", 25}, {"Brown", 40}, {"Black", 15}, {"Red", 10}, {"Gray", 10}},
}

var hairStyles = []string{"short", "medium", "long", "bald"}
var hairTextures = []string{"straight", "wavy", "curly"}
var facialHairStyles = []string{"clean-shaven", "stubble", "goatee", "beard"}
var buildTypes = []string{"slim", "average", "athletic", "stocky"}

// selectWeighted selects a value based on weights.
func selectWeighted(options []weightedOption) string {
	totalWeight := 0
	for _, opt := range options {
		totalWeight += opt.Weight
	}
	r := gofakeit.Number(1, totalWeight)
	cumulative := 0
	for _, opt := range options {
		cumulative += opt.Weight
		if r <= cumulative {
			return opt.Value
		}
	}
	return options[0].Value
}

// NewPersonSeed generates a new PersonSeed with traits appropriate for the role level.
func NewPersonSeed(role RoleLevel) PersonSeed {
	gender := gofakeit.Gender()
	ethnicity := gofakeit.RandomString([]string{"Asian", "Black", "Hispanic", "White"})

	minAge, maxAge := role.ageRange()
	age := gofakeit.Number(minAge, maxAge)

	// Get hair color based on ethnicity
	hairColorOpts, ok := hairColorByEthnicity[ethnicity]
	if !ok {
		hairColorOpts = hairColorByEthnicity["White"]
	}
	hairColor := selectWeighted(hairColorOpts)

	// Determine facial hair (male only)
	facialHair := ""
	if strings.EqualFold(gender, "male") {
		facialHair = gofakeit.RandomString(facialHairStyles)
	}

	// 30% probability for glasses
	glasses := gofakeit.Number(1, 100) <= 30

	return PersonSeed{
		FirstName:   gofakeit.FirstName(),
		LastName:    gofakeit.LastName(),
		Gender:      gender,
		Age:         age,
		City:        gofakeit.City(),
		State:       gofakeit.State(),
		Ethnicity:   ethnicity,
		HairColor:   hairColor,
		HairStyle:   gofakeit.RandomString(hairStyles),
		HairTexture: gofakeit.RandomString(hairTextures),
		FacialHair:  facialHair,
		EyeColor:    gofakeit.RandomString([]string{"Brown", "Blue", "Green", "Hazel", "Gray"}),
		Glasses:     glasses,
		Build:       gofakeit.RandomString(buildTypes),
	}
}

// ToPromptFragment returns a string describing the person's traits for LLM prompts.
func (p PersonSeed) ToPromptFragment() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s %s", p.FirstName, p.LastName))
	parts = append(parts, fmt.Sprintf("%d-year-old %s %s", p.Age, p.Ethnicity, strings.ToLower(p.Gender)))
	parts = append(parts, fmt.Sprintf("from %s, %s", p.City, p.State))
	parts = append(parts, fmt.Sprintf("%s build", p.Build))
	parts = append(parts, fmt.Sprintf("%s %s %s hair", p.HairTexture, p.HairStyle, strings.ToLower(p.HairColor)))
	parts = append(parts, fmt.Sprintf("%s eyes", strings.ToLower(p.EyeColor)))

	if p.Glasses {
		parts = append(parts, "wears glasses")
	}

	if p.FacialHair != "" && p.FacialHair != "clean-shaven" {
		parts = append(parts, fmt.Sprintf("has %s", p.FacialHair))
	}

	return strings.Join(parts, ", ")
}

