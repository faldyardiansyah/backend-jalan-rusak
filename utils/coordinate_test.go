package utils

import (
	"math"
	"testing"
)

func TestCoordinateValidation_BoundaryAndInvalidCases(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		lng      float64
		expected bool
	}{
		// Valid cases & boundaries
		{"Latitude 90 valid boundary", 90.0, 108.0, true},
		{"Latitude -90 valid boundary", -90.0, 108.0, true},
		{"Longitude 180 valid boundary", -6.32, 180.0, true},
		{"Longitude -180 valid boundary", -6.32, -180.0, true},
		{"Standard Indramayu coordinate", -6.3265, 108.3241, true},
		{"Zero coordinate", 0.0, 0.0, true},

		// Invalid cases - Out of range
		{"Latitude 90.1 invalid (too high)", 90.1, 108.0, false},
		{"Latitude -90.1 invalid (too low)", -90.1, 108.0, false},
		{"Longitude 180.1 invalid (too high)", -6.32, 180.1, false},
		{"Longitude -180.1 invalid (too low)", -6.32, -180.1, false},

		// Invalid cases - NaN & Inf
		{"Latitude NaN", math.NaN(), 108.0, false},
		{"Longitude NaN", -6.32, math.NaN(), false},
		{"Both NaN", math.NaN(), math.NaN(), false},
		{"Latitude Positive Inf", math.Inf(1), 108.0, false},
		{"Latitude Negative Inf", math.Inf(-1), 108.0, false},
		{"Longitude Positive Inf", -6.32, math.Inf(1), false},
		{"Longitude Negative Inf", -6.32, math.Inf(-1), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidCoordinate(tc.lat, tc.lng)
			if result != tc.expected {
				t.Errorf("[%s] lat=%v, lng=%v: expected %v, got %v", tc.name, tc.lat, tc.lng, tc.expected, result)
			}
		})
	}
}
