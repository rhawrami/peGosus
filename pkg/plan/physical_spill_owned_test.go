package plan

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestSpillTakeRecordRetainedFrame(t *testing.T) {
	const budget = 128 << 10
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"text", "float", "null", "bool"}, []dtype.Type{dtype.StringT(), dtype.Float64T(), dtype.Int32T(), dtype.BoolT()})
	text := strings.Repeat("a\x00\xffz", 300)
	body := make([]byte, 36+len(text))
	binary.LittleEndian.PutUint64(body[1:], uint64(len(text)))
	copy(body[9:], text)
	at := 9 + len(text)
	binary.LittleEndian.PutUint64(body[at+1:], 0xfff812345678abcd)
	body[at+9] = 1
	binary.LittleEndian.PutUint64(body[at+19:], 1)
	nextBody := append([]byte(nil), body...)
	copy(nextBody[9:], strings.Repeat("b\x01\xfex", 300))
	wire := append(spillRecordTestWire(body), spillRecordTestWire(nextBody)...)
	stream := openSpillRecordTest(t, context.Background(), scoped, wire, budget)
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("initial %t %v", ok, err)
	}
	frame := stream.recordFrame
	batch, err := stream.takeRecord(scoped, schema)
	if err != nil || batch == nil {
		t.Fatalf("take %v %v", batch, err)
	}
	if stream.recordFrame != nil || stream.recordOffsets != nil || stream.recordBytes() != nil || stream.recordValue(schema, 0).valid() {
		t.Fatal("cursor did not detach")
	}
	if batch.VectorAt(0).Backing() != frame {
		t.Fatal("string frame copied")
	}
	retained := batch.Retain()
	batch.Release()
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("advance %t %v", ok, err)
	}
	if stream.recordFrame == frame {
		t.Fatal("retained frame reused")
	}
	stream.release()
	values := []Scalar{MakeStringScalar(text), {dType: dtype.Float64T(), bits: 0xfff812345678abcd}, MakeNullScalar(dtype.Int32T()), MakeBoolScalar(true)}
	for col, want := range values {
		if got := scalarAt(retained.VectorAt(col), 0); got != want {
			t.Fatalf("col %d got %#v want %#v", col, got, want)
		}
	}
	if scope.Live() == 0 {
		t.Fatal("retained frame not accounted")
	}
	retained.Release()
	if scope.Live() != 0 {
		t.Fatalf("leak %d", scope.Live())
	}
}

func TestSpillTakeRecordDropsOffsetsForAdmission(t *testing.T) {
	const budget = 128 << 10
	const limit = budget/64 + 32<<10 + 64
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, limit)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})
	text := strings.Repeat("large\x00\xff", 4200)
	body := make([]byte, 9+len(text))
	binary.LittleEndian.PutUint64(body[1:], uint64(len(text)))
	copy(body[9:], text)
	stream := openSpillRecordTest(t, context.Background(), scoped, spillRecordTestWire(body), budget)
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("initial %t %v", ok, err)
	}
	if scope.Live() != limit {
		t.Fatalf("live=%d limit=%d", scope.Live(), limit)
	}
	batch, err := stream.takeRecord(scoped, schema)
	if err != nil || batch == nil {
		t.Fatalf("take %v %v", batch, err)
	}
	if got := scalarAt(batch.VectorAt(0), 0).text; got != text {
		t.Fatal("large text changed")
	}
	stream.release()
	batch.Release()
	if scope.Live() != 0 || scope.Exhausted() {
		t.Fatalf("scope live=%d exhausted=%t", scope.Live(), scope.Exhausted())
	}
}

func TestSpillTakeRecordFailureCleanup(t *testing.T) {
	for _, mode := range []string{"cancel", "budget", "unready", "schema"} {
		t.Run(mode, func(t *testing.T) {
			const budget = 64 << 10
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			scope := mem.MakeAllocationScope(a, budget)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			names, types := make([]string, 8), make([]dtype.Type, 8)
			for col := range types {
				types[col] = dtype.StringT()
			}
			schema := MakeSchema(names, types)
			stream := openSpillRecordTest(t, ctx, scoped, spillRecordTestWire(make([]byte, 72)), budget)
			want := errSpillFormat
			if mode != "unready" {
				if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
					t.Fatalf("initial %t %v", ok, err)
				}
			}
			var blocker *mem.Segment
			switch mode {
			case "cancel":
				cancel()
				want = context.Canceled
			case "budget":
				blocker = scoped.AllocSeg(int(scope.Limit() - scope.Live()))
				if blocker == nil {
					t.Fatal("blocker")
				}
				want = errSpillBudget
			case "schema":
				schema = Schema{}
			}
			batch, err := stream.takeRecord(scoped, schema)
			if batch != nil || !errors.Is(err, want) {
				t.Fatalf("take %v %v want %v", batch, err, want)
			}
			if mode == "cancel" || mode == "budget" {
				if stream.recordFrame != nil || stream.recordOffsets != nil || stream.recordBytes() != nil {
					t.Fatal("failed handoff retained frame")
				}
			}
			stream.release()
			if blocker != nil {
				blocker.Dec()
			}
			if scope.Live() != 0 {
				t.Fatalf("leak %d", scope.Live())
			}
		})
	}
}

func TestSpillTakeRecordZeroColumnDomain(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, 64<<10)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema(nil, nil)
	stream := openSpillRecordTest(t, context.Background(), scoped, spillRecordTestWire(nil), 64<<10)
	if ok, err := stream.nextRecord(scoped, schema, 64<<10); !ok || err != nil {
		t.Fatalf("record %t %v", ok, err)
	}
	batch, err := stream.takeRecord(scoped, schema)
	if err != nil || batch == nil || batch.Len() != 1 || batch.NVectors() != 0 {
		t.Fatalf("empty row %v %v", batch, err)
	}
	batch.Release()
	stream.release()
	if scope.Live() != 0 {
		t.Fatalf("leak %d", scope.Live())
	}
}
