package dtype

import "testing"

func TestTypeConstructors(t *testing.T) {
	tests := []struct {
		name        string
		typeValue   Type
		id          TID
		stringValue string
		slotBits    int
		fixed       bool
		numericPrim bool
		numeric     bool
		time        bool
		integral    bool
		floating    bool
		varlen      bool
		bitPacked   bool
	}{
		{"null", NullT(), NULLT, "na_t", 0, false, false, false, false, false, false, false, false},
		{"int32", Int32T(), INT32T, "int32_t", 32, true, true, true, false, true, false, false, false},
		{"int64", Int64T(), INT64T, "int64_t", 64, true, true, true, false, true, false, false, false},
		{"float32", Float32T(), FLOAT32T, "float32_t", 32, true, true, true, false, false, true, false, false},
		{"float64", Float64T(), FLOAT64T, "float64_t", 64, true, true, true, false, false, true, false, false},
		{"date", DateT(), DATET, "date_t", 32, true, true, false, true, false, false, false, false},
		{"timestamp", TimestampTZT(), TIMESTAMPTZT, "timestamptz_t", 64, true, true, false, true, false, false, false, false},
		{"string", StringT(), STRT, "string_t", 128, false, false, false, false, false, false, true, false},
		{"bool", BoolT(), BOOLT, "bool_t", 1, true, false, false, false, false, false, false, true},
	}

	ids := make(map[TID]struct{}, len(tests))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !test.typeValue.Valid() {
				t.Fatal("canonical type is invalid")
			}
			if got := test.typeValue.ID(); got != test.id {
				t.Fatalf("ID: got %d, expected %d", got, test.id)
			}
			if got := test.typeValue.String(); got != test.stringValue {
				t.Fatalf("String: got %q, expected %q", got, test.stringValue)
			}
			if got := test.typeValue.SlotBits(); got != test.slotBits {
				t.Fatalf("SlotBits: got %d, expected %d", got, test.slotBits)
			}
			if got := test.typeValue.Size1(); got != test.slotBits {
				t.Fatalf("Size1: got %d, expected %d", got, test.slotBits)
			}
			if got := test.typeValue.IsFixedSize(); got != test.fixed {
				t.Fatalf("IsFixedSize: got %t, expected %t", got, test.fixed)
			}
			if got := test.typeValue.IsNumericPrim(); got != test.numericPrim {
				t.Fatalf("IsNumericPrim: got %t, expected %t", got, test.numericPrim)
			}
			if got := test.typeValue.IsNumericType(); got != test.numeric {
				t.Fatalf("IsNumericType: got %t, expected %t", got, test.numeric)
			}
			if got := test.typeValue.IsTimeType(); got != test.time {
				t.Fatalf("IsTimeType: got %t, expected %t", got, test.time)
			}
			if got := test.typeValue.IsIntegral(); got != test.integral {
				t.Fatalf("IsIntegral: got %t, expected %t", got, test.integral)
			}
			if got := test.typeValue.IsFloating(); got != test.floating {
				t.Fatalf("IsFloating: got %t, expected %t", got, test.floating)
			}
			if got := test.typeValue.HasVarlen(); got != test.varlen {
				t.Fatalf("HasVarlen: got %t, expected %t", got, test.varlen)
			}
			if got := test.typeValue.IsBitPacked(); got != test.bitPacked {
				t.Fatalf("IsBitPacked: got %t, expected %t", got, test.bitPacked)
			}
			if got := MakeType(test.id); !got.Equal(test.typeValue) {
				t.Fatalf("MakeType: got %+v, expected %+v", got, test.typeValue)
			}
		})

		if _, exists := ids[test.id]; exists {
			t.Fatalf("duplicate type ID %d", test.id)
		}
		ids[test.id] = struct{}{}
	}
}

func TestInvalidType(t *testing.T) {
	var invalid Type
	if invalid.Valid() {
		t.Fatal("zero Type is valid")
	}
	if invalid.ID() != INVALIDT {
		t.Fatalf("ID: got %d, expected %d", invalid.ID(), INVALIDT)
	}
	if invalid.String() != "invalid_t" {
		t.Fatalf("String: got %q, expected invalid_t", invalid.String())
	}
	if MakeType(INVALIDT).Valid() || MakeType(TID(255)).Valid() {
		t.Fatal("MakeType accepted an invalid ID")
	}
	if (Type{id: TID(255)}).String() != "unknown" {
		t.Fatal("unknown type ID has an unexpected name")
	}
}

func TestTypeCapabilities(t *testing.T) {
	if !Int64T().CanArithmetic() || DateT().CanArithmetic() || BoolT().CanArithmetic() {
		t.Fatal("unexpected arithmetic capability")
	}
	if !StringT().CanOrder() || !DateT().CanOrder() || BoolT().CanOrder() {
		t.Fatal("unexpected ordering capability")
	}
	if !BoolT().CanCompareEquality() || NullT().CanCompareEquality() {
		t.Fatal("unexpected equality capability")
	}
	if !StringT().CanHash() || NullT().CanHash() || (Type{}).CanHash() {
		t.Fatal("unexpected hashing capability")
	}
}
