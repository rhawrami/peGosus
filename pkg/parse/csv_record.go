package parse

import (
	"bytes"
	"encoding/csv"
)

func decodeCSVFields(record []byte, delimiter byte, expected int, fields [][]byte) ([][]byte, error) {
	fields = fields[:0]
	finalNewline := len(record) > 0 && record[len(record)-1] == '\n'
	if n := len(record); n > 0 {
		if record[n-1] == '\n' {
			record = record[:n-1]
			if len(record) > 0 && record[len(record)-1] == '\r' {
				record = record[:len(record)-1]
			}
		} else if record[n-1] == '\r' {
			record = record[:n-1]
		}
	}
	fieldCount := 0
	for pos := 0; ; {
		if pos < len(record) && record[pos] == '"' {
			start := pos + 1
			write := start
			pos = start
			closed := false
			for pos < len(record) {
				switch record[pos] {
				case '"':
					if pos+1 < len(record) && record[pos+1] == '"' {
						record[write] = '"'
						write++
						pos += 2
						continue
					}
					pos++
					closed = true
				case '\r':
					if pos+1 < len(record) && record[pos+1] == '\n' {
						record[write] = '\n'
						write++
						pos += 2
						continue
					}
				}
				if closed {
					break
				}
				record[write] = record[pos]
				write++
				pos++
			}
			if !closed || pos < len(record) && record[pos] != delimiter {
				location := pos
				if closed {
					location--
				}
				failure := csvRecordError(record, location, csv.ErrQuote)
				if !closed && finalNewline {
					failure.Column++
				}
				return nil, failure
			}
			if fieldCount < expected {
				fields = append(fields, record[start:write])
			}
		} else {
			end := len(record)
			if next := bytes.IndexByte(record[pos:], delimiter); next >= 0 {
				end = pos + next
			}
			if quote := bytes.IndexByte(record[pos:end], '"'); quote >= 0 {
				return nil, csvRecordError(record, pos+quote, csv.ErrBareQuote)
			}
			if fieldCount < expected {
				fields = append(fields, record[pos:end])
			}
			pos = end
		}
		fieldCount++
		if pos == len(record) {
			break
		}
		pos++
	}
	if fieldCount != expected {
		return nil, csv.ErrFieldCount
	}
	return fields, nil
}

func csvRecordError(record []byte, offset int, kind error) *csv.ParseError {
	line := bytes.Count(record[:offset], []byte{'\n'}) + 1
	column := offset + 1
	if previous := bytes.LastIndexByte(record[:offset], '\n'); previous >= 0 {
		column = offset - previous
	}
	return &csv.ParseError{StartLine: 1, Line: line, Column: column, Err: kind}
}
