# TODO

## vectorized [[op]](./pkg/op/)erations preferred for v1

// **note**: operations should ideally be defined on both arm64's **ASIMD**, and x86-64's **AVX2**.
### numeric
// **note**: when reasonable, numeric operations should be defined for the following types:
1. **int32**
2. **int64**
3. **float32**
4. **float64**

// futher, when reasonable, vertical binary operations should be defined for both a vector and a literal, and a vector and a second vector.

#### binary:
- "add" [lit & vec] => **done**
- "sub" [lit & vec] => **done**
- "mul" [lit & vec]=> **done**
- "div" [lit & vec]=> **done**

#### unary:
- "abs" => **done**
- "neg" => **done**
- "sq" => **done**
- "sqrt" => **done**
- "recip" => **done**
- "round" => not yet :\(

#### casting:
// int32 -> ?
- "I32toI64" => **done**
- "I32toF32" => **done**
- "I32toF64" => **done**

// float32 -> ?
- "F32toF64" => **done**
- "F32toF32" => **done**
- "F32toF64" => **done**

// int64 -> ?
- "I64toF32" => **done**
- "I64toF64" => **done**

// float64 -> ?
- "F64toI32" => **done**
- "F64toI64" => **done**

#### boolean:
- "gt" => *not defined yet on two vec*
- "ge" => *not defined yet on two vec*
- "lt" => *not defined yet on two vec*
- "le" => *not defined yet on two vec*
- "eq" => *not defined yet on two vec*
- "neq" => *not defined yet on two vec*
- "bet" => *not defined yet on three vec*
- "nbet" => *not defined yet on three vec*

#### aggregation:
// note: aggregations should allow for a validity bitmap (packed bitmask representing an element's NULL status) as an input; further, aggregations with floating point data should be able to account for NaNs.
- "min" => **done**
- "max" => **done**
- "minmax" => **done**
- "sum" => not yet :\(
- "count" => not yet :\( (must also account for NaN)
- "mean" => not yet :\(
- "wtdmean" => not yet :\(
- "stddev => not yet :\(

#### other
- "clip" => **done**

### string
// note: for now, vectorized operations will be limited to ASCII-only data.

#### binary:
- "concat" => not yet :\(

#### unary:
- "toUpper" => not yet :\(
- "toLower" => not yet :\(
- "toTitle" => not yet :\(
- "replace" => not yet :\(
- "strip" => not yet :\(
- "stripLeft" => not yet :\(
- "stripRight" => not yet :\(
- "strLen" => not yet :\(


#### boolean:
- "eq" => not yet :\(
- "contains" => not yet :\(

#### aggregation:
// 

### date

// note: given that dates are stored as 32 bit integers, all operations defined on int32 are implicitly also defined on dates.
#### binary:
- "diff" => **done**

#### unary:
- "extractYear" => *not done on x86-64*
- "extractMonth" => *not done on x86-64*
- "extractDay" => *not done on x86-64*
- "extractDayOfWeek" => *not done on x86-64*
- "truncateYear" => *not done on x86-64*
- "truncateMonth" => *not done on x86-64*