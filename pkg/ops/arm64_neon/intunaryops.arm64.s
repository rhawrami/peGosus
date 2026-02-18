//go:build arm64

#include "textflag.h"

#define vOpUnaryInt(w1, w2, w3, w4, dSize, chnkSize, spec, spec1) \
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

// func absI64Vec(src, dst []int64)
// w1: $0x4ee0b825 => 'abs.2d v5, v1'
// w2: $0x4ee0b846 => 'abs.2d v6, v2'
// w3: $0x4ee0b867 => 'abs.2d v7, v3'
// w4: $0x4ee0b888 => 'abs.2d v8, v4'
TEXT ·absI64Vec(SB),NOSPLIT,$0-48
    vOpUnaryInt($0x4ee0b825, $0x4ee0b846, $0x4ee0b867, $0x4ee0b888, $8, $8, D2, D)

// func absI32Vec(src, dst []int32)
// w1: $0x4ea0b825 => 'abs.4s v5, v1'
// w2: $0x4ea0b846 => 'abs.4s v6, v2'
// w3: $0x4ea0b867 => 'abs.4s v7, v3'
// w4: $0x4ea0b888 => 'abs.4s v8, v4'
TEXT ·absI32Vec(SB),NOSPLIT,$0-48
    vOpUnaryInt($0x4ea0b825, $0x4ea0b846, $0x4ea0b867, $0x4ea0b888, $4, $16, S4, S)

// func negI64Vec(src, dst []int64)
// w1: $0x6ee0b825 => 'neg.2d v5, v1'
// w2: $0x6ee0b846 => 'neg.2d v6, v2'
// w3: $0x6ee0b867 => 'neg.2d v7, v3'
// w4: $0x6ee0b888 => 'neg.2d v8, v4'
TEXT ·negI64Vec(SB),NOSPLIT,$0-48
    vOpUnaryInt($0x6ee0b825, $0x6ee0b846, $0x6ee0b867, $0x6ee0b888, $8, $8, D2, D)

// func negI32Vec(src, dst []int32)
// w1: $0x6ea0b825 => 'neg.4s v5, v1'
// w2: $0x6ea0b846 => 'neg.4s v6, v2'
// w3: $0x6ea0b867 => 'neg.4s v7, v3'
// w4: $0x6ea0b888 => 'neg.4s v8, v4'
TEXT ·negI32Vec(SB),NOSPLIT,$0-48
    vOpUnaryInt($0x6ea0b825, $0x6ea0b846, $0x6ea0b867, $0x6ea0b888, $4, $16, S4, S)

// func sqI32Vec(src, dst []int32)
// w1: $0x4ea19c25 => 'mul.4s v5, v1, v1'
// w2: $0x4ea29c46 => 'mul.4s v6, v2, v2'
// w3: $0x4ea39c67 => 'mul.4s v7, v3, v3'
// w4: $0x4ea49c88 => 'mul.4s v8, v4, v4'
TEXT ·sqI32Vec(SB),NOSPLIT,$0-48
    vOpUnaryInt($0x4ea19c25, $0x4ea29c46, $0x4ea39c67, $0x4ea49c88, $4, $16, S4, S)

#define vOpSqInt(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, dSize, chnkSize, spec, spec1) \
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
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    WORD w9                                                \
    WORD w10                                               \
    WORD w11                                               \
    WORD w12                                               \
    VST1.P [V13.spec, V14.spec, V15.spec, V16.spec], 64(R1)\  
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    VST1 V13.spec1[0], (R1)                                \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func sqI64Vec(src, dst []int64)
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x6e65dca6 => 'fmul.2d v6, v5, v5'
// w3: $0x4e61c8cd => 'fcvtas.2d v13, v6'
// w4: $0x4e61d847 => 'scvtf.2d v7, v2'
// w5: $0x6e67dce8 => 'fmul.2d v8, v7, v7'
// w6: $0x4e61c90e => 'fcvtas.2d v14, v8'
// w7: $0x4e61d869 => 'scvtf.2d v9, v3'
// w8: $0x6e69dd2a => 'fmul.2d v10, v9, v9'
// w9: $0x4e61c94f => 'fcvtas.2d v15, v10'
// w10: $0x4e61d88b => 'scvtf.2d v11, v4'
// w11: $0x6e6bdd6c => 'fmul.2d v12, v11, v11'
// w12: $0x4e61c990 => 'fcvtas.2d v16, v12'
TEXT ·sqI64Vec(SB),NOSPLIT,$0-48
    vOpSqInt($0x4e61d825, $0x6e65dca6, $0x4e61c8cd, $0x4e61d847, $0x6e67dce8, $0x4e61c90e, $0x4e61d869, $0x6e69dd2a, $0x4e61c94f, $0x4e61d88b, $0x6e6bdd6c, $0x4e61c990, $8, $8, D2, D)

#define vSqrtInt(w1, w2, w3, w4, w5, w6, w7, w8, dSize, chnkSize, spec, spec1) \
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
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P [V9.spec, V10.spec, V11.spec, V12.spec], 64(R1) \  
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    VST1 V9.spec1[0], (R1)                                 \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func sqrtI64Vec(src []int64, dst []float64)
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x6ee1f8a9 => 'fsqrt.2d v9, v5'
// w3: $0x4e61d846 => 'scvtf.2d v6, v2'
// w4: $0x6ee1f8ca => 'fsqrt.2d v10, v6'
// w5: $0x4e61d867 => 'scvtf.2d v7, v3'
// w6: $0x6ee1f8eb => 'fsqrt.2d v11, v7'
// w7: $0x4e61d888 => 'scvtf.2d v8, v4'
// w8: $0x6ee1f90c => 'fsqrt.2d v12, v8'
TEXT ·sqrtI64Vec(SB),NOSPLIT,$0-48
    vSqrtInt($0x4e61d825, $0x6ee1f8a9, $0x4e61d846, $0x6ee1f8ca, $0x4e61d867, $0x6ee1f8eb, $0x4e61d888, $0x6ee1f90c, $8, $8, D2, D)

// func sqrtI32Vec(src []int32, dst []float32)
// w1: $0x4e21d825 => 'scvtf.4s v5, v1'
// w2: $0x6ea1f8a9 => 'fsqrt.4s v9, v5'
// w3: $0x4e21d846 => 'scvtf.4s v6, v2'
// w4: $0x6ea1f8ca => 'fsqrt.4s v10, v6'
// w5: $0x4e21d867 => 'scvtf.4s v7, v3'
// w6: $0x6ea1f8eb => 'fsqrt.4s v11, v7'
// w7: $0x4e21d888 => 'scvtf.4s v8, v4'
// w8: $0x6ea1f90c => 'fsqrt.4s v12, v8'
TEXT ·sqrtI32Vec(SB),NOSPLIT,$0-48
    vSqrtInt($0x4e21d825, $0x6ea1f8a9, $0x4e21d846, $0x6ea1f8ca, $0x4e21d867, $0x6ea1f8eb, $0x4e21d888, $0x6ea1f90c, $4, $16, S4, S)

#define vRecipInt(w1, w2, w3, w4, w5, w6, w7, w8, mOp, dSize, chnkSize, spec, spec1) \
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
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P [V9.spec, V10.spec, V11.spec, V12.spec], 64(R1) \  
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    WORD w2                                                \
    VST1 V9.spec1[0], (R1)                                 \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func recipI64Vec(src []int64, dst []float64)
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x6e65fc09 => 'fdiv.2d v9, v0, v5'
// w3: $0x4e61d846 => 'scvtf.2d v6, v2'
// w4: $0x6e66fc0a => 'fdiv.2d v10, v0, v6'
// w5: $0x4e61d867 => 'scvtf.2d v7, v3'
// w6: $0x6e67fc0b => 'fdiv.2d v11, v0, v7'
// w7: $0x4e61d888 => 'scvtf.2d v8, v4'
// w8: $0x6e68fc0c => 'fdiv.2d v12, v0, v8'
TEXT ·recipI64Vec(SB),NOSPLIT,$0-48
    vRecipInt($0x4e61d825, $0x6e65fc09, $0x4e61d846, $0x6e66fc0a, $0x4e61d867, $0x6e67fc0b, $0x4e61d888, $0x6e68fc0c, FMOVD, $8, $8, D2, D)

// func recipI32Vec(src []int32, dst []float32)
// w1: $0x4e21d825 => 'scvtf.4s v5, v1'
// w2: $0x6e25fc09 => 'fdiv.4s v9, v0, v5'
// w3: $0x4e21d846 => 'scvtf.4s v6, v2'
// w4: $0x6e26fc0a => 'fdiv.4s v10, v0, v6'
// w5: $0x4e21d867 => 'scvtf.4s v7, v3'
// w6: $0x6e27fc0b => 'fdiv.4s v11, v0, v7'
// w7: $0x4e21d888 => 'scvtf.4s v8, v4'
// w8: $0x6e28fc0c => 'fdiv.4s v12, v0, v8'
TEXT ·recipI32Vec(SB),NOSPLIT,$0-48
    vRecipInt($0x4e21d825, $0x6e25fc09, $0x4e21d846, $0x6e26fc0a, $0x4e21d867, $0x6e27fc0b, $0x4e21d888, $0x6e28fc0c, FMOVS, $4, $16, S4, S)
