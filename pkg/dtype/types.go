package dtype

// Int32T is a 32-bit signed integer type.
type Int32T struct{}

func (t *Int32T) String() string   { return "int32_t" }
func (t *Int32T) ID() TID          { return INT32T }
func (t *Int32T) Size1() int       { return 32 }
func (t *Int32T) Size8() int       { return 4 }
func (t *Int32T) IsNumeric() bool  { return true }
func (t *Int32T) IsNumericT() bool { return true }

// Int64T is a 64-bit signed integer type.
type Int64T struct{}

func (t *Int64T) String() string   { return "int64_t" }
func (t *Int64T) ID() TID          { return INT64T }
func (t *Int64T) Size1() int       { return 64 }
func (t *Int64T) Size8() int       { return 8 }
func (t *Int64T) IsNumeric() bool  { return true }
func (t *Int64T) IsNumericT() bool { return true }

// Float32T is a 32-bit floating-point type.
type Float32T struct{}

func (t *Float32T) String() string   { return "float32_t" }
func (t *Float32T) ID() TID          { return FLOAT32T }
func (t *Float32T) Size1() int       { return 32 }
func (t *Float32T) Size8() int       { return 4 }
func (t *Float32T) IsNumeric() bool  { return true }
func (t *Float32T) IsNumericT() bool { return true }

// Float64T is a 64-bit floating-point type.
type Float64T struct{}

func (t *Float64T) String() string   { return "float64_t" }
func (t *Float64T) ID() TID          { return FLOAT64T }
func (t *Float64T) Size1() int       { return 64 }
func (t *Float64T) Size8() int       { return 8 }
func (t *Float64T) IsNumeric() bool  { return true }
func (t *Float64T) IsNumericT() bool { return true }

// DateT is a 32-bit signed integer type.
type DateT struct{}

func (t *DateT) String() string   { return "date_t" }
func (t *DateT) ID() TID          { return DATET }
func (t *DateT) Size1() int       { return 32 }
func (t *DateT) Size8() int       { return 4 }
func (t *DateT) IsNumeric() bool  { return true }
func (t *DateT) IsNumericT() bool { return false }

// TimestampTZT is a 32-bit signed integer type.
type TimestampTZT struct{}

func (t *TimestampTZT) String() string   { return "timestamptz_t" }
func (t *TimestampTZT) ID() TID          { return TIMESTAMPTZT }
func (t *TimestampTZT) Size1() int       { return 64 }
func (t *TimestampTZT) Size8() int       { return 8 }
func (t *TimestampTZT) IsNumeric() bool  { return true }
func (t *TimestampTZT) IsNumericT() bool { return false }

// StringT is a string type.
type StringT struct{}

func (t *StringT) String() string   { return "string_t" }
func (t *StringT) ID() TID          { return STRT }
func (t *StringT) Size1() int       { return -1 }
func (t *StringT) Size8() int       { return -1 }
func (t *StringT) IsNumeric() bool  { return false }
func (t *StringT) IsNumericT() bool { return false }

// BoolT is a boolean type.
type BoolT struct{}

func (t *BoolT) String() string   { return "bool_t" }
func (t *BoolT) ID() TID          { return BOOLT }
func (t *BoolT) Size1() int       { return 1 }
func (t *BoolT) Size8() int       { return -1 }
func (t *BoolT) IsNumeric() bool  { return false }
func (t *BoolT) IsNumericT() bool { return false }
