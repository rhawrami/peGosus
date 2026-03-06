package array

type NullT struct{}

func (t NullT) Logical() LogicalType { return NULL }
func (t NullT) IsNumeric() bool      { return false }
func (t NullT) IsFixedSize() bool    { return true }
func (t NullT) ByteCount() int       { return 0 }
func (t NullT) BitCount() int        { return 0 }

type Int32T struct{}

func (t Int32T) Logical() LogicalType { return I32 }
func (t Int32T) IsNumeric() bool      { return true }
func (t Int32T) IsFixedSize() bool    { return true }
func (t Int32T) ByteCount() int       { return 4 }
func (t Int32T) BitCount() int        { return 32 }

type Int64T struct{}

func (t Int64T) Logical() LogicalType { return I64 }
func (t Int64T) IsNumeric() bool      { return true }
func (t Int64T) IsFixedSize() bool    { return true }
func (t Int64T) ByteCount() int       { return 8 }
func (t Int64T) BitCount() int        { return 64 }

type Float32T struct{}

func (t Float32T) Logical() LogicalType { return I32 }
func (t Float32T) IsNumeric() bool      { return true }
func (t Float32T) IsFixedSize() bool    { return true }
func (t Float32T) ByteCount() int       { return 4 }
func (t Float32T) BitCount() int        { return 32 }

type Float64T struct{}

func (t Float64T) Logical() LogicalType { return F64 }
func (t Float64T) IsNumeric() bool      { return true }
func (t Float64T) IsFixedSize() bool    { return true }
func (t Float64T) ByteCount() int       { return 8 }
func (t Float64T) BitCount() int        { return 64 }

type DateT struct{}

func (t DateT) Logical() LogicalType { return I32 }
func (t DateT) IsNumeric() bool      { return false }
func (t DateT) IsFixedSize() bool    { return true }
func (t DateT) ByteCount() int       { return 4 }
func (t DateT) BitCount() int        { return 32 }

type StringT struct{}

func (t StringT) Logical() LogicalType { return STR }
func (t StringT) IsNumeric() bool      { return false }
func (t StringT) IsFixedSize() bool    { return false }
func (t StringT) ByteCount() int       { return -1 }
func (t StringT) BitCount() int        { return -1 }

type BoolT struct{}

func (t BoolT) Logical() LogicalType { return BOOL }
func (t BoolT) IsNumeric() bool      { return false }
func (t BoolT) IsFixedSize() bool    { return true }
func (t BoolT) ByteCount() int       { return -1 }
func (t BoolT) BitCount() int        { return 1 }
