//go:build arm64

package op

// Magic constants for date operations.
// See Howard Hinnant's scalar implementations, which I just copied
// and vectorized:
// https://howardhinnant.github.io/date_algorithms.html
var dateScalarConstants = []float32{
	1,      // v0
	2,      // v1
	3,      // v2
	4,      // v3
	5,      // v4
	10,     // v5
	12,     // v6
	100,    // v7
	153,    // v8
	365,    // v9
	400,    // v10
	1460,   // v11
	36524,  // v12
	146096, // v13
	146097, // v14
	719468, // v15
}

// ExtYear extracts the year from each date in `src`, and places the
// result in `dst`.
func ExYear(src, dst []int32) {
	extractYear(src, dst, dateScalarConstants)
}

// ExttMonth extracts the month from each date in `src`, and places the
// result in `dst`.
func ExtMonth(src, dst []int32) {
	extractMonth(src, dst, dateScalarConstants)
}

// ExtDay extracts the day from each date in `src`, and places the
// result in `dst`.
func ExtDay(src, dst []int32) {
	extractDay(src, dst, dateScalarConstants)
}

// ExtDayOfWeek extracts the day of the week from each date in `src`, and places the
// result in `dst`.
func ExtDayOfWeek(src, dst []int32) {
	extractDayOfWeek(src, dst)
}

// TruncYear each date in `src` to its year (e.g., "2026-03-30" => "2026-01-01"),
// and places the result in `dst`.
func TruncYear(src, dst []int32) {
	truncateYear(src, dst, dateScalarConstants)
}

// TruncMonth each date in `src` to its month (e.g., "2026-03-30" => "2026-03-01"),
// and places the result in `dst`.
func TruncMonth(src, dst []int32) {
	truncateMonth(src, dst, dateScalarConstants)
}

//go:noescape
func extractYear(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractMonth(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractDay(src, dst []int32, scalarConstants []float32)

//go:noescape
func truncateYear(src, dst []int32, scalarConstants []float32)

//go:noescape
func truncateMonth(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractDayOfWeek(src, dst []int32)
