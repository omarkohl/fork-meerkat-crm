// Package idutil converts untrusted numbers (JWT claims, URL parameters) to the
// uint IDs the database uses. Every conversion is range-checked here, so callers
// need no lint suppressions.
package idutil

import (
	"errors"
	"math"
	"strconv"
)

// FromFloat converts a JSON number to an ID. Valid IDs are whole numbers from 1 to
// MaxUint32.
func FromFloat(f float64) (uint, bool) {
	if math.IsNaN(f) || f < 1 || f > math.MaxUint32 || f != math.Trunc(f) {
		return 0, false
	}
	return uint(f), true
}

// Parse converts a decimal string (a URL parameter) to an ID from 1 to MaxUint32.
func Parse(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errors.New("id must be positive")
	}
	return uint(n), nil
}
