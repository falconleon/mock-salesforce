package seed

import (
	"github.com/brianvoe/gofakeit/v7"
)

// CompanySeed contains faker-generated traits for a company.
type CompanySeed struct {
	Suffix        string // Inc, LLC, Corp, Ltd
	City          string
	State         string
	Zip           string
	FoundedYear   int // 1970-2015
	EmployeeCount int // 50-10000
}

var companySuffixes = []string{"Inc", "LLC", "Corp", "Ltd", "Co", "Group"}

// NewCompanySeed generates a new CompanySeed with random company traits.
func NewCompanySeed() CompanySeed {
	return CompanySeed{
		Suffix:        gofakeit.RandomString(companySuffixes),
		City:          gofakeit.City(),
		State:         gofakeit.State(),
		Zip:           gofakeit.Zip(),
		FoundedYear:   gofakeit.Number(1970, 2015),
		EmployeeCount: gofakeit.Number(50, 10000),
	}
}

