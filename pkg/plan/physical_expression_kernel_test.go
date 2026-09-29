package plan

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalConstantBoundKernelDifferential(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	rng := rand.New(rand.NewPCG(41, 19))
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT()} {
		for _, length := range []int{0, 1, 7, 8, 9, 31, 128} {
			value := store.MakeVector(a, length, typ, true)
			lower := store.MakeVector(a, length, typ, true)
			upper := store.MakeVector(a, length, typ, true)
			for i := range length {
				if i%5 == 0 {
					value.Validity().Clear(i)
				}
				switch typ.ID() {
				case dtype.INT32T, dtype.DATET:
					value.I32s()[i], lower.I32s()[i], upper.I32s()[i] = int32(rng.IntN(45)-20), -7, 11
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					value.I64s()[i], lower.I64s()[i], upper.I64s()[i] = int64(rng.IntN(45)-20), -7, 11
				case dtype.FLOAT32T:
					value.F32s()[i], lower.F32s()[i], upper.F32s()[i] = float32(rng.IntN(45)-20), -7, 11
					if i%17 == 0 {
						value.F32s()[i] = float32(math.Copysign(0, -1))
					}
					if i%11 == 0 {
						value.F32s()[i] = float32(math.NaN())
					}
					if i%13 == 0 {
						value.F32s()[i] = float32(math.Inf(1))
					}
				case dtype.FLOAT64T:
					value.F64s()[i], lower.F64s()[i], upper.F64s()[i] = float64(rng.IntN(45)-20), -7, 11
					if i%17 == 0 {
						value.F64s()[i] = math.Copysign(0, -1)
					}
					if i%11 == 0 {
						value.F64s()[i] = math.NaN()
					}
					if i%13 == 0 {
						value.F64s()[i] = math.Inf(-1)
					}
				}
			}
			for _, operation := range []exprOp{exprOpBetween, exprOpNotBetween, exprOpClip} {
				if operation == exprOpClip && (typ.ID() == dtype.DATET || typ.ID() == dtype.TIMESTAMPTZT) {
					continue
				}
				resultType := typ
				if operation != exprOpClip {
					resultType = dtype.BoolT()
				}
				actual := store.MakeVector(a, length, resultType, true)
				want := store.MakeVector(a, length, resultType, true)
				var low, high Scalar
				switch typ.ID() {
				case dtype.INT32T:
					low, high = MakeI32Scalar(-7), MakeI32Scalar(11)
				case dtype.INT64T:
					low, high = MakeI64Scalar(-7), MakeI64Scalar(11)
				case dtype.DATET:
					low, high = MakeDateScalar(-7), MakeDateScalar(11)
				case dtype.TIMESTAMPTZT:
					low, high = MakeTimestampTZScalar(-7), MakeTimestampTZScalar(11)
				case dtype.FLOAT32T:
					low, high = MakeF32Scalar(-7), MakeF32Scalar(11)
				case dtype.FLOAT64T:
					low, high = MakeF64Scalar(-7), MakeF64Scalar(11)
				}
				if !evaluateTernary(operation, &value, &lower, &upper, &actual, [2]*Scalar{&low, &high}) || !evaluateTernary(operation, &value, &lower, &upper, &want, [2]*Scalar{}) {
					t.Fatalf("%s, %d, operation %d: cannot evaluate", typ, length, operation)
				}
				for i := range length {
					if actual.Validity().IsSet(i) != want.Validity().IsSet(i) {
						t.Fatalf("%s, %d, operation %d, row %d: validity", typ, length, operation, i)
					}
					if !want.Validity().IsSet(i) {
						continue
					}
					switch resultType.ID() {
					case dtype.BOOLT:
						if (actual.Bools()[i>>3]>>(i&7))&1 != (want.Bools()[i>>3]>>(i&7))&1 {
							t.Fatalf("%s, %d, operation %d, row %d: predicate", typ, length, operation, i)
						}
					case dtype.INT32T:
						if actual.I32s()[i] != want.I32s()[i] {
							t.Fatalf("%s, %d, row %d: clip", typ, length, i)
						}
					case dtype.INT64T:
						if actual.I64s()[i] != want.I64s()[i] {
							t.Fatalf("%s, %d, row %d: clip", typ, length, i)
						}
					case dtype.FLOAT32T:
						if math.Float32bits(actual.F32s()[i]) != math.Float32bits(want.F32s()[i]) {
							t.Fatalf("%s, %d, row %d: clip got %v want %v", typ, length, i, actual.F32s()[i], want.F32s()[i])
						}
					case dtype.FLOAT64T:
						if math.Float64bits(actual.F64s()[i]) != math.Float64bits(want.F64s()[i]) {
							t.Fatalf("%s, %d, row %d: clip got %v want %v", typ, length, i, actual.F64s()[i], want.F64s()[i])
						}
					}
				}
				actual.Release()
				want.Release()
			}
			value.Release()
			lower.Release()
			upper.Release()
		}
	}
}

func TestPhysicalSquareKernelDispatch(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T()} {
		left := store.MakeVector(a, 17, typ, true)
		right := store.MakeVector(a, 17, typ, true)
		for i := range 17 {
			if i%6 == 0 {
				left.Validity().Clear(i)
				right.Validity().Clear(i)
			}
			switch typ.ID() {
			case dtype.INT32T:
				left.I32s()[i], right.I32s()[i] = int32(math.MaxInt32-i), int32(math.MaxInt32-i)
			case dtype.INT64T:
				left.I64s()[i], right.I64s()[i] = math.MaxInt64-int64(i), math.MaxInt64-int64(i)
			case dtype.FLOAT32T:
				left.F32s()[i], right.F32s()[i] = float32(i-4)/3, float32(i-4)/3
			case dtype.FLOAT64T:
				left.F64s()[i], right.F64s()[i] = float64(i-4)/3, float64(i-4)/3
			}
		}
		if typ.ID() == dtype.FLOAT32T {
			left.F32s()[7], right.F32s()[7] = float32(math.NaN()), float32(math.NaN())
			left.F32s()[8], right.F32s()[8] = float32(math.Inf(-1)), float32(math.Inf(-1))
			left.F32s()[9], right.F32s()[9] = float32(math.Copysign(0, -1)), float32(math.Copysign(0, -1))
		} else if typ.ID() == dtype.FLOAT64T {
			left.F64s()[7], right.F64s()[7] = math.NaN(), math.NaN()
			left.F64s()[8], right.F64s()[8] = math.Inf(-1), math.Inf(-1)
			left.F64s()[9], right.F64s()[9] = math.Copysign(0, -1), math.Copysign(0, -1)
		}
		var expectedI64 []int64
		if typ.ID() == dtype.INT64T {
			expectedI64 = append([]int64(nil), left.I64s()...)
		}
		batch := store.MakeBatch([]store.Vector{left, right})
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		physical, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"x", "y"}, []dtype.Type{typ, typ}, []bool{true, true})).Project(
			MakeColumn("x").Mul(MakeColumn("x")), MakeColumn("x").Mul(MakeColumn("y")), MakeColumn("x").Mul(int64(3)), MakeColumn("x").Add(1).Sq(),
		))
		if err != nil {
			t.Fatal(err)
		}
		table.Release()
		squares := 0
		for _, node := range physical.steps[0].program.nodes {
			if node.square {
				squares++
			}
		}
		if squares != 2 {
			t.Fatalf("%s: specialized %d squares", typ, squares)
		}
		if !physical.Execute(a, func(output *store.Batch) bool {
			fast, slow := output.VectorAt(0), output.VectorAt(1)
			literal := output.VectorAt(2)
			computed := output.VectorAt(3)
			for i := range 17 {
				if fast.Validity().IsSet(i) != slow.Validity().IsSet(i) {
					t.Fatalf("%s row %d: validity", typ, i)
				}
				if !fast.Validity().IsSet(i) {
					continue
				}
				switch typ.ID() {
				case dtype.INT32T:
					if fast.I32s()[i] != slow.I32s()[i] {
						t.Fatalf("%s row %d: square", typ, i)
					}
				case dtype.INT64T:
					input := expectedI64[i]
					if fast.I64s()[i] != input*input || slow.I64s()[i] != input*input || literal.I64s()[i] != input*3 || computed.I64s()[i] != (input+1)*(input+1) {
						t.Fatalf("%s row %d: square %d, product %d, literal %d", typ, i, fast.I64s()[i], slow.I64s()[i], literal.I64s()[i])
					}
				case dtype.FLOAT32T:
					if fast.F32s()[i] != slow.F32s()[i] && !(math.IsNaN(float64(fast.F32s()[i])) && math.IsNaN(float64(slow.F32s()[i]))) {
						t.Fatalf("%s row %d: square", typ, i)
					}
				case dtype.FLOAT64T:
					if fast.F64s()[i] != slow.F64s()[i] && !(math.IsNaN(fast.F64s()[i]) && math.IsNaN(slow.F64s()[i])) {
						t.Fatalf("%s row %d: square", typ, i)
					}
				}
			}
			return true
		}) {
			t.Fatalf("%s: execution failed", typ)
		}
		physical.Release()
	}
}

func TestPhysicalMixedIntegerFloatClip(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	for _, length := range []int{0, 1, 7, 8, 9, 33} {
		column := store.MakeVector(a, length, dtype.Int64T(), true)
		for row := range length {
			column.I64s()[row] = int64(row)*math.MaxInt64/31 - math.MaxInt64/2
			if row%4 == 0 {
				column.Validity().Clear(row)
			}
		}
		batch := store.MakeBatch([]store.Vector{column})
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		for _, pair := range []struct{ lower, upper any }{
			{float32(-2.5), float32(8.5)},
			{float64(-2.5), float64(8.5)},
			{float32(5), float64(-5)},
			{math.Inf(-1), math.Inf(1)},
			{math.NaN(), float64(8.5)},
		} {
			physical, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"x"}, []dtype.Type{dtype.Int64T()}, []bool{true})).Project(
				MakeColumn("x").Clip(pair.lower, pair.upper),
			))
			if err != nil {
				t.Fatal(err)
			}
			root := physical.steps[0].program.roots[0]
			lower64, isF64 := pair.lower.(float64)
			if physical.steps[0].program.nodes[root].clipI64F64 == (isF64 && math.IsNaN(lower64)) {
				t.Fatal("integer-to-float clip chose the wrong dispatch")
			}
			for i, node := range physical.steps[0].program.nodes {
				if node.kind == exprLiteral && physical.steps[0].program.materialize[i] {
					t.Fatal("clip materialized a scalar bound")
				}
			}
			if !physical.Execute(a, func(result *store.Batch) bool {
				out := result.VectorAt(0)
				for i := range length {
					if out.Validity().IsSet(i) != (i%4 != 0) {
						t.Fatalf("length %d bounds %v/%v row %d: output validity %t", length, pair.lower, pair.upper, i, out.Validity().IsSet(i))
					}
					if !out.Validity().IsSet(i) {
						continue
					}
					value := float64(int64(i)*math.MaxInt64/31 - math.MaxInt64/2)
					lower := pair.lower
					upper := pair.upper
					lo, hi := float64(0), float64(0)
					switch x := lower.(type) {
					case float32:
						lo = float64(x)
					case float64:
						lo = x
					}
					switch x := upper.(type) {
					case float32:
						hi = float64(x)
					case float64:
						hi = x
					}
					if value < lo {
						value = lo
					}
					if value > hi {
						value = hi
					}
					if got := out.F64s()[i]; math.Float64bits(got) != math.Float64bits(value) && !(math.IsNaN(got) && math.IsNaN(value)) {
						t.Fatalf("bounds %v/%v row %d: clip got %v want %v", pair.lower, pair.upper, i, got, value)
					}
				}
				return true
			}) {
				t.Fatal("execution failed")
			}
			physical.Release()
		}
		table.Release()
	}
}

func TestPhysicalScalarOperandsWithoutLiteralVectors(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	for _, length := range []int{0, 1, 9} {
		text := make([][]byte, length)
		upper := make([][]byte, length)
		valid := make([]bool, length)
		numbers := store.MakeVector(a, length, dtype.Int32T(), true)
		bools := store.MakeVector(a, length, dtype.BoolT(), true)
		for i := range length {
			text[i], upper[i], valid[i] = []byte("abc"), []byte("az"), true
			numbers.I32s()[i] = int32(i)
			if i%3 == 0 {
				text[i], valid[i] = nil, false
				numbers.Validity().Clear(i)
				bools.Validity().Clear(i)
			}
			if i%2 == 0 {
				bools.Bools()[i>>3] |= 1 << (i & 7)
			}
		}
		strings := store.MakeStringVector(a, text, valid)
		upperStrings := store.MakeStringVector(a, upper, nil)
		batch := store.MakeBatch([]store.Vector{strings, upperStrings, numbers, bools})
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		physical, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability(
			[]string{"s", "upper", "n", "flag"}, []dtype.Type{dtype.StringT(), dtype.StringT(), dtype.Int32T(), dtype.BoolT()}, []bool{true, false, true, true},
		)).Project(
			MakeColumn("s").Eq("abc"), MakeLiteral("abc").Eq(MakeColumn("s")),
			MakeColumn("s").Like("a%"), MakeLiteral("xabc").Contains(MakeColumn("s")),
			MakeColumn("flag").Eq(true), MakeLiteral(false).Ne(MakeColumn("flag")),
			MakeColumn("s").Between("aaa", MakeColumn("upper")),
			MakeColumn("n").Between(2, MakeColumn("n").Add(3)),
			MakeColumn("n").Clip(MakeColumn("n").Sub(2), 5),
			MakeColumn("n").Between(0, 8),
			MakeColumn("s").Replace("b", "x"), MakeColumn("s").Concat("Z"),
			MakeColumn("s").Slice(1, 3), MakeColumn("s").Replace("b", nil),
			MakeColumn("n").Between(nil, 8), MakeColumn("n").Clip(nil, 5),
		))
		if err != nil {
			t.Fatal(err)
		}
		table.Release()
		for i, node := range physical.steps[0].program.nodes {
			if node.kind == exprLiteral && physical.steps[0].program.materialize[i] {
				t.Fatalf("length %d: materialized literal %d", length, i)
			}
		}
		if !physical.Execute(a, func(output *store.Batch) bool {
			for i := range length {
				for col := range output.NVectors() {
					result := output.VectorAt(col)
					if col >= 13 {
						if result.Validity().IsSet(i) {
							t.Fatalf("length %d row %d: NULL string replacement became valid", length, i)
						}
						continue
					}
					if !result.Validity().IsSet(i) {
						if i%3 != 0 {
							t.Fatalf("length %d row %d column %d: unexpected NULL", length, i, col)
						}
						continue
					}
					if col == 8 {
						if got, want := result.I32s()[i], int32(min(i, 5)); got != want {
							t.Fatalf("length %d row %d: clipped %d want %d", length, i, got, want)
						}
						continue
					}
					if col >= 10 {
						want := []string{"axc", "abcZ", "bc"}[col-10]
						if got := result.Strings()[i].View(); got != want {
							t.Fatalf("length %d row %d column %d: string %q want %q", length, i, col, got, want)
						}
						continue
					}
					got := result.Bools()[i>>3]&(1<<(i&7)) != 0
					want := []bool{true, true, true, true, i%2 == 0, i%2 == 0, true, i >= 2, false, i <= 8}[col]
					if got != want {
						t.Fatalf("length %d row %d column %d: got %t want %t", length, i, col, got, want)
					}
				}
			}
			return true
		}) {
			t.Fatal("scalar operand execution failed")
		}
		physical.Release()
	}
}

func TestPhysicalNaNScalarBoundsMatchDenseReference(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	for _, typ := range []dtype.Type{dtype.Float32T(), dtype.Float64T()} {
		value := store.MakeVector(a, 9, typ, true)
		lower := store.MakeVector(a, 9, typ, true)
		upper := store.MakeVector(a, 9, typ, true)
		var low, high Scalar
		if typ.ID() == dtype.FLOAT32T {
			low, high = MakeF32Scalar(float32(math.NaN())), MakeF32Scalar(5)
			for i := range 9 {
				value.F32s()[i], lower.F32s()[i], upper.F32s()[i] = float32(i-4), float32(math.NaN()), 5
			}
		} else {
			low, high = MakeF64Scalar(math.NaN()), MakeF64Scalar(5)
			for i := range 9 {
				value.F64s()[i], lower.F64s()[i], upper.F64s()[i] = float64(i-4), math.NaN(), 5
			}
		}
		value.Validity().Clear(5)
		for _, operation := range []exprOp{exprOpBetween, exprOpNotBetween, exprOpClip} {
			resultType := typ
			if operation != exprOpClip {
				resultType = dtype.BoolT()
			}
			got := store.MakeVector(a, 9, resultType, true)
			want := store.MakeVector(a, 9, resultType, true)
			if !evaluateTernary(operation, &value, &lower, &upper, &got, [2]*Scalar{&low, &high}) || !evaluateTernary(operation, &value, &lower, &upper, &want, [2]*Scalar{}) {
				t.Fatalf("%s operation %d: evaluate failed", typ, operation)
			}
			for i := range 9 {
				if got.Validity().IsSet(i) != want.Validity().IsSet(i) {
					t.Fatalf("%s operation %d row %d: validity", typ, operation, i)
				}
				if !want.Validity().IsSet(i) {
					continue
				}
				if operation != exprOpClip {
					if got.Bools()[i>>3]&(1<<(i&7)) != want.Bools()[i>>3]&(1<<(i&7)) {
						t.Fatalf("%s operation %d row %d: predicate", typ, operation, i)
					}
				} else if typ.ID() == dtype.FLOAT32T {
					if math.Float32bits(got.F32s()[i]) != math.Float32bits(want.F32s()[i]) {
						t.Fatalf("%s row %d: clipping NaN lower bound", typ, i)
					}
				} else if math.Float64bits(got.F64s()[i]) != math.Float64bits(want.F64s()[i]) {
					t.Fatalf("%s row %d: clipping NaN lower bound", typ, i)
				}
			}
			got.Release()
			want.Release()
		}
		value.Release()
		lower.Release()
		upper.Release()
	}
}
