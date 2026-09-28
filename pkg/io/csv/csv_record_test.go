package csv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"math/rand/v2"
	"testing"
)

func TestCSVRecordDecoderDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(319, 74))
	for length := range 2500 {
		var text bytes.Buffer
		writer := csv.NewWriter(&text)
		values := make([]string, 3)
		for column := range values {
			var value bytes.Buffer
			for range rng.IntN(160) {
				value.WriteByte([]byte{'a', ',', '"', '\r', '\n', 0, 0xff, '#', ' '}[rng.IntN(9)])
			}
			values[column] = value.String()
		}
		if err := writer.Write(values); err != nil {
			t.Fatal(err)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			t.Fatal(err)
		}
		reference := csv.NewReader(bytes.NewReader(text.Bytes()))
		reference.FieldsPerRecord = 3
		want, err := reference.Read()
		if err != nil {
			t.Fatal(err)
		}
		input := append([]byte(nil), text.Bytes()...)
		if length&1 == 0 {
			input = bytes.TrimSuffix(input, []byte{'\n'})
		}
		got, err := decodeCSVFields(input, ',', 3, make([][]byte, 0, 3))
		if err != nil || len(got) != len(want) {
			t.Fatalf("record %d: decode error %v, fields %d vs %d", length, err, len(got), len(want))
		}
		for i, field := range got {
			if string(field) != want[i] {
				t.Fatalf("record %d field %d: got %q, want %q", length, i, field, want[i])
			}
		}
	}
	for _, input := range []string{"one,two,three\n", "one\n", "one\"two,three\n", "\"one\"two,three\n", "\"unterminated\n", "\"one\" \n", "\"one\",two\r", "\"one\r\ntwo\",last\r\n"} {
		reference := csv.NewReader(bytes.NewReader([]byte(input)))
		reference.FieldsPerRecord = 2
		want, wantErr := reference.Read()
		got, err := decodeCSVFields([]byte(input), ',', 2, make([][]byte, 0, 2))
		if wantErr == nil {
			if err != nil || len(got) != len(want) {
				t.Fatalf("input %q: got %v, want %v", input, err, want)
			}
			for i := range want {
				if string(got[i]) != want[i] {
					t.Fatalf("input %q field %d: got %q, want %q", input, i, got[i], want[i])
				}
			}
			continue
		}
		var parseErr *csv.ParseError
		if !errors.As(wantErr, &parseErr) || !errors.Is(err, parseErr.Err) {
			t.Fatalf("input %q: got %v, want %v", input, err, wantErr)
		}
		if parseErr.Err != csv.ErrFieldCount {
			var gotErr *csv.ParseError
			if !errors.As(err, &gotErr) || gotErr.Line != parseErr.Line || gotErr.Column != parseErr.Column {
				t.Fatalf("input %q: got location %v, want %v", input, err, wantErr)
			}
		}
	}
	for trial := range 5000 {
		input := make([]byte, rng.IntN(130))
		for i := range input {
			input[i] = []byte{'a', ',', '"', '\r', ' ', 0, 0xff}[rng.IntN(7)]
		}
		input = append(input, '\n')
		if bytes.Equal(input, []byte{'\n'}) || bytes.Equal(input, []byte{'\r', '\n'}) {
			continue
		}
		reference := csv.NewReader(bytes.NewReader(input))
		reference.FieldsPerRecord = 3
		want, wantErr := reference.Read()
		got, err := decodeCSVFields(append([]byte(nil), input...), ',', 3, make([][]byte, 0, 3))
		if wantErr == nil {
			if err != nil || len(got) != len(want) {
				t.Fatalf("trial %d input %q: got %v, want %v", trial, input, err, want)
			}
			for i := range want {
				if string(got[i]) != want[i] {
					t.Fatalf("trial %d input %q: field %d got %q, want %q", trial, input, i, got[i], want[i])
				}
			}
			continue
		}
		var parseErr *csv.ParseError
		if !errors.As(wantErr, &parseErr) || !errors.Is(err, parseErr.Err) {
			t.Fatalf("trial %d input %q: got %v, want %v", trial, input, err, wantErr)
		}
		if parseErr.Err != csv.ErrFieldCount {
			var gotErr *csv.ParseError
			if !errors.As(err, &gotErr) || gotErr.Line != parseErr.Line || gotErr.Column != parseErr.Column {
				t.Fatalf("trial %d input %q: got location %v, want %v", trial, input, err, wantErr)
			}
		}
	}
}
