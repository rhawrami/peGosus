package plan

import (
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

func TestSchemaBinding(t *testing.T) {
	schema := MakeSchema([]string{"income", "sex", "age"}, []dtype.Type{dtype.Int64T(), dtype.StringT(), dtype.Int32T()})
	if !schema.Valid() || schema.Len() != 3 {
		t.Fatal("schema is invalid")
	}
	for i := range schema.Len() {
		if schema.FieldAt(i).ID() == 0 || (i != 0 && schema.FieldAt(i).ID() == schema.FieldAt(i-1).ID()) {
			t.Fatalf("field %d has an invalid or duplicate ID", i)
		}
		if !schema.FieldAt(i).Nullable() {
			t.Fatalf("default field %d is unexpectedly non-nullable", i)
		}
	}
	if offset, ok := schema.find("sex"); !ok || offset != 1 {
		t.Fatalf("sex binding: got %d, %t", offset, ok)
	}

	ambiguous := MakeSchema([]string{"x", "x"}, []dtype.Type{dtype.Int32T(), dtype.Int64T()})
	if _, ok := ambiguous.find("x"); ok {
		t.Fatal("ambiguous name was bound")
	}
	if MakeSchema([]string{"x"}, nil).Valid() || MakeSchema([]string{"x"}, []dtype.Type{dtype.NullT()}).Valid() {
		t.Fatal("invalid schema was accepted")
	}
}

func TestScalarNumericConversion(t *testing.T) {
	tests := []struct {
		src  Scalar
		dst  dtype.Type
		bits uint64
	}{
		{MakeI32Scalar(-7), dtype.Int64T(), MakeI64Scalar(-7).bits},
		{MakeI64Scalar(42), dtype.Float64T(), MakeF64Scalar(42).bits},
		{MakeF32Scalar(2.5), dtype.Float64T(), MakeF64Scalar(2.5).bits},
	}
	for _, test := range tests {
		got, ok := test.src.convert(test.dst)
		if !ok || !got.Type().Equal(test.dst) || got.bits != test.bits {
			t.Fatalf("convert %s to %s: got %+v, %t", test.src.Type(), test.dst, got, ok)
		}
	}
	if _, ok := MakeDateScalar(1).convert(dtype.Int32T()); ok {
		t.Fatal("converted DATE as a numeric scalar")
	}
	if _, ok := MakeF64Scalar(9.75).convert(dtype.Int32T()); ok {
		t.Fatal("accepted a narrowing scalar conversion")
	}
}
