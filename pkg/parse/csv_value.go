package parse

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

func setCSVValue(vector *store.Vector, row int, raw string, t dtype.Type) error {
	switch t.ID() {
	case dtype.INT32T, dtype.DATET:
		var value int64
		var err error
		if t.ID() == dtype.DATET {
			var date time.Time
			date, err = time.Parse(time.DateOnly, raw)
			if err == nil {
				value = date.Unix() / 86400
			}
			if value < math.MinInt32 || value > math.MaxInt32 {
				err = strconv.ErrRange
			}
		} else {
			value, err = strconv.ParseInt(raw, 10, 32)
		}
		vector.I32s()[row] = int32(value)
		return err
	case dtype.INT64T:
		value, err := strconv.ParseInt(raw, 10, 64)
		vector.I64s()[row] = value
		return err
	case dtype.TIMESTAMPTZT:
		timestamp, err := time.Parse(time.RFC3339Nano, raw)
		if err == nil {
			vector.I64s()[row] = timestamp.UnixMicro()
		}
		return err
	case dtype.FLOAT32T, dtype.FLOAT64T:
		width := 64
		if t.ID() == dtype.FLOAT32T {
			width = 32
		}
		value, err := strconv.ParseFloat(raw, width)
		if width == 32 {
			vector.F32s()[row] = float32(value)
		} else {
			vector.F64s()[row] = value
		}
		return err
	case dtype.BOOLT:
		value, err := strconv.ParseBool(raw)
		if value {
			vector.Bools()[row>>3] |= 1 << (row & 7)
		}
		return err
	}
	return nil
}

func ownCSVValueError(err error) error {
	var number *strconv.NumError
	if errors.As(err, &number) {
		owned := *number
		owned.Num = strings.Clone(number.Num)
		return &owned
	}
	var timestamp *time.ParseError
	if errors.As(err, &timestamp) {
		owned := *timestamp
		owned.Value = strings.Clone(timestamp.Value)
		owned.ValueElem = strings.Clone(timestamp.ValueElem)
		return &owned
	}
	return err
}
