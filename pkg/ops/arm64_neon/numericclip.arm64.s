//go:build arm64

#include "textflag.h"

#define vClipIFWithIFBounds(w1, w2, w3, w4, w5, w6, w7, w8, mOp, dSize, chnkSize, spec, spec1, maxOffset) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp min+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
    mOp max+maxOffset(FP), R6                              \
    VDUP R6, V1.spec                                       \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V2.spec, V3.spec, V4.spec, V5.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P [V10.spec, V11.spec, V12.spec, V13.spec], 64(R1)\
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V2.spec1[0]                                 \
    WORD w1                                                \
    WORD w2                                                \
    VST1 V10.spec1[0], (R1)                                \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func clipF64VecWithF64Bounds(src, dst []float64, lower, upper float64)
// w1: $0x4e60f446 => 'fmax.2d v6, v2, v0'
// w2: $0x4ee1f4ca => 'fmin.2d v10, v6, v1'
// w3: $0x4e60f467 => 'fmax.2d v7, v3, v0'
// w4: $0x4ee1f4eb => 'fmin.2d v11, v7, v1'
// w5: $0x4e60f488 => 'fmax.2d v8, v4, v0'
// w6: $0x4ee1f50c => 'fmin.2d v12, v8, v1'
// w7: $0x4e60f4a9 => 'fmax.2d v9, v5, v0'
// w8: $0x4ee1f52d => 'fmin.2d v13, v9, v1'
TEXT ·clipF64VecWithF64Bounds(SB),NOSPLIT,$0-64
    vClipIFWithIFBounds($0x4e60f446, $0x4ee1f4ca, $0x4e60f467, $0x4ee1f4eb, $0x4e60f488, $0x4ee1f50c, $0x4e60f4a9, $0x4ee1f52d, MOVD, $8, $8, D2, D, 56)

// func clipF32VecWithF32Bounds(src, dst []float32, lower, upper float32)
// w1: $0x4e20f446 => 'fmax.4s v6, v2, v0'
// w2: $0x4ea1f4ca => 'fmin.4s v10, v6, v1'
// w3: $0x4e20f467 => 'fmax.4s v7, v3, v0'
// w4: $0x4ea1f4eb => 'fmin.4s v11, v7, v1'
// w5: $0x4e20f488 => 'fmax.4s v8, v4, v0'
// w6: $0x4ea1f50c => 'fmin.4s v12, v8, v1'
// w7: $0x4e20f4a9 => 'fmax.4s v9, v5, v0'
// w8: $0x4ea1f52d => 'fmin.4s v13, v9, v1'
TEXT ·clipF32VecWithF32Bounds(SB),NOSPLIT,$0-56
    vClipIFWithIFBounds($0x4e20f446, $0x4ea1f4ca, $0x4e20f467, $0x4ea1f4eb, $0x4e20f488, $0x4ea1f50c, $0x4e20f4a9, $0x4ea1f52d, MOVW, $4, $16, S4, S, 52)

// func clipI32VecWithI32Bounds(src, dst []int32, lower, upper int32)
// w1: $0x4ea06446 => 'smax.4s v6, v2, v0'
// w2: $0x4ea16cca => 'smin.4s v10, v6, v1'
// w3: $0x4ea06467 => 'smax.4s v7, v3, v0'
// w4: $0x4ea16ceb => 'smin.4s v11, v7, v1'
// w5: $0x4ea06488 => 'smax.4s v8, v4, v0'
// w6: $0x4ea16d0c => 'smin.4s v12, v8, v1'
// w7: $0x4ea064a9 => 'smax.4s v9, v5, v0'
// w8: $0x4ea16d2d => 'smin.4s v13, v9, v1'
TEXT ·clipI32VecWithI32Bounds(SB),NOSPLIT,$0-56
    vClipIFWithIFBounds($0x4ea06446, $0x4ea16cca, $0x4ea06467, $0x4ea16ceb, $0x4ea06488, $0x4ea16d0c, $0x4ea064a9, $0x4ea16d2d, MOVW, $4, $16, S4, S, 52)

#define vClipI64(w0_1, w0_2, w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, w13, w14, w15, w16) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVD min+48(FP), R5                                    \
    VDUP R5, V0.D2                                         \
    MOVD max+56(FP), R6                                    \
    VDUP R6, V1.D2                                         \
    WORD w0_1                                              \
    WORD w0_2                                              \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V2.D2, V3.D2, V4.D2, V5.D2]            \
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
    VST1.P [V18.D2, V19.D2, V20.D2, V21.D2], 64(R1)        \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V2.D[0]                                     \
    WORD w1                                                \
    WORD w5                                                \
    WORD w9                                                \
    WORD w13                                               \
    VST1 V18.D[0], (R1)                                    \
    ADD $8, R0, R0                                         \
    ADD $8, R1, R1                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func clipI64VecWithI64Bounds(src, dst []int64, lower, upper int64)
// w0_1: $0x4e61d800 => 'scvtf.2d v0, v0'
// w0_2: $0x4e61d821 => 'scvtf.2d v1, v1'
// w1: $0x4e61d846 => 'scvtf.2d v6, v2'
// w2: $0x4e61d867 => 'scvtf.2d v7, v3'
// w3: $0x4e61d888 => 'scvtf.2d v8, v4'
// w4: $0x4e61d8a9 => 'scvtf.2d v9, v5'
// w5: $0x4e60f4ca => 'fmax.2d v10, v6, v0'
// w6: $0x4e60f4eb => 'fmax.2d v11, v7, v0'
// w7: $0x4e60f50c => 'fmax.2d v12, v8, v0'
// w8: $0x4e60f52d => 'fmax.2d v13, v9, v0'
// w9: $0x4ee1f54e => 'fmin.2d v14, v10, v1'
// w10: $0x4ee1f56f => 'fmin.2d v15, v11, v1'
// w11: $0x4ee1f590 => 'fmin.2d v16, v12, v1'
// w12: $0x4ee1f5b1 => 'fmin.2d v17, v13, v1'
// w13: $0x4e61c9d2 => 'fcvtas.2d v18, v14'
// w14: $0x4e61c9f3 => 'fcvtas.2d v19, v15'
// w15: $0x4e61ca14 => 'fcvtas.2d v20, v16'
// w16: $0x4e61ca35 => 'fcvtas.2d v21, v17'
TEXT ·clipI64VecWithI64Bounds(SB),NOSPLIT,$0-64
    vClipI64($0x4e61d800, $0x4e61d821, $0x4e61d846, $0x4e61d867, $0x4e61d888, $0x4e61d8a9, $0x4e60f4ca, $0x4e60f4eb, $0x4e60f50c, $0x4e60f52d, $0x4ee1f54e, $0x4ee1f56f, $0x4ee1f590, $0x4ee1f5b1, $0x4e61c9d2, $0x4e61c9f3, $0x4e61ca14, $0x4e61ca35)

#define vClipIWithFBounds(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, mOp, dSize, chnkSize, spec, spec1, maxOffset) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp min+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
    mOp max+maxOffset(FP), R6                              \
    VDUP R6, V1.spec                                       \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V2.spec, V3.spec, V4.spec, V5.spec]    \
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
    VST1.P [V14.spec, V15.spec, V16.spec, V17.spec], 64(R1)\
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V2.spec1[0]                                 \
    WORD w1                                                \
    WORD w5                                                \
    WORD w9                                                \
    VST1 V14.spec1[0], (R1)                                \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func clipI64VecWithF64Bounds(src []int64, dst []float64, lower, upper float64)
// w1: $0x4e61d846 => 'scvtf.2d v6, v2'
// w2: $0x4e61d867 => 'scvtf.2d v7, v3'
// w3: $0x4e61d888 => 'scvtf.2d v8, v4'
// w4: $0x4e61d8a9 => 'scvtf.2d v9, v5'
// w5: $0x4e60f4ca => 'fmax.2d v10, v6, v0'
// w6: $0x4e60f4eb => 'fmax.2d v11, v7, v0'
// w7: $0x4e60f50c => 'fmax.2d v12, v8, v0'
// w8: $0x4e60f52d => 'fmax.2d v13, v9, v0'
// w9: $0x4ee1f54e => 'fmin.2d v14, v10, v1'
// w10: $0x4ee1f56f => 'fmin.2d v15, v11, v1'
// w11: $0x4ee1f590 => 'fmin.2d v16, v12, v1'
// w12: $0x4ee1f5b1 => 'fmin.2d v17, v13, v1'
TEXT ·clipI64VecWithF64Bounds(SB),NOSPLIT,$0-64
    vClipIWithFBounds($0x4e61d846, $0x4e61d867, $0x4e61d888, $0x4e61d8a9, $0x4e60f4ca, $0x4e60f4eb, $0x4e60f50c, $0x4e60f52d, $0x4ee1f54e, $0x4ee1f56f, $0x4ee1f590, $0x4ee1f5b1, MOVD, $8, $8, D2, D, 56)

// func clipI32VecWithF32Bounds(src []int32, dst []float32, lower, upper float32)
// w1: $0x4e21d846 => 'scvtf.4s v6, v2'
// w2: $0x4e21d867 => 'scvtf.4s v7, v3'
// w3: $0x4e21d888 => 'scvtf.4s v8, v4'
// w4: $0x4e21d8a9 => 'scvtf.4s v9, v5'
// w5: $0x4e20f4ca => 'fmax.4s v10, v6, v0'
// w6: $0x4e20f4eb => 'fmax.4s v11, v7, v0'
// w7: $0x4e20f50c => 'fmax.4s v12, v8, v0'
// w8: $0x4e20f52d => 'fmax.4s v13, v9, v0'
// w9: $0x4ea1f54e => 'fmin.4s v14, v10, v1'
// w10: $0x4ea1f56f => 'fmin.4s v15, v11, v1'
// w11: $0x4ea1f590 => 'fmin.4s v16, v12, v1'
// w12: $0x4ea1f5b1 => 'fmin.4s v17, v13, v1'
TEXT ·clipI32VecWithF32Bounds(SB),NOSPLIT,$0-56
    vClipIWithFBounds($0x4e21d846, $0x4e21d867, $0x4e21d888, $0x4e21d8a9, $0x4e20f4ca, $0x4e20f4eb, $0x4e20f50c, $0x4e20f52d, $0x4ea1f54e, $0x4ea1f56f, $0x4ea1f590, $0x4ea1f5b1, MOVW, $4, $16, S4, S, 52)
