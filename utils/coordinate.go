package utils

import "math"

// IsValidCoordinate memvalidasi bahwa latitude berada dalam [-90, 90] dan longitude dalam [-180, 180],
// serta bukan merupakan nilai NaN atau Infinity.
func IsValidCoordinate(lat, lng float64) bool {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return false
	}
	if math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return false
	}
	return true
}
