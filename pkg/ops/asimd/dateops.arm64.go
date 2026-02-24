//go:build arm64

package asimd

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

//go:noescape
func extractYearFromDateVec(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractMonthFromDateVec(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractDayFromDateVec(src, dst []int32, scalarConstants []float32)

//go:noescape
func truncateYearFromDateVec(src, dst []int32, scalarConstants []float32)

//go:noescape
func truncateMonthFromDateVec(src, dst []int32, scalarConstants []float32)

//go:noescape
func extractDayOfWeekFromDateVec(src, dst []int32)
