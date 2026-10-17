// Package conv holds the small value conversions the modules share: reading
// an optional field and narrowing a number to the API's int32.
package conv

import "math"

// OrZero returns *p, or the zero value when p is nil.
func OrZero[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// Int32 narrows n to int32, clamping at both ends. The numbers it carries
// (rules tables, validated sheets, row counts) are small, so the clamp never
// changes a real value; it keeps the conversion provably safe.
func Int32[T ~int | ~int64](n T) int32 {
	v := int64(n)
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
