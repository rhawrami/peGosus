package dtype

import "testing"

func TestTypesCanConvIdentity(t *testing.T) {
	types := []Type{NullT(), Int32T(), Int64T(), Float32T(), Float64T(), DateT(), TimestampTZT(), StringT(), BoolT()}
	for _, typeValue := range types {
		if !TypesCanConv(typeValue, typeValue) {
			t.Fatalf("identity conversion rejected for %s", typeValue)
		}
	}
	if TypesCanConv(Type{}, Int64T()) || TypesCanConv(Int64T(), Type{}) {
		t.Fatal("conversion accepted an invalid type")
	}
}

func TestTypesCanConvRules(t *testing.T) {
	tests := []struct {
		name string
		src  Type
		dst  Type
		ok   bool
	}{
		{"null to int64", NullT(), Int64T(), true},
		{"int64 to null", Int64T(), NullT(), false},
		{"int32 to float64", Int32T(), Float64T(), true},
		{"float64 to int32", Float64T(), Int32T(), true},
		{"string to timestamp", StringT(), TimestampTZT(), true},
		{"timestamp to string", TimestampTZT(), StringT(), true},
		{"bool to int32", BoolT(), Int32T(), true},
		{"int32 to bool", Int32T(), BoolT(), true},
		{"date to timestamp", DateT(), TimestampTZT(), true},
		{"timestamp to date", TimestampTZT(), DateT(), true},
		{"date to int32", DateT(), Int32T(), false},
		{"bool to date", BoolT(), DateT(), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TypesCanConv(test.src, test.dst); got != test.ok {
				t.Fatalf("got %t, expected %t", got, test.ok)
			}
		})
	}
}

func TestTypesCanConvMatrix(t *testing.T) {
	types := []Type{NullT(), Int32T(), Int64T(), Float32T(), Float64T(), DateT(), TimestampTZT(), StringT(), BoolT()}
	matrix := []string{
		"111111111",
		"011110011",
		"011110011",
		"011110011",
		"011110011",
		"000001110",
		"000001110",
		"011111111",
		"011110011",
	}

	for srcO, src := range types {
		for dstO, dst := range types {
			want := matrix[srcO][dstO] == '1'
			if got := TypesCanConv(src, dst); got != want {
				t.Fatalf("TypesCanConv(%s, %s): got %t, expected %t", src, dst, got, want)
			}
		}
	}
}

func TestCommonNumericType(t *testing.T) {
	tests := []struct {
		left  Type
		right Type
		want  Type
	}{
		{Int32T(), Int32T(), Int32T()},
		{Int32T(), Int64T(), Int64T()},
		{Int64T(), Int32T(), Int64T()},
		{Float32T(), Float32T(), Float32T()},
		{Float32T(), Int32T(), Float64T()},
		{Int64T(), Float32T(), Float64T()},
		{Float64T(), Int32T(), Float64T()},
		{Float32T(), Float64T(), Float64T()},
	}

	for _, test := range tests {
		got, ok := CommonNumericType(test.left, test.right)
		if !ok || !got.Equal(test.want) {
			t.Fatalf("CommonNumericType(%s, %s): got %s, %t; expected %s, true", test.left, test.right, got, ok, test.want)
		}
	}
	if got, ok := CommonNumericType(Int64T(), StringT()); ok || got.Valid() {
		t.Fatalf("nonnumeric common type: got %s, %t", got, ok)
	}
}

func TestCommonNumericTypeMatrix(t *testing.T) {
	types := []Type{Int32T(), Int64T(), Float32T(), Float64T()}
	want := [][]Type{
		{Int32T(), Int64T(), Float64T(), Float64T()},
		{Int64T(), Int64T(), Float64T(), Float64T()},
		{Float64T(), Float64T(), Float32T(), Float64T()},
		{Float64T(), Float64T(), Float64T(), Float64T()},
	}

	for leftO, left := range types {
		for rightO, right := range types {
			got, ok := CommonNumericType(left, right)
			if !ok || !got.Equal(want[leftO][rightO]) {
				t.Fatalf("CommonNumericType(%s, %s): got %s, %t; expected %s, true", left, right, got, ok, want[leftO][rightO])
			}
		}
	}
}

func TestTypeComparisons(t *testing.T) {
	if !TypesEqType(Int64T(), MakeType(INT64T)) || TypesEqType(Int64T(), Float64T()) {
		t.Fatal("unexpected type equality")
	}
	if !TypesEqSize(Int32T(), Float32T()) || !TypesEqSize(Int64T(), TimestampTZT()) {
		t.Fatal("equal slot sizes were not recognized")
	}
	if TypesEqSize(NullT(), NullT()) || TypesEqSize(Type{}, NullT()) || TypesEqSize(StringT(), Int64T()) {
		t.Fatal("unexpected slot-size equality")
	}
}
