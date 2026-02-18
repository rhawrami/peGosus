//go:build arm64

#include "textflag.h"

#define vBinaryOpIntLit(w1, w2, w3, w4, mOp, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
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
                                                           \
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

// func addI64VecI64Lit(src, dst []int64, lit int64)
// w1: $0x4ee08425 => 'add.2d v5, v1, v0'
// w2: $0x4ee08446 => 'add.2d v6, v2, v0'
// w3: $0x4ee08467 => 'add.2d v7, v3, v0'
// w4: $0x4ee08488 => 'add.2d v8, v4, v0'
TEXT ·addI64VecI64Lit(SB),NOSPLIT,$0-56
    vBinaryOpIntLit($0x4ee08425, $0x4ee08446, $0x4ee08467, $0x4ee08488, MOVD, $8, $8, D2, D)

// func addI32VecI32Lit(src, dst []int32, lit int32)
// w1: $0x4ea08425 => 'add.4s v5, v1, v0'
// w2: $0x4ea08446 => 'add.4s v6, v2, v0'
// w3: $0x4ea08467 => 'add.4s v7, v3, v0'
// w4: $0x4ea08488 => 'add.4s v8, v4, v0'
TEXT ·addI32VecI32Lit(SB),NOSPLIT,$0-52
    vBinaryOpIntLit($0x4ea08425, $0x4ea08446, $0x4ea08467, $0x4ea08488, MOVW, $4, $16, S4, S)

// func subI64VecI64Lit(src, dst []int64, lit int64)
// w1: $0x6ee08425 => 'sub.2d v5, v1, v0'
// w2: $0x6ee08446 => 'sub.2d v6, v2, v0'
// w3: $0x6ee08467 => 'sub.2d v7, v3, v0'
// w4: $0x6ee08488 => 'sub.2d v8, v4, v0'
TEXT ·subI64VecI64Lit(SB),NOSPLIT,$0-56
    vBinaryOpIntLit($0x6ee08425, $0x6ee08446, $0x6ee08467, $0x6ee08488, MOVD, $8, $8, D2, D)

// func subI32VecI32Lit(src, dst []int32, lit int32)
// w1: $0x6ea08425 => 'sub.4s v5, v1, v0'
// w2: $0x6ea08446 => 'sub.4s v6, v2, v0'
// w3: $0x6ea08467 => 'sub.4s v7, v3, v0'
// w4: $0x6ea08488 => 'sub.4s v8, v4, v0'
TEXT ·subI32VecI32Lit(SB),NOSPLIT,$0-52
    vBinaryOpIntLit($0x6ea08425, $0x6ea08446, $0x6ea08467, $0x6ea08488, MOVW, $4, $16, S4, S)

// func mulI32VecI32Lit(src, dst []int32, lit int32)
// w1: $0x4ea09c25 => 'mul.4s v5, v1, v0'
// w2: $0x4ea09c46 => 'mul.4s v6, v2, v0'
// w3: $0x4ea09c67 => 'mul.4s v7, v3, v0'
// w4: $0x4ea09c88 => 'mul.4s v8, v4, v0'
TEXT ·mulI32VecI32Lit(SB),NOSPLIT,$0-52
    vBinaryOpIntLit($0x4ea09c25, $0x4ea09c46, $0x4ea09c67, $0x4ea09c88, MOVW, $4, $16, S4, S)

#define vMulIntLit(w0, w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, mOp, dSize, chnkSize, spec, spec1)   \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
    WORD w0                                                \
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
                                                           \
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

// func mulI64VecI64Lit(src, dst []int64, lit int64)
// w0: $0x4e61d800 => 'scvtf.2d v0, v0'
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x6e60dca6 => 'fmul.2d v6, v5, v0'
// w3: $0x4e61c8cd => 'fcvtas.2d v13, v6'
// w4: $0x4e61d847 => 'scvtf.2d v7, v2'
// w5: $0x6e60dce8 => 'fmul.2d v8, v7, v0'
// w6: $0x4e61c90e => 'fcvtas.2d v14, v8'
// w7: $0x4e61d869 => 'scvtf.2d v9, v3'
// w8: $0x6e60dd2a => 'fmul.2d v10, v9, v0'
// w9: $0x4e61c94f => 'fcvtas.2d v15, v10'
// w10: $0x4e61d88b => 'scvtf.2d v11, v4'
// w11: $0x6e60dd6c => 'fmul.2d v12, v11, v0'
// w12: $0x4e61c990 => 'fcvtas.2d v16, v12'
TEXT ·mulI64VecI64Lit(SB),NOSPLIT,$0-56
    vMulIntLit($0x4e61d800, $0x4e61d825, $0x6e60dca6, $0x4e61c8cd, $0x4e61d847, $0x6e60dce8, $0x4e61c90e, $0x4e61d869, $0x6e60dd2a, $0x4e61c94f, $0x4e61d88b, $0x6e60dd6c, $0x4e61c990, MOVD, $8, $8, D2, D)

#define vBinaryOpIntVec(w1, w2, w3, w4, mOp, dSize, chnkSize, spec, spec1) \
    MOVD src1Addr+0(FP), R0                                \
    MOVD src2Addr+24(FP), R1                               \
    MOVD dstAddr+48(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB chnkSize, R3, R5                                   \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R3                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    VLD1.P 64(R1), [V5.spec, V6.spec, V7.spec, V8.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V9.spec, V10.spec, V11.spec, V12.spec], 64(R2) \  
    ADD chnkSize, R4, R4                                   \
    CMP R5, R4                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    VLD1 (R1), V5.spec1[0]                                 \
    WORD w1                                                \
    VST1 V9.spec1[0], (R2)                                 \
    ADD dSize, R0, R0                                      \ 
    ADD dSize, R1, R1                                      \
    ADD dSize, R2, R2                                      \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addI64VecI64Vec(src1, src2, dst []float64)
// w1: $0x4ee58429 => 'add.2d v9, v1, v5'
// w2: $0x4ee6844a => 'add.2d v10, v2, v6'
// w3: $0x4ee7846b => 'add.2d v11, v3, v7'
// w4: $0x4ee8848c => 'add.2d v12, v4, v8'
TEXT ·addI64VecI64Vec(SB),NOSPLIT,$0-72
    vBinaryOpIntVec($0x4ee58429, $0x4ee6844a, $0x4ee7846b, $0x4ee8848c, MOVD, $8, $8, D2, D)

// func addI32VecI32Vec(src1, src2, dst []float32)
// w1: $0x4ea58429 => 'add.4s v9, v1, v5'
// w2: $0x4ea6844a => 'add.4s v10, v2, v6'
// w3: $0x4ea7846b => 'add.4s v11, v3, v7'
// w4: $0x4ea8848c => 'add.4s v12, v4, v8'
TEXT ·addI32VecI32Vec(SB),NOSPLIT,$0-72
    vBinaryOpIntVec($0x4ea58429, $0x4ea6844a, $0x4ea7846b, $0x4ea8848c, MOVW, $4, $16, S4, S)

// func subI64VecI64Vec(src1, src2, dst []float64)
// w1: $0x6ee58429 => 'sub.2d v9, v1, v5'
// w2: $0x6ee6844a => 'sub.2d v10, v2, v6'
// w3: $0x6ee7846b => 'sub.2d v11, v3, v7'
// w4: $0x6ee8848c => 'sub.2d v12, v4, v8'
TEXT ·subI64VecI64Vec(SB),NOSPLIT,$0-72
    vBinaryOpIntVec($0x6ee58429, $0x6ee6844a, $0x6ee7846b, $0x6ee8848c, MOVD, $8, $8, D2, D)

// func subI32VecI32Vec(src1, src2, dst []float32)
// w1: $0x6ea58429 => 'sub.4s v9, v1, v5'
// w2: $0x6ea6844a => 'sub.4s v10, v2, v6'
// w3: $0x6ea7846b => 'sub.4s v11, v3, v7'
// w4: $0x6ea8848c => 'sub.4s v12, v4, v8'
TEXT ·subI32VecI32Vec(SB),NOSPLIT,$0-72
    vBinaryOpIntVec($0x6ea58429, $0x6ea6844a, $0x6ea7846b, $0x6ea8848c, MOVW, $4, $16, S4, S)

// func mulI32VecI32Vec(src1, src2, dst []float32)
// w1: $0x4ea59c29 => 'mul.4s v9, v1, v5'
// w2: $0x4ea69c4a => 'mul.4s v10, v2, v6'
// w3: $0x4ea79c6b => 'mul.4s v11, v3, v7'
// w4: $0x4ea89c8c => 'mul.4s v12, v4, v8'
TEXT ·mulI32VecI32Vec(SB),NOSPLIT,$0-72
    vBinaryOpIntVec($0x4ea59c29, $0x4ea69c4a, $0x4ea79c6b, $0x4ea89c8c, MOVW, $4, $16, S4, S)

#define vMulIntVec(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, w13, w14, w15, w16, mOp, dSize, chnkSize, spec, spec1)   \
    MOVD src1Addr+0(FP), R0                                \
    MOVD src2Addr+24(FP), R1                               \
    MOVD dstAddr+48(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB chnkSize, R3, R5                                   \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R3                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    VLD1.P 64(R1), [V5.spec, V6.spec, V7.spec, V8.spec]    \
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
    WORD w13                                               \
    WORD w14                                               \
    WORD w15                                               \
    WORD w16                                               \
    VST1.P [V22.spec, V23.spec, V24.spec, V25.spec], 64(R2)\
    ADD chnkSize, R4, R4                                   \
    CMP R5, R4                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    VLD1 (R1), V5.spec1[0]                                 \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1 V22.spec1[0], (R2)                                \
    ADD dSize, R0, R0                                      \ 
    ADD dSize, R1, R1                                      \
    ADD dSize, R2, R2                                      \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func mulI64VecI64Vec(src1, src2, dst []int64)
// w1: $0x4e61d829 => 'scvtf.2d v9, v1'
// w2: $0x4e61d8aa => 'scvtf.2d v10, v5'
// w3: $0x6e6add2b => 'fmul.2d v11, v9, v10'
// w4: $0x4e61c976 => 'fcvtas.2d v22, v11'
// w5: $0x4e61d84d => 'scvtf.2d v13, v2'
// w6: $0x4e61d8ce => 'scvtf.2d v14, v6'
// w7: $0x6e6eddaf => 'fmul.2d v15, v13, v14'
// w8: $0x4e61c9f7 => 'fcvtas.2d v23, v15'
// w9: $0x4e61d870 => 'scvtf.2d v16, v3'
// w10: $0x4e61d8f1 => 'scvtf.2d v17, v7'
// w11: $0x6e71de12 => 'fmul.2d v18, v16, v17'
// w12: $0x4e61ca58 => 'fcvtas.2d v24, v18'
// w13: $0x4e61d893 => 'scvtf.2d v19, v4'
// w14: $0x4e61d914 => 'scvtf.2d v20, v8'
// w15: $0x6e74de75 => 'fmul.2d v21, v19, v20'
// w16: $0x4e61cab9 => 'fcvtas.2d v25, v21'
TEXT ·mulI64VecI64Vec(SB),NOSPLIT,$0-72
    vMulIntVec($0x4e61d829, $0x4e61d8aa, $0x6e6add2b, $0x4e61c976, $0x4e61d84d, $0x4e61d8ce, $0x6e6eddaf, $0x4e61c9f7, $0x4e61d870, $0x4e61d8f1, $0x6e71de12, $0x4e61ca58, $0x4e61d893, $0x4e61d914, $0x6e74de75, $0x4e61cab9, MOVD, $8, $8, D2, D)

#define vDivIntLit(w0, w1, w2, w3, w4, w5, w6, w7, w8, mOp, dSize, chnkSize, spec, spec1)   \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
    WORD w0                                                \
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
                                                           \
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

// func divI64VecI64Lit(src []int64, dst []float64, lit int64)
// w0: $0x4e61d800 => 'scvtf.2d v0, v0'
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x6e60fca9 => 'fdiv.2d v9, v5, v0'
// w3: $0x4e61d846 => 'scvtf.2d v6, v2'
// w4: $0x6e60fcca => 'fdiv.2d v10, v6, v0'
// w5: $0x4e61d867 => 'scvtf.2d v7, v3'
// w6: $0x6e60fceb => 'fdiv.2d v11, v7, v0'
// w7: $0x4e61d888 => 'scvtf.2d v8, v4'
// w8: $0x6e60fd0c => 'fdiv.2d v12, v8, v0'
TEXT ·divI64VecI64Lit(SB),NOSPLIT,$0-56
    vDivIntLit($0x4e61d800, $0x4e61d825, $0x6e60fca9, $0x4e61d846, $0x6e60fcca, $0x4e61d867, $0x6e60fceb, $0x4e61d888, $0x6e60fd0c, MOVD, $8, $8, D2, D)

// func divI32VecI32Lit(src []int64, dst []float32, lit int32)
// w0: $0x4e21d800 => 'scvtf.4s v0, v0'
// w1: $0x4e21d825 => 'scvtf.s4 v5, v1'
// w2: $0x6e20fca9 => 'fdiv.s4 v9, v5, v0'
// w3: $0x4e21d846 => 'scvtf.s4 v6, v2'
// w4: $0x6e20fcca => 'fdiv.s4 v10, v6, v0'
// w5: $0x4e21d867 => 'scvtf.s4 v7, v3'
// w6: $0x6e20fceb => 'fdiv.s4 v11, v7, v0'
// w7: $0x4e21d888 => 'scvtf.s4 v8, v4'
// w8: $0x6e20fd0c => 'fdiv.s4 v12, v8, v0'
TEXT ·divI32VecI32Lit(SB),NOSPLIT,$0-56
    vDivIntLit($0x4e21d800, $0x4e21d825, $0x6e20fca9, $0x4e21d846, $0x6e20fcca, $0x4e21d867, $0x6e20fceb, $0x4e21d888, $0x6e20fd0c, MOVW, $4, $16, S4, S)

#define vDivIntVec(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, mOp, dSize, chnkSize, spec, spec1)   \
    MOVD src1Addr+0(FP), R0                                \
    MOVD src2Addr+24(FP), R1                               \
    MOVD dstAddr+48(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB chnkSize, R3, R5                                   \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R3                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    VLD1.P 64(R1), [V5.spec, V6.spec, V7.spec, V8.spec]    \
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
    VST1.P [V17.spec, V18.spec, V19.spec, V20.spec], 64(R2)\
    ADD chnkSize, R4, R4                                   \
    CMP R5, R4                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    VLD1 (R1), V5.spec1[0]                                 \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1 V17.spec1[0], (R2)                                \
    ADD dSize, R0, R0                                      \ 
    ADD dSize, R1, R1                                      \
    ADD dSize, R2, R2                                      \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func divI64VecI64Vec(src1, src2 []int64, dst []float64)
// w1: $0x4e61d829 => 'scvtf.2d v9, v1'
// w2: $0x4e61d8aa => 'scvtf.2d v10, v5'
// w3: $0x6e6afd31 => 'fdiv.2d v17, v9, v10'
// w4: $0x4e61d84b => 'scvtf.2d v11, v2'
// w5: $0x4e61d8cc => 'scvtf.2d v12, v6'
// w6: $0x6e6cfd72 => 'fdiv.2d v18, v11, v12'
// w7: $0x4e61d86d => 'scvtf.2d v13, v3'
// w8: $0x4e61d8ee => 'scvtf.2d v14, v7'
// w9: $0x6e6efdb3 => 'fdiv.2d v19, v13, v14'
// w10: $0x4e61d88f => 'scvtf.2d v15, v4'
// w11: $0x4e61d910 => 'scvtf.2d v16, v8'
// w12: $0x6e70fdf4 => 'fdiv.2d v20, v15, v16'
TEXT ·divI64VecI64Vec(SB),NOSPLIT,$0-72
    vDivIntVec($0x4e61d829, $0x4e61d8aa, $0x6e6afd31, $0x4e61d84b, $0x4e61d8cc, $0x6e6cfd72, $0x4e61d86d, $0x4e61d8ee, $0x6e6efdb3, $0x4e61d88f, $0x4e61d910, $0x6e70fdf4, MOVD, $8, $8, D2, D)

// func divI32VecI32Vec(src1, src2 []int32, dst []float32)
// w1: $0x4e21d829 => 'scvtf.4s v9, v1'
// w2: $0x4e21d8aa => 'scvtf.4s v10, v5'
// w3: $0x6e2afd31 => 'fdiv.4s v17, v9, v10'
// w4: $0x4e21d84b => 'scvtf.4s v11, v2'
// w5: $0x4e21d8cc => 'scvtf.4s v12, v6'
// w6: $0x6e2cfd72 => 'fdiv.4s v18, v11, v12'
// w7: $0x4e21d86d => 'scvtf.4s v13, v3'
// w8: $0x4e21d8ee => 'scvtf.4s v14, v7'
// w9: $0x6e2efdb3 => 'fdiv.4s v19, v13, v14'
// w10: $0x4e21d88f => 'scvtf.4s v15, v4'
// w11: $0x4e21d910 => 'scvtf.4s v16, v8'
// w12: $0x6e30fdf4 => 'fdiv.4s v20, v15, v16'
TEXT ·divI32VecI32Vec(SB),NOSPLIT,$0-72
    vDivIntVec($0x4e21d829, $0x4e21d8aa, $0x6e2afd31, $0x4e21d84b, $0x4e21d8cc, $0x6e2cfd72, $0x4e21d86d, $0x4e21d8ee, $0x6e2efdb3, $0x4e21d88f, $0x4e21d910, $0x6e30fdf4, MOVW, $4, $16, S4, S)
