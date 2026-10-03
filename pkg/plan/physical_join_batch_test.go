package plan

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type joinBatchReferenceRow struct {
	key                            float64
	text                           string
	boolean, payload               bool
	keyNull, boolNull, payloadNull bool
	id                             int64
	active                         bool
}

func makeJoinBatchTestSource(a *mem.Allocator, parts, rows int, dictionary bool) (*store.Table, Schema, []joinBatchReferenceRow) {
	keys := [][]byte{[]byte("null binary key"), []byte("nan binary key"), []byte("long binary \x00\xff shared zero key"), []byte("long binary \x00\xff shared zero key"), []byte("long positive infinity binary \x00 key"), []byte("long negative infinity binary \xff key"), []byte("long numeric two binary \x00 key")}
	records := make([]joinBatchReferenceRow, 0, parts*rows)
	batches := make([]*store.Batch, parts)
	for part := range parts {
		f := store.MakeVector(a, rows, dtype.Float64T(), true)
		boolean := store.MakeVector(a, rows, dtype.BoolT(), true)
		ids := store.MakeVector(a, rows, dtype.Int64T(), false)
		payload := store.MakeVector(a, rows, dtype.BoolT(), true)
		indices := a.AllocSeg(rows * 4)
		texts := make([][]byte, rows)
		selected := store.MakeBitMap(a, rows)
		offsets := []uint32{}
		for row := range rows {
			id := part*rows + row
			tag := id % len(keys)
			value := float64(2)
			switch tag {
			case 0:
				f.Validity().Clear(row)
			case 1:
				value = math.Float64frombits(0x7ff8000000000000 | uint64(id+1))
			case 2:
				value = math.Copysign(0, -1)
			case 3:
				value = 0
			case 4:
				value = math.Inf(1)
			case 5:
				value = math.Inf(-1)
			}
			f.F64s()[row] = value
			ids.I64s()[row] = int64(id)
			bit := tag%2 == 0
			if tag == 3 {
				bit = true
			}
			if bit {
				boolean.Bools()[row>>3] |= 1 << (row & 7)
			}
			boolNull := id%17 == 0
			if boolNull {
				boolean.Validity().Clear(row)
			}
			payloadBit := id%3 == 0
			if payloadBit {
				payload.Bools()[row>>3] |= 1 << (row & 7)
			}
			payloadNull := id%11 == 0
			if payloadNull {
				payload.Validity().Clear(row)
			}
			indices.AsU32T()[row] = uint32(tag)
			texts[row] = keys[tag]
			active := id%9 != 0
			if active {
				selected.Set(row)
				offsets = append(offsets, uint32(row))
			}
			records = append(records, joinBatchReferenceRow{key: value, text: string(keys[tag]), boolean: bit, payload: payloadBit, keyNull: tag == 0, boolNull: boolNull, payloadNull: payloadNull, id: int64(id), active: active})
		}
		var stringVector store.Vector
		if dictionary {
			stringVector = store.MakeDictionaryStringVectorFromOwnedSegments(store.MakeStringVector(a, keys, nil), rows, indices, nil)
		} else {
			indices.Dec()
			stringVector = store.MakeStringVector(a, texts, nil)
		}
		batches[part] = store.MakeBatch([]store.Vector{f, stringVector, boolean, ids, payload})
		if part%2 == 0 {
			batches[part].SetSelection(store.MakeRowSelectionFromBitMap(selected))
		} else {
			selected.Release()
			batches[part].SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, rows, offsets)))
		}
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"k", "text", "flag", "id", "payload"}, []dtype.Type{dtype.Float64T(), dtype.StringT(), dtype.BoolT(), dtype.Int64T(), dtype.BoolT()}, []bool{true, false, true, false, true})
	return table, schema, records
}

func joinBatchReferenceMatches(left, right joinBatchReferenceRow, residual bool) bool {
	return left.active && right.active && !left.keyNull && !right.keyNull && !left.boolNull && !right.boolNull && left.key == right.key && left.text == right.text && left.boolean == right.boolean && (!residual || left.id < right.id)
}

func joinBatchReferencePairs(left, right []joinBatchReferenceRow, kind JoinKind, residual bool) map[string]int {
	want := make(map[string]int)
	matchedRight := make([]bool, len(right))
	for _, l := range left {
		if !l.active {
			continue
		}
		matches := 0
		for j, r := range right {
			if !joinBatchReferenceMatches(l, r, residual) {
				continue
			}
			matches++
			matchedRight[j] = true
			if kind == JoinInner || kind == JoinLeft || kind == JoinFull {
				want[fmt.Sprintf("%d/%d", l.id, r.id)]++
			}
		}
		if kind == JoinSemi && matches > 0 || kind == JoinAnti && matches == 0 {
			want[fmt.Sprintf("%d/-", l.id)]++
		} else if matches == 0 && (kind == JoinLeft || kind == JoinFull) {
			want[fmt.Sprintf("%d/-", l.id)]++
		}
	}
	if kind == JoinFull {
		for j, r := range right {
			if r.active && !matchedRight[j] {
				want[fmt.Sprintf("-/%d", r.id)]++
			}
		}
	}
	return want
}

func checkJoinBatchReferenceSide(t *testing.T, batch *store.Batch, row, offset int, records []joinBatchReferenceRow) string {
	t.Helper()
	idVector := batch.VectorAt(offset + 3)
	if idVector.Validity() != nil && !idVector.Validity().IsSet(row) {
		for c := range 5 {
			v := batch.VectorAt(offset + c)
			if v.Validity() == nil || v.Validity().IsSet(row) {
				t.Errorf("unmatched side column %d is valid", c)
			}
		}
		return "-"
	}
	id := idVector.I64s()[row]
	if id < 0 || id >= int64(len(records)) {
		t.Fatalf("unknown output id %d", id)
	}
	ref := records[id]
	for column := range 5 {
		v := batch.VectorAt(offset + column)
		isNull := v.Validity() != nil && !v.Validity().IsSet(row)
		wantNull := column == 0 && ref.keyNull || column == 2 && ref.boolNull || column == 4 && ref.payloadNull
		if isNull != wantNull {
			t.Errorf("id=%d column=%d null=%t want=%t", id, column, isNull, wantNull)
		}
		if isNull {
			continue
		}
		switch column {
		case 0:
			if math.Float64bits(v.F64s()[row]) != math.Float64bits(ref.key) {
				t.Errorf("key bits changed id=%d", id)
			}
		case 1:
			if v.StringAt(row).View() != ref.text {
				t.Errorf("string ownership/value changed id=%d", id)
			}
		case 2, 4:
			want := ref.boolean
			if column == 4 {
				want = ref.payload
			}
			actual := v.Bools()[row>>3]&(1<<(row&7)) != 0
			if actual != want {
				t.Errorf("boolean payload changed id=%d column=%d", id, column)
			}
		}
	}
	return fmt.Sprint(id)
}

func TestPhysicalJoinBatchReferenceAndOwnership(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		for _, residual := range []bool{false, true} {
			t.Run(fmt.Sprintf("kind=%d/residual=%t", kind, residual), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				left, leftSchema, leftRows := makeJoinBatchTestSource(a, 4, 97, false)
				right, rightSchema, rightRows := makeJoinBatchTestSource(a, 3, 113, true)
				l := MakeScan(left, leftSchema).As("l")
				r := MakeScan(right, rightSchema).As("r")
				var predicates []Expr
				if residual {
					predicates = []Expr{MakeColumn("l.id").Lt(MakeColumn("r.id"))}
				}
				p, err := MakePhysicalPlan(l.Join(r, kind, []Expr{MakeColumn("l.k"), MakeColumn("l.text"), MakeColumn("l.flag")}, []Expr{MakeColumn("r.k"), MakeColumn("r.text"), MakeColumn("r.flag")}, predicates...))
				if err != nil {
					t.Fatal(err)
				}
				var outputs []*store.Batch
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, func(batch *store.Batch) bool {
					if batch.Len() > joinOutputBatchRows {
						t.Errorf("unbounded output %d", batch.Len())
					}
					outputs = append(outputs, batch.Retain())
					return true
				})
				p.Release()
				left.Release()
				right.Release()
				if result.Code() != ExecutionCompleted {
					t.Fatalf("result %v/%v", result.Code(), result.Err())
				}
				want := joinBatchReferencePairs(leftRows, rightRows, kind, residual)
				seen := make(map[string]int)
				for _, batch := range outputs {
					bitmap := batch.Selection().RetainBitMap(a)
					for row := range batch.Len() {
						if bitmap != nil && !bitmap.IsSet(row) {
							continue
						}
						lid := checkJoinBatchReferenceSide(t, batch, row, 0, leftRows)
						rid := "-"
						if kind != JoinSemi && kind != JoinAnti {
							rid = checkJoinBatchReferenceSide(t, batch, row, 5, rightRows)
						}
						seen[lid+"/"+rid]++
					}
					bitmap.Release()
					batch.Release()
				}
				if kind == JoinInner && (len(want) <= joinOutputBatchRows || len(outputs) < 2) {
					t.Fatal("reference case did not cross candidate/output chunk boundaries")
				}
				if len(seen) != len(want) {
					t.Fatalf("pairs %d want %d", len(seen), len(want))
				}
				for pair, count := range want {
					if seen[pair] != count {
						t.Errorf("pair %s count=%d want=%d", pair, seen[pair], count)
					}
				}
				usage := a.Usage()
				if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked %+v", usage)
				}
			})
		}
	}
}

func TestPhysicalJoinBatchFanoutStopBudgetAndCancellation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	leftKey := store.MakeVector(a, 1, dtype.Int32T(), false)
	leftKey.I32s()[0] = 1
	leftBatch := store.MakeBatch([]store.Vector{leftKey})
	left := store.MakeTable([]*store.Batch{leftBatch})
	leftBatch.Release()
	rightBatches := make([]*store.Batch, 6)
	for part := range rightBatches {
		keys := store.MakeVector(a, 1000, dtype.Int32T(), false)
		texts := make([][]byte, 1000)
		for row := range texts {
			keys.I32s()[row] = 1
			texts[row] = []byte(fmt.Sprintf("long duplicate payload after retained build %06d", part*1000+row))
		}
		rightBatches[part] = store.MakeBatch([]store.Vector{keys, store.MakeStringVector(a, texts, nil)})
	}
	right := store.MakeTable(rightBatches)
	for _, b := range rightBatches {
		b.Release()
	}
	p, err := MakePhysicalPlan(MakeScan(left, MakeSchemaWithNullability([]string{"k"}, []dtype.Type{dtype.Int32T()}, []bool{false})).As("l").Join(MakeScan(right, MakeSchemaWithNullability([]string{"k", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{false, false})).As("r"), JoinInner, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")}))
	if err != nil {
		t.Fatal(err)
	}
	calls, rows := 0, 0
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: 1}, func(batch *store.Batch) bool {
		calls++
		rows += batch.ActiveLen()
		if batch.Len() > joinOutputBatchRows {
			t.Errorf("unbounded batch %d", batch.Len())
		}
		return false
	})
	if result.Code() != ExecutionStopped || calls != 1 || rows != joinOutputBatchRows {
		t.Fatalf("early fanout %v calls=%d rows=%d", result.Code(), calls, rows)
	}
	limited := *p
	limited.steps = append(append([]physicalStep(nil), p.steps...), physicalStep{operation: physicalLimit, limit: 3})
	calls, rows = 0, 0
	result = limited.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: 1}, func(batch *store.Batch) bool {
		calls++
		rows += batch.ActiveLen()
		return true
	})
	if result.Code() != ExecutionCompleted || calls != 1 || rows != 3 {
		t.Fatalf("limited fanout %v calls=%d rows=%d", result.Code(), calls, rows)
	}
	result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 512, Workers: 1}, func(*store.Batch) bool { t.Error("tight budget reached sink"); return true })
	if result.Code() != ExecutionResourceExhausted {
		t.Fatalf("budget %v", result.Code())
	}
	scope := mem.MakeAllocationScope(a, 8<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	state := joinState{scope: scope}
	state.keys.setScope(scope)
	step := p.steps[0]
	if result := state.build(context.Background(), scoped, step.join, 8<<20); result.Code() != ExecutionCompleted {
		t.Fatalf("build %v", result.Code())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls = 0
	result = state.probe(ctx, scoped, left.BatchAt(0), step, 8<<20, func(batch *store.Batch) ExecutionResult {
		calls++
		batch.Release()
		cancel()
		return ExecutionResult{code: ExecutionCompleted}
	})
	if result.Code() != ExecutionCancelled || calls != 1 {
		t.Fatalf("duplicate expansion cancellation %v calls=%d", result.Code(), calls)
	}
	state.release()
	if scope.Live() != 0 {
		t.Fatalf("cancel leaked scoped bytes %d", scope.Live())
	}
	p.Release()
	left.Release()
	right.Release()
	usage := a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestPhysicalJoinBatchFullEmptyInputLimit(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchemaWithNullability([]string{"k"}, []dtype.Type{dtype.Int32T()}, []bool{false})
	left := store.MakeEmptyTable([]dtype.Type{dtype.Int32T()})
	rightBatches := make([]*store.Batch, 3)
	for chunk := range rightBatches {
		v := store.MakeVector(a, 1025, dtype.Int32T(), false)
		for row := range v.I32s() {
			v.I32s()[row] = int32(chunk*1025 + row)
		}
		rightBatches[chunk] = store.MakeBatch([]store.Vector{v})
	}
	right := store.MakeTable(rightBatches)
	for _, batch := range rightBatches {
		batch.Release()
	}
	joined := MakeScan(left, schema).As("l").Join(MakeScan(right, schema).As("r"), JoinFull, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")})
	for _, limit := range []int64{0, 3, 1026} {
		p, err := MakePhysicalPlan(joined.Limit(limit))
		if err != nil {
			t.Fatal(err)
		}
		var retained []*store.Batch
		count := 0
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: 4}, func(batch *store.Batch) bool {
			count += batch.ActiveLen()
			retained = append(retained, batch.Retain())
			return true
		})
		p.Release()
		if result.Code() != ExecutionCompleted || int64(count) != limit {
			t.Fatalf("full empty left limit=%d result=%v rows=%d", limit, result.Code(), count)
		}
		for _, batch := range retained {
			bitmap := batch.Selection().RetainBitMap(a)
			for row := range batch.Len() {
				if bitmap != nil && !bitmap.IsSet(row) {
					continue
				}
				if batch.VectorAt(0).Validity() == nil || batch.VectorAt(0).Validity().IsSet(row) {
					t.Error("unmatched left is valid")
				}
			}
			bitmap.Release()
			batch.Release()
		}
	}
	left.Release()
	right.Release()
	usage := a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestPhysicalJoinBatchNestedBuildScope(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 2, 33, false)
	right, rs, _ := makeJoinBatchTestSource(a, 3, 65, true)
	projected := MakeScan(right, rs).Project(MakeColumn("k"), MakeColumn("text").Concat(" / newly allocated build payload").Alias("text")).As("r")
	p, err := MakePhysicalPlan(MakeScan(left, ls).As("l").Join(projected, JoinInner, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")}))
	if err != nil {
		t.Fatal(err)
	}
	scope := mem.MakeAllocationScope(a, 16<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	state := joinState{scope: scope}
	state.keys.setScope(scope)
	step := p.steps[0]
	result := state.build(context.Background(), scoped, step.join, 16<<20)
	if result.Code() != ExecutionCompleted {
		t.Fatalf("nested build %v/%v", result.Code(), result.Err())
	}
	owned := map[*mem.Segment]bool{}
	for _, segment := range []*mem.Segment{state.rows, state.head, state.next, state.keys.index.slots, state.keys.keys.data, state.keys.keys.refs, state.keys.rows.values, state.keys.rows.strings, state.keys.scratch} {
		if segment != nil {
			owned[segment] = true
		}
	}
	payloadBytes := int64(0)
	for _, batch := range state.batches {
		v := batch.VectorAt(1)
		for _, segment := range []*mem.Segment{v.Data(), v.Backing()} {
			if segment != nil {
				owned[segment] = true
				payloadBytes += int64(segment.Len())
			}
		}
		if v.Validity() != nil {
			t.Fatal("non-null concatenation unexpectedly nullable")
		}
	}
	var expected int64
	for segment := range owned {
		expected += int64(segment.Len())
	}
	if payloadBytes == 0 || scope.Live() != expected || scope.Peak() < expected || scope.Peak() > scope.Limit() {
		t.Fatalf("nested build accounting live=%d peak=%d expected=%d newPayload=%d", scope.Live(), scope.Peak(), expected, payloadBytes)
	}
	var retained *store.Batch
	result = state.probe(context.Background(), scoped, left.BatchAt(0), step, 16<<20, func(batch *store.Batch) ExecutionResult {
		retained = batch.Retain()
		batch.Release()
		return ExecutionResult{code: ExecutionStopped}
	})
	if result.Code() != ExecutionStopped || retained == nil {
		t.Fatalf("probe retain %v", result.Code())
	}
	state.release()
	p.Release()
	left.Release()
	right.Release()
	if scope.Live() <= 0 {
		t.Fatal("retained owned output lost its scope reservation")
	}
	if retained.VectorAt(6).StringAt(0).View() == "" {
		t.Fatal("retained nested payload lost")
	}
	retained.Release()
	if scope.Live() != 0 {
		t.Fatalf("nested build/output leaked %d bytes", scope.Live())
	}
	usage := a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
