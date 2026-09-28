package csv

import (
	"bytes"
	"context"
	"encoding/csv"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func BenchmarkCSVFieldClassification(b *testing.B) {
	rng := rand.New(rand.NewPCG(418, 51))
	line := make([]byte, 4096)
	for i := range line {
		line[i] = 'a'
		if rng.IntN(18) == 0 {
			line[i] = ','
		}
	}
	for _, test := range []struct {
		name     string
		classify csvClassifier
	}{
		{"scalar", classifyCSVScalar}, {"selected", classifyCSV},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(line)))
			for range b.N {
				var separators uint64
				for i := 0; i < len(line); i += 64 {
					masks := test.classify(line[i:i+64], ',')
					if masks.quotes != 0 {
						b.Fatal("unexpected quote")
					}
					separators |= masks.delimiters
				}
				if separators == 0 {
					b.Fatal("incorrect delimiter classification")
				}
			}
		})
	}
}

func BenchmarkCSVReader(b *testing.B) {
	for _, name := range []string{"unquoted", "long-unquoted", "quoted"} {
		b.Run(name, func(b *testing.B) {
			b.StopTimer()
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			var buffer bytes.Buffer
			writer := csv.NewWriter(&buffer)
			for i := range 8192 {
				text := "sensor=" + strconv.Itoa(i)
				if name == "long-unquoted" {
					text = strings.Repeat("sensor", 20) + "=" + strconv.Itoa(i)
				}
				if name == "quoted" {
					text += " , quoted"
				}
				if err := writer.Write([]string{text, strconv.Itoa(i), "2024-02-29", strconv.Itoa(i % 2)}); err != nil {
					b.Fatal(err)
				}
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				b.Fatal(err)
			}
			input := buffer.Bytes()
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			b.StartTimer()
			for range b.N {
				reader, err := MakeCSVReader(bytes.NewReader(input), a, []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.DateT(), dtype.BoolT()}, []bool{false, false, false, false}, CSVOptions{})
				if err != nil {
					b.Fatal(err)
				}
				count := 0
				for {
					batch, err := reader.Next(context.Background())
					if err != nil {
						b.Fatal(err)
					}
					if batch == nil {
						break
					}
					count += batch.Len()
					batch.Release()
				}
				if count != 8192 {
					b.Fatalf("got %d rows", count)
				}
			}
		})
	}
}
