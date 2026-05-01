package soql

import (
	"fmt"
	"time"
)

// DateLiteral represents a SOQL date literal such as TODAY, YESTERDAY, or
// LAST_N_DAYS:30. It evaluates to a half-open time range [Start, End) at
// query execution time.
type DateLiteral struct {
	Name string // canonical upper-case name, e.g. "TODAY", "LAST_N_DAYS"
	N    int    // parameter for parameterized literals (e.g. 30 for LAST_N_DAYS:30)
}

// String returns the source-form representation.
func (d DateLiteral) String() string {
	if isParamDateLiteral(d.Name) {
		return fmt.Sprintf("%s:%d", d.Name, d.N)
	}
	return d.Name
}

// simpleDateLiterals is the set of date literals that take no parameter.
var simpleDateLiterals = map[string]struct{}{
	"YESTERDAY":     {},
	"TODAY":         {},
	"TOMORROW":      {},
	"LAST_WEEK":     {},
	"THIS_WEEK":     {},
	"NEXT_WEEK":     {},
	"LAST_MONTH":    {},
	"THIS_MONTH":    {},
	"NEXT_MONTH":    {},
	"LAST_QUARTER":  {},
	"THIS_QUARTER":  {},
	"NEXT_QUARTER":  {},
	"LAST_YEAR":     {},
	"THIS_YEAR":     {},
	"NEXT_YEAR":     {},
	"LAST_90_DAYS":  {},
	"NEXT_90_DAYS":  {},
}

// paramDateLiterals is the set of date literals that require a `:N` parameter.
var paramDateLiterals = map[string]struct{}{
	"LAST_N_DAYS":     {},
	"NEXT_N_DAYS":     {},
	"N_DAYS_AGO":      {},
	"LAST_N_WEEKS":    {},
	"NEXT_N_WEEKS":    {},
	"N_WEEKS_AGO":     {},
	"LAST_N_MONTHS":   {},
	"NEXT_N_MONTHS":   {},
	"N_MONTHS_AGO":    {},
	"LAST_N_QUARTERS": {},
	"NEXT_N_QUARTERS": {},
	"N_QUARTERS_AGO":  {},
	"LAST_N_YEARS":    {},
	"NEXT_N_YEARS":    {},
	"N_YEARS_AGO":     {},
}

func isSimpleDateLiteral(name string) bool {
	_, ok := simpleDateLiterals[name]
	return ok
}

func isParamDateLiteral(name string) bool {
	_, ok := paramDateLiterals[name]
	return ok
}

// IsDateLiteralName reports whether name (upper-case) is a recognized date literal.
func IsDateLiteralName(name string) bool {
	return isSimpleDateLiteral(name) || isParamDateLiteral(name)
}

// Range returns the half-open time range [start, end) covered by this literal,
// relative to now. now is the reference moment used to anchor "today".
func (d DateLiteral) Range(now time.Time) (start, end time.Time) {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	switch d.Name {
	case "YESTERDAY":
		return today.AddDate(0, 0, -1), today
	case "TODAY":
		return today, today.AddDate(0, 0, 1)
	case "TOMORROW":
		return today.AddDate(0, 0, 1), today.AddDate(0, 0, 2)
	case "LAST_WEEK":
		s := startOfWeek(today).AddDate(0, 0, -7)
		return s, s.AddDate(0, 0, 7)
	case "THIS_WEEK":
		s := startOfWeek(today)
		return s, s.AddDate(0, 0, 7)
	case "NEXT_WEEK":
		s := startOfWeek(today).AddDate(0, 0, 7)
		return s, s.AddDate(0, 0, 7)
	case "LAST_MONTH":
		s := startOfMonth(today).AddDate(0, -1, 0)
		return s, s.AddDate(0, 1, 0)
	case "THIS_MONTH":
		s := startOfMonth(today)
		return s, s.AddDate(0, 1, 0)
	case "NEXT_MONTH":
		s := startOfMonth(today).AddDate(0, 1, 0)
		return s, s.AddDate(0, 1, 0)
	case "LAST_QUARTER":
		s := startOfQuarter(today).AddDate(0, -3, 0)
		return s, s.AddDate(0, 3, 0)
	case "THIS_QUARTER":
		s := startOfQuarter(today)
		return s, s.AddDate(0, 3, 0)
	case "NEXT_QUARTER":
		s := startOfQuarter(today).AddDate(0, 3, 0)
		return s, s.AddDate(0, 3, 0)
	case "LAST_YEAR":
		s := time.Date(today.Year()-1, 1, 1, 0, 0, 0, 0, loc)
		return s, s.AddDate(1, 0, 0)
	case "THIS_YEAR":
		s := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, loc)
		return s, s.AddDate(1, 0, 0)
	case "NEXT_YEAR":
		s := time.Date(today.Year()+1, 1, 1, 0, 0, 0, 0, loc)
		return s, s.AddDate(1, 0, 0)
	case "LAST_90_DAYS":
		return today.AddDate(0, 0, -90), today.AddDate(0, 0, 1)
	case "NEXT_90_DAYS":
		return today.AddDate(0, 0, 1), today.AddDate(0, 0, 91)
	case "LAST_N_DAYS":
		return today.AddDate(0, 0, -d.N), today.AddDate(0, 0, 1)
	case "NEXT_N_DAYS":
		return today.AddDate(0, 0, 1), today.AddDate(0, 0, d.N+1)
	case "N_DAYS_AGO":
		s := today.AddDate(0, 0, -d.N)
		return s, s.AddDate(0, 0, 1)
	case "LAST_N_WEEKS":
		s := startOfWeek(today).AddDate(0, 0, -7*d.N)
		return s, startOfWeek(today)
	case "NEXT_N_WEEKS":
		s := startOfWeek(today).AddDate(0, 0, 7)
		return s, s.AddDate(0, 0, 7*d.N)
	case "N_WEEKS_AGO":
		s := startOfWeek(today).AddDate(0, 0, -7*d.N)
		return s, s.AddDate(0, 0, 7)
	case "LAST_N_MONTHS":
		s := startOfMonth(today).AddDate(0, -d.N, 0)
		return s, startOfMonth(today)
	case "NEXT_N_MONTHS":
		s := startOfMonth(today).AddDate(0, 1, 0)
		return s, s.AddDate(0, d.N, 0)
	case "N_MONTHS_AGO":
		s := startOfMonth(today).AddDate(0, -d.N, 0)
		return s, s.AddDate(0, 1, 0)
	}
	return d.rangeQuarterYear(today, loc)
}

func (d DateLiteral) rangeQuarterYear(today time.Time, loc *time.Location) (time.Time, time.Time) {
	switch d.Name {
	case "LAST_N_QUARTERS":
		s := startOfQuarter(today).AddDate(0, -3*d.N, 0)
		return s, startOfQuarter(today)
	case "NEXT_N_QUARTERS":
		s := startOfQuarter(today).AddDate(0, 3, 0)
		return s, s.AddDate(0, 3*d.N, 0)
	case "N_QUARTERS_AGO":
		s := startOfQuarter(today).AddDate(0, -3*d.N, 0)
		return s, s.AddDate(0, 3, 0)
	case "LAST_N_YEARS":
		s := time.Date(today.Year()-d.N, 1, 1, 0, 0, 0, 0, loc)
		return s, time.Date(today.Year(), 1, 1, 0, 0, 0, 0, loc)
	case "NEXT_N_YEARS":
		s := time.Date(today.Year()+1, 1, 1, 0, 0, 0, 0, loc)
		return s, time.Date(today.Year()+1+d.N, 1, 1, 0, 0, 0, 0, loc)
	case "N_YEARS_AGO":
		s := time.Date(today.Year()-d.N, 1, 1, 0, 0, 0, 0, loc)
		return s, s.AddDate(1, 0, 0)
	}
	// Unknown literal: return a zero range; callers treat zero as no match.
	return time.Time{}, time.Time{}
}

// startOfWeek returns the most recent Sunday at 00:00:00 (Salesforce default).
func startOfWeek(t time.Time) time.Time {
	wd := int(t.Weekday()) // Sunday=0
	return t.AddDate(0, 0, -wd)
}

// startOfMonth returns the first day of t's month at 00:00:00.
func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// startOfQuarter returns the first day of t's calendar quarter at 00:00:00.
func startOfQuarter(t time.Time) time.Time {
	q := (int(t.Month())-1)/3*3 + 1
	return time.Date(t.Year(), time.Month(q), 1, 0, 0, 0, 0, t.Location())
}

// parseFieldTime tries to parse a record field value as a time.Time. Returns
// false if the value isn't a recognized date/datetime string.
func parseFieldTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

