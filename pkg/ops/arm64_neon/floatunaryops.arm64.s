//go:build arm64

#include "textflag.h"

#define vOpUnaryFloat(w1, w2, w3, w4, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V5.spec, V6.spec, V7.spec, V8.spec], 64(R1)    \                                                         
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    VST1 V5.spec1[0], (R1)                                 \
    ADD dSize, R0, R0                                      \ 
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \ 
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func sqrtF64Vec(src, dst []float64)
// w1: $0x6ee1f825 => 'fsqrt.2d v5, v1'
// w2: $0x6ee1f846 => 'fsqrt.2d v6, v2'
// w3: $0x6ee1f867 => 'fsqrt.2d v7, v3'
// w4: $0x6ee1f888 => 'fsqrt.2d v8, v4'
TEXT ·sqrtF64Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6ee1f825, $0x6ee1f846, $0x6ee1f867, $0x6ee1f888, $8, $8, D2, D)

// func sqrtF32Vec(src, dst []float32)
// w1: $0x6ea1f825 => 'fsqrt.4s v5, v1'
// w2: $0x6ea1f846 => 'fsqrt.4s v6, v2'
// w3: $0x6ea1f867 => 'fsqrt.4s v7, v3'
// w4: $0x6ea1f888 => 'fsqrt.4s v8, v4'
TEXT ·sqrtF32Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6ea1f825, $0x6ea1f846, $0x6ea1f867, $0x6ea1f888, $4, $16, S4, S)

// func sqF64Vec(src, dst []float64)
// w1: $0x6e61dc25 => 'fmul.2d v5, v1, v1'
// w2: $0x6e62dc46 => 'fmul.2d v6, v2, v2'
// w3: $0x6e63dc67 => 'fmul.2d v7, v3, v3'
// w4: $0x6e64dc88 => 'fmul.2d v8, v4, v4'
TEXT ·sqF64Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6e61dc25, $0x6e62dc46, $0x6e63dc67, $0x6e64dc88, $8, $8, D2, D)

// func sqF32Vec(src, dst []float32)
// w1: $0x6e21dc25 => 'fmul.4s v5, v1, v1'
// w2: $0x6e22dc46 => 'fmul.4s v6, v2, v2'
// w3: $0x6e23dc67 => 'fmul.4s v7, v3, v3'
// w4: $0x6e24dc88 => 'fmul.4s v8, v4, v4'
TEXT ·sqF32Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6e21dc25, $0x6e22dc46, $0x6e23dc67, $0x6e24dc88, $4, $16, S4, S)

// func absF64Vec(src, dst []float64)
// w1: $0x4ee0f825 => 'fabs.2d v5, v1'
// w2: $0x4ee0f846 => 'fabs.2d v6, v2'
// w3: $0x4ee0f867 => 'fabs.2d v7, v3'
// w4: $0x4ee0f888 => 'fabs.2d v8, v4'
TEXT ·absF64Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x4ee0f825, $0x4ee0f846, $0x4ee0f867, $0x4ee0f888, $8, $8, D2, D)

// func absF32Vec(src, dst []float32)
// w1: $0x4ea0f825 => 'fabs.4s v5, v1'
// w2: $0x4ea0f846 => 'fabs.4s v6, v2'
// w3: $0x4ea0f867 => 'fabs.4s v7, v3'
// w4: $0x4ea0f888 => 'fabs.4s v8, v4'
TEXT ·absF32Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x4ea0f825, $0x4ea0f846, $0x4ea0f867, $0x4ea0f888, $4, $16, S4, S)

// func negF64Vec(src, dst []float64)
// w1: $0x6ee0f825 => 'fneg.2d v5, v1'
// w2: $0x6ee0f846 => 'fneg.2d v6, v2'
// w3: $0x6ee0f867 => 'fneg.2d v7, v3'
// w4: $0x6ee0f888 => 'fneg.2d v8, v4'
TEXT ·negF64Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6ee0f825, $0x6ee0f846, $0x6ee0f867, $0x6ee0f888, $8, $8, D2, D)

// func negF32Vec(src, dst []float32)
// w1: $0x6ea0f825 => 'fneg.4s v5, v1'
// w2: $0x6ea0f846 => 'fneg.4s v6, v2'
// w3: $0x6ea0f867 => 'fneg.4s v7, v3'
// w4: $0x6ea0f888 => 'fneg.4s v8, v4'
TEXT ·negF32Vec(SB),NOSPLIT,$0-48
    vOpUnaryFloat($0x6ea0f825, $0x6ea0f846, $0x6ea0f867, $0x6ea0f888, $4, $16, S4, S)

#define vRecipFloat(w1, w2, w3, w4, mOp, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp $1.00, F0                                          \
    VDUP V0.spec1[0], V0.spec                              \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V5.spec, V6.spec, V7.spec, V8.spec], 64(R1)    \  
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    VST1 V5.spec1[0], (R1)                                 \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func recipF64Vec(src, dst []float64)
// w1: $0x6e61fc05 => 'fdiv.2d	v5, v0, v1'
// w2: $0x6e62fc06 => 'fdiv.2d	v6, v0, v2'
// w3: $0x6e63fc07 => 'fdiv.2d	v7, v0, v3'
// w4: $0x6e64fc08 => 'fdiv.2d	v8, v0, v4'
TEXT ·recipF64Vec(SB),NOSPLIT,$0-48
    vRecipFloat($0x6e61fc05, $0x6e62fc06, $0x6e63fc07, $0x6e64fc08, FMOVD, $8, $8, D2, D)

// func recipF32Vec(src, dst []float32)
// w1: $0x6e21fc05 => 'fdiv.4s	v5, v0, v1'
// w2: $0x6e22fc06 => 'fdiv.4s	v6, v0, v2'
// w3: $0x6e23fc07 => 'fdiv.4s	v7, v0, v3'
// w4: $0x6e24fc08 => 'fdiv.4s	v8, v0, v4'
TEXT ·recipF32Vec(SB),NOSPLIT,$0-48
    vRecipFloat($0x6e21fc05, $0x6e22fc06, $0x6e23fc07, $0x6e24fc08, FMOVS, $4, $16, S4, S)
