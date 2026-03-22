//go:build arm64

#include "textflag.h"

#define boundsISFSD(w1, w2, w3, w4, w5, w6, w7, w8, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    VLD1R (R0), [V0.spec]                                  \
    VDUP V0.spec1[0], V1.spec                              \
    VDUP V0.spec1[0], V2.spec                              \
    VDUP V0.spec1[0], V3.spec                              \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V4.spec, V5.spec, V6.spec, V7.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \  
                                                           \
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1R (R0), [V4.spec]                                  \
    WORD w1                                                \
    ADD dSize, R0, R0                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1 V7.spec1[0], (R1)                                 \
exitFn:                                                    \
    RET      

// func maxI32(src, dst []int32)
// w1: $0x4ea46400 => 'smax.4s v0, v0, v4'
// w2: $0x4ea56421 => 'smax.4s v1, v1, v5'
// w3: $0x4ea66442 => 'smax.4s v2, v2, v6'
// w4: $0x4ea76463 => 'smax.4s v3, v3, v7'
// w5: $0x4ea16404 => 'smax.4s v4, v0, v1'
// w6: $0x4ea36445 => 'smax.4s v5, v2, v3'
// w7: $0x4ea56486 => 'smax.4s v6, v4, v5'
// w8: $0x4eb0a8c7 => 'smaxv s7, v6.4s'
TEXT ·maxI32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ea46400, $0x4ea56421, $0x4ea66442, $0x4ea76463, $0x4ea16404, $0x4ea36445, $0x4ea56486, $0x4eb0a8c7, $4, $16, S4, S)

// func minI32(src, dst []int32)
// w1: $0x4ea46c00 => 'smin.4s v0, v0, v4'
// w2: $0x4ea56c21 => 'smin.4s v1, v1, v5'
// w3: $0x4ea66c42 => 'smin.4s v2, v2, v6'
// w4: $0x4ea76c63 => 'smin.4s v3, v3, v7'
// w5: $0x4ea16c04 => 'smin.4s v4, v0, v1'
// w6: $0x4ea36c45 => 'smin.4s v5, v2, v3'
// w7: $0x4ea56c86 => 'smin.4s v6, v4, v5'
// w8: $0x4eb1a8c7 => 'sminv s7, v6.4s'
TEXT ·minI32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ea46c00, $0x4ea56c21, $0x4ea66c42, $0x4ea76c63, $0x4ea16c04, $0x4ea36c45, $0x4ea56c86, $0x4eb1a8c7, $4, $16, S4, S)

// func maxF32(src, dst []float32)
// w1: $0x4e24c400 => 'fmaxnm.4s v0, v0, v4'
// w2: $0x4e25c421 => 'fmaxnm.4s v1, v1, v5'
// w3: $0x4e26c442 => 'fmaxnm.4s v2, v2, v6'
// w4: $0x4e27c463 => 'fmaxnm.4s v3, v3, v7'
// w5: $0x4e21c404 => 'fmaxnm.4s v4, v0, v1'
// w6: $0x4e23c445 => 'fmaxnm.4s v5, v2, v3'
// w7: $0x4e25c486 => 'fmaxnm.4s v6, v4, v5'
// w8: $0x6e30f8c7 => 'fmaxv s7, v6.4s'
TEXT ·maxF32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4e24c400, $0x4e25c421, $0x4e26c442, $0x4e27c463, $0x4e21c404, $0x4e23c445, $0x4e25c486, $0x6e30f8c7, $4, $16, S4, S)

// func minF32(src, dst []float32)
// w1: $0x4ea4c400 => 'fminnm.4s v0, v0, v4'
// w2: $0x4ea5c421 => 'fminnm.4s v1, v1, v5'
// w3: $0x4ea6c442 => 'fminnm.4s v2, v2, v6'
// w4: $0x4ea7c463 => 'fminnm.4s v3, v3, v7'
// w5: $0x4ea1c404 => 'fminnm.4s v4, v0, v1'
// w6: $0x4ea3c445 => 'fminnm.4s v5, v2, v3'
// w7: $0x4ea5c486 => 'fminnm.4s v6, v4, v5'
// w8: $0x6eb0f8c7 => 'fminv s7, v6.4s'
TEXT ·minF32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ea4c400, $0x4ea5c421, $0x4ea6c442, $0x4ea7c463, $0x4ea1c404, $0x4ea3c445, $0x4ea5c486, $0x6eb0f8c7, $4, $16, S4, S)

// func maxF64(src, dst []float64)
// w1: $0x4e64c400 => 'fmaxnm.2d v0, v0, v4'
// w2: $0x4e65c421 => 'fmaxnm.2d v1, v1, v5'
// w3: $0x4e66c442 => 'fmaxnm.2d v2, v2, v6'
// w4: $0x4e67c463 => 'fmaxnm.2d v3, v3, v7'
// w5: $0x4e61c404 => 'fmaxnm.2d v4, v0, v1'
// w6: $0x4e63c445 => 'fmaxnm.2d v5, v2, v3'
// w7: $0x4e65c486 => 'fmaxnm.2d v6, v4, v5'
// w8: $0x6e66f4c7 => 'fmaxp v7.2d, v6.2d, v6.2d'
TEXT ·maxF64(SB),NOSPLIT,$0-48
    boundsISFSD($0x4e64c400, $0x4e65c421, $0x4e66c442, $0x4e67c463, $0x4e61c404, $0x4e63c445, $0x4e65c486, $0x6e66f4c7, $8, $8, D2, D)

// func minF64(src, dst []float64)
// w1: $0x4ee4c400 => 'fminnm.2d v0, v0, v4'
// w2: $0x4ee5c421 => 'fminnm.2d v1, v1, v5'
// w3: $0x4ee6c442 => 'fminnm.2d v2, v2, v6'
// w4: $0x4ee7c463 => 'fminnm.2d v3, v3, v7'
// w5: $0x4ee1c404 => 'fminnm.2d v4, v0, v1'
// w6: $0x4ee3c445 => 'fminnm.2d v5, v2, v3'
// w7: $0x4ee5c486 => 'fminnm.2d v6, v4, v5'
// w8: $0x6ee6f4c7 => 'fminp v7.2d, v6.2d, v6.2d'
TEXT ·minF64(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ee4c400, $0x4ee5c421, $0x4ee6c442, $0x4ee7c463, $0x4ee1c404, $0x4ee3c445, $0x4ee5c486, $0x6ee6f4c7, $8, $8, D2, D)

#define boundsI64(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, w13, w14, w15, w16) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    VLD1R (R0), [V0.D2]                                    \
    VDUP V0.D[0], V1.D2                                    \
    VDUP V0.D[0], V2.D2                                    \
    VDUP V0.D[0], V3.D2                                    \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V4.D2, V5.D2, V6.D2, V7.D2]            \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \  
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
    VEOR V4.B16, V4.B16, V4.B16                            \
tradLoop:                                                  \
    VLD1R (R0), [V4.D2]                                    \
    WORD w1                                                \
    WORD w5                                                \
    ADD $8, R0, R0                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    WORD w9                                                \
    WORD w10                                               \
    WORD w11                                               \
    WORD w12                                               \
    WORD w13                                               \
    WORD w14                                               \
    VDUP V3.D[1], V0                                       \
    WORD w15                                               \
    WORD w16                                               \
    VST1 V3.D[0], (R1)                                     \
exitFn:                                                    \
    RET      

// func maxI64(src, dst []int64)
// w1: $0x4ee03488 =>'cmgt.2d v8, v4, v0'
// w2: $0x4ee134a9 =>'cmgt.2d v9, v5, v1'
// w3: $0x4ee234ca =>'cmgt.2d v10, v6, v2'
// w4: $0x4ee334eb =>'cmgt.2d v11, v7, v3'
// w5: $0x6ea81c80 =>'bit.16b v0, v4, v8'
// w6: $0x6ea91ca1 =>'bit.16b v1, v5, v9'
// w7: $0x6eaa1cc2 =>'bit.16b v2, v6, v10'
// w8: $0x6eab1ce3 =>'bit.16b v3, v7, v11'
// w9: $0x4ee13404 =>'cmgt.2d v4, v0, v1'
// w10: $0x4ee33445 =>'cmgt.2d v5, v2, v3'
// w11: $0x6ea41c01 =>'bit.16b v1, v0, v4'
// w12: $0x6ea51c43 =>'bit.16b v3, v2, v5'
// w13: $0x4ee33422 =>'cmgt.2d v2, v1, v3'
// w14: $0x6ea21c23 =>'bit.16b v3, v1, v2'
// w15: $0x4ee33401 =>'cmgt.2d v1, v0, v3'
// w16: $0x6ea11c03 =>'bit.16b v3, v0, v1'
TEXT ·maxI64(SB),NOSPLIT,$0-48
    boundsI64($0x4ee03488, $0x4ee134a9, $0x4ee234ca, $0x4ee334eb, $0x6ea81c80, $0x6ea91ca1, $0x6eaa1cc2, $0x6eab1ce3, $0x4ee13404, $0x4ee33445, $0x6ea41c01, $0x6ea51c43, $0x4ee33422, $0x6ea21c23, $0x4ee33401, $0x6ea11c03)

// func minI64(src, dst []int64)
// w1: $0x4ee43408 =>'cmlt.2d v8, v4, v0'
// w2: $0x4ee53429 =>'cmlt.2d v9, v5, v1'
// w3: $0x4ee6344a =>'cmlt.2d v10, v6, v2'
// w4: $0x4ee7346b =>'cmlt.2d v11, v7, v3'
// w5: $0x6ea81c80 =>'bit.16b v0, v4, v8'
// w6: $0x6ea91ca1 =>'bit.16b v1, v5, v9'
// w7: $0x6eaa1cc2 =>'bit.16b v2, v6, v10'
// w8: $0x6eab1ce3 =>'bit.16b v3, v7, v11'
// w9: $0x4ee03424 =>'cmlt.2d v4, v0, v1'
// w10: $0x4ee23465 =>'cmlt.2d v5, v2, v3'
// w11: $0x6ea41c01 =>'bit.16b v1, v0, v4'
// w12: $0x6ea51c43 =>'bit.16b v3, v2, v5'
// w13: $0x4ee13462 =>'cmlt.2d v2, v1, v3'
// w14: $0x6ea21c23 =>'bit.16b v3, v1, v2'
// w15: $0x4ee03461 =>'cmlt.2d v1, v0, v3'
// w16: $0x6ea11c03 =>'bit.16b v3, v0, v1'
TEXT ·minI64(SB),NOSPLIT,$0-48
    boundsI64($0x4ee43408, $0x4ee53429, $0x4ee6344a, $0x4ee7346b, $0x6ea81c80, $0x6ea91ca1, $0x6eaa1cc2, $0x6eab1ce3, $0x4ee03424, $0x4ee23465, $0x6ea41c01, $0x6ea51c43, $0x4ee13462, $0x6ea21c23, $0x4ee03461, $0x6ea11c03)

#define doubleBoundsISFSD(w1, w2, w3, w4, w5, w6, w7, w8, w9, w10, w11, w12, w13, w14, w15, w16, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    VLD1R (R0), [V0.spec]                                  \
    VDUP V0.spec1[0], V1.spec                              \
    VDUP V0.spec1[0], V2.spec                              \
    VDUP V0.spec1[0], V3.spec                              \
    VDUP V0.spec1[0], V4.spec                              \
    VDUP V0.spec1[0], V5.spec                              \
    VDUP V0.spec1[0], V6.spec                              \
    VDUP V0.spec1[0], V7.spec                              \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V8.spec, V9.spec, V10.spec, V11.spec]  \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \  
                                                           \
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1R (R0), [V8.spec]                                  \
    WORD w1                                                \
    WORD w5                                                \
    ADD dSize, R0, R0                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    WORD w9                                                \
    WORD w10                                               \
    WORD w11                                               \
    WORD w12                                               \
    WORD w13                                               \
    WORD w14                                               \
    WORD w15                                               \
    WORD w16                                               \
    VST1 V0.spec1[0], (R1)                                 \
    ADD dSize, R1, R1                                      \
    VST1 V1.spec1[0], (R1)                                 \
exitFn:                                                    \
    RET

// func minmaxI32(src, dst []int32)
// w1: $0x4ea86400 => 'smax.4s v0, v0, v8'
// w2: $0x4ea96421 => 'smax.4s v1, v1, v9'
// w3: $0x4eaa6442 => 'smax.4s v2, v2, v10'
// w4: $0x4eab6463 => 'smax.4s v3, v3, v11'
// w5: $0x4ea86c84 => 'smin.4s v4, v4, v8'
// w6: $0x4ea96ca5 => 'smin.4s v5, v5, v9'
// w7: $0x4eaa6cc6 => 'smin.4s v6, v6, v10'
// w8: $0x4eab6ce7 => 'smin.4s v7, v7, v11'
// w9: $0x4ea16408 => 'smax.4s v8, v0, v1'
// w10: $0x4ea26469 => 'smax.4s v9, v3, v2'
// w11: $0x4ea56c8a => 'smin.4s v10, v4, v5'
// w12: $0x4ea76ccb => 'smin.4s v11, v6, v7'
// w13: $0x4ea9650c => 'smax.4s v12, v8, v9'
// w14: $0x4eab6d4d => 'smin.4s v13, v10, v11'
// w15: $0x4eb0a981 => 'smaxv s1, v12.4s'
// w16: $0x4eb1a9a0 => 'sminv s0, v13.4s'
TEXT ·minmaxI32(SB),NOSPLIT,$0-48
    doubleBoundsISFSD($0x4ea86400, $0x4ea96421, $0x4eaa6442, $0x4eab6463, $0x4ea86c84, $0x4ea96ca5, $0x4eaa6cc6, $0x4eab6ce7, $0x4ea16408, $0x4ea26469, $0x4ea56c8a, $0x4ea76ccb, $0x4ea9650c, $0x4eab6d4d, $0x4eb0a981, $0x4eb1a9a0, $4, $16, S4, S)

// func minmaxF32(src, dst []float32)
// w1: $0x4e28c400 => 'fmaxnm.4s v0, v0, v8'
// w2: $0x4e29c421 => 'fmaxnm.4s v1, v1, v9'
// w3: $0x4e2ac442 => 'fmaxnm.4s v2, v2, v10'
// w4: $0x4e2bc463 => 'fmaxnm.4s v3, v3, v11'
// w5: $0x4ea8c484 => 'fminnm.4s v4, v4, v8'
// w6: $0x4ea9c4a5 => 'fminnm.4s v5, v5, v9'
// w7: $0x4eaac4c6 => 'fminnm.4s v6, v6, v10'
// w8: $0x4eabc4e7 => 'fminnm.4s v7, v7, v11'
// w9: $0x4e21c408 => 'fmaxnm.4s v8, v0, v1'
// w10: $0x4e22c469 => 'fmaxnm.4s v9, v3, v2'
// w11: $0x4ea5c48a => 'fminnm.4s v10, v4, v5'
// w12: $0x4ea7c4cb => 'fminnm.4s v11, v6, v7'
// w13: $0x4e29c50c => 'fmaxnm.4s v12, v8, v9'
// w14: $0x4eabc54d => 'fminnm.4s v13, v10, v11'
// w15: $0x6e30f981 => 'fmaxv s1, v12.4s'
// w16: $0x6eb0f9a0 => 'fminv s0, v13.4s'
TEXT ·minmaxF32(SB),NOSPLIT,$0-48
    doubleBoundsISFSD($0x4e28c400, $0x4e29c421, $0x4e2ac442, $0x4e2bc463, $0x4ea8c484, $0x4ea9c4a5, $0x4eaac4c6, $0x4eabc4e7, $0x4e21c408, $0x4e22c469, $0x4ea5c48a, $0x4ea7c4cb, $0x4e29c50c, $0x4eabc54d, $0x6e30f981, $0x6eb0f9a0, $4, $16, S4, S)

// func minmaxF64(src, dst []float64)
// w1: $0x4e68c400 => 'fmaxnm.2d v0, v0, v8'
// w2: $0x4e69c421 => 'fmaxnm.2d v1, v1, v9'
// w3: $0x4e6ac442 => 'fmaxnm.2d v2, v2, v10'
// w4: $0x4e6bc463 => 'fmaxnm.2d v3, v3, v11'
// w5: $0x4ee8c484 => 'fminnm.2d v4, v4, v8'
// w6: $0x4ee9c4a5 => 'fminnm.2d v5, v5, v9'
// w7: $0x4eeac4c6 => 'fminnm.2d v6, v6, v10'
// w8: $0x4eebc4e7 => 'fminnm.2d v7, v7, v11'
// w9: $0x4e61c408 => 'fmaxnm.2d v8, v0, v1'
// w10: $0x4e62c469 => 'fmaxnm.2d v9, v3, v2'
// w11: $0x4ee5c48a => 'fminnm.2d v10, v4, v5'
// w12: $0x4ee7c4cb => 'fminnm.2d v11, v6, v7'
// w13: $0x4e69c50c => 'fmaxnm.2d v12, v8, v9'
// w14: $0x4eebc54d => 'fminnm.2d v13, v10, v11'
// w15: $0x6e6cf581 => 'fmaxp.2d v1, v12, v12'
// w16: $0x6eedf5a0 => 'fminp.2d v0, v13, v13'
TEXT ·minmaxF64(SB),NOSPLIT,$0-48
    doubleBoundsISFSD($0x4e68c400, $0x4e69c421, $0x4e6ac442, $0x4e6bc463, $0x4ee8c484, $0x4ee9c4a5, $0x4eeac4c6, $0x4eebc4e7, $0x4e61c408, $0x4e62c469, $0x4ee5c48a, $0x4ee7c4cb, $0x4e69c50c, $0x4eebc54d, $0x6e6cf581, $0x6eedf5a0, $8, $8, D2, D)

// func minmaxI64(src, dst []int64)
TEXT ·minmaxI64(SB),NOSPLIT,$0-48
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $8, R2, R4

    CMP $0, R2
    BEQ exitFn

    VLD1R (R0), [V0.D2]
    VDUP V0.D[0], V1.D2
    VDUP V0.D[0], V2.D2
    VDUP V0.D[0], V3.D2
    VDUP V0.D[0], V4.D2
    VDUP V0.D[0], V5.D2
    VDUP V0.D[0], V6.D2
    VDUP V0.D[0], V7.D2

    CMP $8, R2
    BLT tradLoop

vecLoop:
    VLD1.P 64(R0), [V8.D2, V9.D2, V10.D2, V11.D2]
    WORD $0x4ee0350c                                       // 'cmgt.2d v12, v8, v0'
    WORD $0x4ee1352d                                       // 'cmgt.2d v13, v9, v1'
    WORD $0x4ee2354e                                       // 'cmgt.2d v14, v10, v2'
    WORD $0x4ee3356f                                       // 'cmgt.2d v15, v11, v3'
    WORD $0x4ee83490                                       // 'cmlt.2d v16, v8, v4'
    WORD $0x4ee934b1                                       // 'cmlt.2d v17, v9, v5'
    WORD $0x4eea34d2                                       // 'cmlt.2d v18, v10, v6'
    WORD $0x4eeb34f3                                       // 'cmlt.2d v19, v11, v7'
    WORD $0x6eac1d00                                       // 'bit.16b v0, v8, v12'
    WORD $0x6ead1d21                                       // 'bit.16b v1, v9, v13'
    WORD $0x6eae1d42                                       // 'bit.16b v2, v10, v14'
    WORD $0x6eaf1d63                                       // 'bit.16b v3, v11, v15'
    WORD $0x6eb01d04                                       // 'bit.16b v4, v8, v16'
    WORD $0x6eb11d25                                       // 'bit.16b v5, v9, v17'
    WORD $0x6eb21d46                                       // 'bit.16b v6, v10, v18'
    WORD $0x6eb31d67                                       // 'bit.16b v7, v11, v19'

    ADD $8, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1R (R0), [V8.D2]
    WORD $0x4ee0350c                                       // 'cmgt.2d v12, v8, v0'
    WORD $0x4ee83490                                       // 'cmlt.2d v16, v8, v4'
    WORD $0x6eac1d00                                       // 'bit.16b v0, v8, v12'
    WORD $0x6eb01d04                                       // 'bit.16b v4, v8, v16'
    ADD $8, R0, R0
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

    WORD $0x4ee13408                                       // 'cmgt.2d v8, v0, v1'
    WORD $0x4ee33449                                       // 'cmgt.2d v9, v2, v3'
    WORD $0x4ee434aa                                       // 'cmlt.2d v10, v4, v5'
    WORD $0x4ee634eb                                       // 'cmlt.2d v11, v6, v7'
    WORD $0x6ea81c01                                       // 'bit.16b v1, v0, v8'
    WORD $0x6ea91c43                                       // 'bit.16b v3, v2, v9'
    WORD $0x6eaa1c85                                       // 'bit.16b v5, v4, v10'
    WORD $0x6eab1cc7                                       // 'bit.16b v7, v6, v11'
    WORD $0x4ee33428                                       // 'cmgt.2d v8, v1, v3'
    WORD $0x4ee534e9                                       // 'cmlt.2d v9, v5, v7'
    WORD $0x6ea81c23                                       // 'bit.16b v3, v1, v8'
    WORD $0x6ea91ca7                                       // 'bit.16b v7, v5, v9'
    VDUP V3.D[1], V0.D2
    VDUP V7.D[1], V1.D2
    WORD $0x4ee03468                                       // 'cmgt.2d v8, v3, v0'
    WORD $0x4ee73429                                       // 'cmlt.2d v9, v7, v1'
    WORD $0x6ea81c60                                       // 'bit.16b v0, v3, v8'
    WORD $0x6ea91ce1                                       // 'bit.16b v1, v7, v9'
    VST1 V1.D[0], (R1)
    ADD $8, R1, R1
    VST1 V0.D[0], (R1)
exitFn:
    RET

#define boundsWithValidityIF32(w1, w2, w3, w4, initVal)    \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD validityAddr+48(FP), R1                           \
    MOVD dstAddr+24(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB $8, R3, R5                                         \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    VEOR V0.B16, V0.B16, V0.B16                            \
    VMOVQ $0x0000000200000001, $0x0000000800000004, V1     \
    VMOVQ $0x0000002000000010, $0x0000008000000040, V2     \                    
    VMOVQ initVal, initVal, V3                             \
    VDUP V3.S[0], V4.S4                                    \
                                                           \
    EOR R7, R7                                             \
    EOR R9, R9                                             \
                                                           \                                                               
    CMP $8, R3                                             \
    BLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V5.S4, V6.S4]                          \
    VLD1R.P 1(R1), [V7.B16]                                \
    VAND V1.B16, V7.B16, V8.B16                            \
    VAND V2.B16, V7.B16, V9.B16                            \
    VCNT V8.B16, V10.B16                                   \
    VCNT V9.B16, V11.B16                                   \
    VSUB V10.S4, V0.S4, V8.S4                              \
    VSUB V11.S4, V0.S4, V9.S4                              \
    WORD w1                                                \
    WORD w2                                                \
    VBIT V8.B16, V10.B16, V3.B16                           \
    VBIT V9.B16, V11.B16, V4.B16                           \ 
                                                           \
    ADD $8, R4, R4                                         \
    CMP R5, R4                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \                                                        
    MOVB (R1), R7                                          \
tradLoop:                                                  \
    VLD1R.P 4(R0), [V5.S4]                                 \
    AND $1, R7, R8                                         \
    SUBW R8, R9, R8                                        \
    VDUP R8, V8.S4                                         \      
    WORD w1                                                \
    VBIT V8.B16, V10.B16, V3.B16                           \
    LSR $1, R7                                             \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
    WORD w3                                                \
    WORD w4                                                \
    VST1 V6.S[0], (R2)                                     \
exitFn:                                                    \
    RET      

#define I32MaxAsI64 $0x7fffffff7fffffff
#define I32MinAsI64 $0x8000000080000000
#define F32MaxAsF64 $0x7f8000007f800000
#define F32MinAsF64 $0xff800000ff800000

// w1: $0x4ea364aa => 'smax.4s v10, v5, v3'
// w2: $0x4ea464cb => 'smax.4s v11, v6, v4'
// w3: $0x4ea46465 => 'smax.4s v5, v3, v4'
// w4: $0x4eb0a8a6 => 'smaxv s6, v5.4s'
// func maxI32WithValidity(src, dst []int32, validity []byte)
TEXT ·maxI32WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF32($0x4ea364aa, $0x4ea464cb, $0x4ea46465, $0x4eb0a8a6, I32MinAsI64)    

// w1: $0x4ea36caa => 'smin.4s v10, v5, v3'
// w2: $0x4ea46ccb => 'smin.4s v11, v6, v4'
// w3: $0x4ea46c65 => 'smin.4s v5, v3, v4'
// w4: $0x4eb1a8a6 => 'sminv s6, v5.4s'
// func minI32WithValidity(src, dst []int32, validity []byte)
TEXT ·minI32WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF32($0x4ea36caa, $0x4ea46ccb, $0x4ea46c65, $0x4eb1a8a6, I32MaxAsI64)   

// w1: $0x4e23c4aa => 'fmaxnm.4s v10, v5, v3'
// w2: $0x4e24c4cb => 'fmaxnm.4s v11, v6, v4'
// w3: $0x4e24c465 => 'fmaxnm.4s v5, v3, v4'
// w4: $0x6e30f8a6 => 'fmaxv s6, v5.4s'
// func maxF32WithValidity(src, dst []float32, validity []byte)
TEXT ·maxF32WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF32($0x4e23c4aa, $0x4e24c4cb, $0x4e24c465, $0x6e30f8a6, F32MinAsF64)   

// w1: $0x4ea3c4aa => 'fminnm.4s v10, v5, v3'
// w2: $0x4ea4c4cb => 'fminnm.4s v11, v6, v4'
// w3: $0x4ea4c465 => 'fminnm.4s v5, v3, v4'
// w4: $0x6eb0f8a6 => 'fminv s6, v5.4s'
// func minF32WithValidity(src, dst []float32, validity []byte)
TEXT ·minF32WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF32($0x4ea3c4aa, $0x4ea4c4cb, $0x4ea4c465, $0x6eb0f8a6, F32MaxAsF64)   

#define boundsWithValidityIF64(wBoundsOpVecLoop, wBoundsOpTradLoop, wReduceOp, initVal) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD validityAddr+48(FP), R1                           \
    MOVD dstAddr+24(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB $8, R3, R5                                         \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    VEOR V0.B16, V0.B16, V0.B16                            \
    VMOVQ $0x0000000000000001, $0x0000000000000002, V1     \
    VMOVQ $0x0000000000000004, $0x0000000000000008, V2     \
    VMOVQ $0x0000000000000010, $0x0000000000000020, V3     \
    VMOVQ $0x0000000000000040, $0x0000000000000080, V4     \                    
    VMOVQ initVal, initVal, V5                             \
    VDUP V5.D[0], V6.D2                                    \
    VDUP V5.D[0], V7.D2                                    \
    VDUP V5.D[0], V8.D2                                    \
                                                           \
    EOR R7, R7                                             \
    EOR R9, R9                                             \
                                                           \                                                               
    CMP $8, R3                                             \
    BLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V9.D2, V10.D2, V11.D2, V12.D2]         \
    VLD1R.P 1(R1), [V13.B16]                               \
    VAND V1.B16, V13.B16, V14.B16                          \
    VAND V2.B16, V13.B16, V15.B16                          \
    VAND V3.B16, V13.B16, V16.B16                          \
    VAND V4.B16, V13.B16, V17.B16                          \
    VCNT V14.B16, V18.B16                                  \
    VCNT V15.B16, V19.B16                                  \
    VCNT V16.B16, V20.B16                                  \
    VCNT V17.B16, V21.B16                                  \
    VSUB V18.D2, V0.D2, V14.D2                             \
    VSUB V19.D2, V0.D2, V15.D2                             \
    VSUB V20.D2, V0.D2, V16.D2                             \
    VSUB V21.D2, V0.D2, V17.D2                             \
    wBoundsOpVecLoop                                       \
    VBIT V14.B16, V9.B16, V5.B16                           \
    VBIT V15.B16, V10.B16, V6.B16                          \
    VBIT V16.B16, V11.B16, V7.B16                          \
    VBIT V17.B16, V12.B16, V8.B16                          \ 
                                                           \
    ADD $8, R4, R4                                         \
    CMP R5, R4                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \                                                        
    MOVB (R1), R7                                          \
tradLoop:                                                  \
    VLD1R.P 8(R0), [V9.D2]                                 \
    AND $1, R7, R8                                         \
    SUB R8, R9, R8                                         \
    VDUP R8, V14.D2                                        \      
    wBoundsOpTradLoop                                      \
    VBIT V14.B16, V9.B16, V5.B16                           \
    LSR $1, R7                                             \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
    wReduceOp                                              \
    VST1 V3.D[0], (R2)                                     \
exitFn:                                                    \
    RET      

#define I64MaxAsI64 $0x7fffffffffffffff
#define I64MinAsI64 $0x8000000000000000
#define F64MaxAsF64 $0x7ff0000000000000
#define F64MinAsF64 $0xfff0000000000000

#define I64MaxWValidityVecLoopOps \
        WORD $0x4ee934b2          \ // 'cmlt.2d v18, v9, v5'
        WORD $0x4eea34d3          \ // 'cmlt.2d v19, v10, v6'
        WORD $0x4eeb34f4          \ // 'cmlt.2d v20, v11, v7'
        WORD $0x4eec3515          \ // 'cmlt.2d v21, v12, v8'
        WORD $0x6eb21ca9          \ // 'bit.16b v9, v5, v18'
        WORD $0x6eb31cca          \ // 'bit.16b v10, v6, v19'
        WORD $0x6eb41ceb          \ // 'bit.16b v11, v7, v20'
        WORD $0x6eb51d0c            // 'bit.16b v12, v8, v21'

#define I64MaxWValidityTradLoopOps \
        WORD $0x4ee934b2           \ // 'cmlt.2d v18, v9, v5'
        WORD $0x6eb21ca9             // 'bit.16b v9, v5, v18'

#define I64MaxWValidityReduceOps \
        WORD $0x4ee534c0         \ // 'cmlt.2d v0, v5, v6'
        WORD $0x4ee73501         \ // 'cmlt.2d v1, v7, v8'
        WORD $0x6ea01cc5         \ // 'bit.16b v5, v6, v0'
        WORD $0x6ea11d07         \ // 'bit.16b v7, v8, v1'
        WORD $0x4ee534e0         \ // 'cmlt.2d v0, v5, v7'
        WORD $0x6ea01ce5         \ // 'bit.16b v5, v7, v0'
        VDUP V5.D[1], V3.D2      \
        WORD $0x4ee334a0         \ // 'cmlt.2d v0, v3, v5'
        WORD $0x6ea01ca3           // 'bit.16b v3, v5, v0'

#define I64MinWValidityVecLoopOps \
        WORD $0x4ee53532          \ // 'cmlt.2d v18, v5, v9'
        WORD $0x4ee63553          \ // 'cmlt.2d v19, v6, v10'
        WORD $0x4ee73574          \ // 'cmlt.2d v20, v7, v11'
        WORD $0x4ee83595          \ // 'cmlt.2d v21, v8, v12'
        WORD $0x6eb21ca9          \ // 'bit.16b v9, v5, v18'
        WORD $0x6eb31cca          \ // 'bit.16b v10, v6, v19'
        WORD $0x6eb41ceb          \ // 'bit.16b v11, v7, v20'
        WORD $0x6eb51d0c            // 'bit.16b v12, v8, v21'

#define I64MinWValidityTradLoopOps \
        WORD $0x4ee53532           \ // 'cmlt.2d v18, v5, v9'
        WORD $0x6eb21ca9             // 'bit.16b v9, v5, v18'

#define I64MinWValidityReduceOps \
        WORD $0x4ee634a0         \ // 'cmlt.2d v0, v6, v5'
        WORD $0x4ee834e1         \ // 'cmlt.2d v1, v8, v7'
        WORD $0x6ea01cc5         \ // 'bit.16b v5, v6, v0'
        WORD $0x6ea11d07         \ // 'bit.16b v7, v8, v1'
        WORD $0x4ee734a0         \ // 'cmlt.2d v0, v7, v5'
        WORD $0x6ea01ce5         \ // 'bit.16b v5, v7, v0'
        VDUP V5.D[1], V3.D2      \
        WORD $0x4ee53460         \ // 'cmlt.2d v0, v5, v3'
        WORD $0x6ea01ca3           // 'bit.16b v3, v5, v0'

#define F64MaxWValidityVecLoopOps \
        WORD $0x4e65c529          \ // 'fmaxnm.2d v9, v9, v5'
        WORD $0x4e66c54a          \ // 'fmaxnm.2d v10, v10, v6'
        WORD $0x4e67c56b          \ // 'fmaxnm.2d v11, v11, v7'
        WORD $0x4e68c58c            // 'fmaxnm.2d v12, v12, v8'

#define F64MaxWValidityTradLoopOps \
        WORD $0x4e65c529             // 'fmaxnm.2d v9, v9, v5'

#define F64MaxWValidityReduceOps \
        WORD $0x4e66c4a0         \ // 'fmaxnm.2d v0, v5, v6'
        WORD $0x4e68c4e1         \ // 'fmaxnm.2d v1, v7, v8'
        WORD $0x4e61c402         \ // 'fmaxnm.2d v2, v0, v1'
        WORD $0x6e62f443           // 'fmaxp v3.2d, v2.2d, v2.2d'

#define F64MinWValidityVecLoopOps \
        WORD $0x4ee5c529          \ // 'fminnm.2d v9, v9, v5'
        WORD $0x4ee6c54a          \ // 'fminnm.2d v10, v10, v6'
        WORD $0x4ee7c56b          \ // 'fminnm.2d v11, v11, v7'
        WORD $0x4ee8c58c            // 'fminnm.2d v12, v12, v8'

#define F64MinWValidityTradLoopOps \
        WORD $0x4ee5c529             // 'fminnm.2d v9, v9, v5'

#define F64MinWValidityReduceOps \
        WORD $0x4ee6c4a0         \ // 'fminnm.2d v0, v5, v6'
        WORD $0x4ee8c4e1         \ // 'fminnm.2d v1, v7, v8'
        WORD $0x4ee1c402         \ // 'fminnm.2d v2, v0, v1'
        WORD $0x6ee2f443           // 'fminp v3.2d, v2.2d, v2.2d'

// func maxI64WithValidity(src, dst []int64, validity []byte)
TEXT ·maxI64WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF64(I64MaxWValidityVecLoopOps, I64MaxWValidityTradLoopOps, I64MaxWValidityReduceOps, I64MinAsI64)

// func minI64WithValidity(src, dst []int64, validity []byte)
TEXT ·minI64WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF64(I64MinWValidityVecLoopOps, I64MinWValidityTradLoopOps, I64MinWValidityReduceOps, I64MaxAsI64)

// func maxF64WithValidity(src, dst []float64, validity []byte)
TEXT ·maxF64WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF64(F64MaxWValidityVecLoopOps, F64MaxWValidityTradLoopOps, F64MaxWValidityReduceOps, F64MinAsF64)

// func minF64WithValidity(src, dst []float64, validity []byte)
TEXT ·minF64WithValidity(SB),NOSPLIT,$0-72
    boundsWithValidityIF64(F64MinWValidityVecLoopOps, F64MinWValidityTradLoopOps, F64MinWValidityReduceOps, F64MaxAsF64)

#define doubleBoundsWithValidityIF32(w1, w2, w3, w4, w5, w6, w7, w8, initVal1, initVal2) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD validityAddr+48(FP), R1                           \
    MOVD dstAddr+24(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB $8, R3, R5                                         \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    VEOR V0.B16, V0.B16, V0.B16                            \
    VMOVQ $0x0000000200000001, $0x0000000800000004, V1     \
    VMOVQ $0x0000002000000010, $0x0000008000000040, V2     \                    
    VMOVQ initVal1, initVal1, V3                           \
    VDUP V3.S[0], V4.S4                                    \
    VMOVQ initVal2, initVal2, V5                           \
    VDUP V5.S[0], V6.S4                                    \
                                                           \
    EOR R7, R7                                             \
    EOR R9, R9                                             \
                                                           \                                                               
    CMP $8, R3                                             \
    BLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V7.S4, V8.S4]                          \
    VLD1R.P 1(R1), [V9.B16]                                \
    VAND V1.B16, V9.B16, V10.B16                           \
    VAND V2.B16, V9.B16, V11.B16                           \
    VCNT V10.B16, V12.B16                                  \
    VCNT V11.B16, V13.B16                                  \
    VSUB V12.S4, V0.S4, V10.S4                             \
    VSUB V13.S4, V0.S4, V11.S4                             \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VBIT V10.B16, V12.B16, V3.B16                          \
    VBIT V11.B16, V13.B16, V4.B16                          \
    VBIT V10.B16, V14.B16, V5.B16                          \ 
    VBIT V11.B16, V15.B16, V6.B16                          \ 
                                                           \
    ADD $8, R4, R4                                         \
    CMP R5, R4                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \                                                        
    MOVB (R1), R7                                          \
tradLoop:                                                  \
    VLD1R.P 4(R0), [V7.S4]                                 \
    AND $1, R7, R8                                         \
    SUBW R8, R9, R8                                        \
    VDUP R8, V10.S4                                        \      
    WORD w1                                                \
    WORD w3                                                \
    VBIT V10.B16, V12.B16, V3.B16                          \
    VBIT V10.B16, V14.B16, V5.B16                          \
    LSR $1, R7                                             \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P V0.S[0], 4(R2)                                  \
    VST1 V1.S[0], (R2)                                     \
exitFn:                                                    \
    RET      

// w1: $0x4ea36cec => 'smin.4s v12, v7, v3'
// w2: $0x4ea46d0d => 'smin.4s v13, v8, v4'
// w3: $0x4ea564ee => 'smax.4s v14, v7, v5'
// w4: $0x4ea6650f => 'smax.4s v15, v8, v6'
// w5: $0x4ea46c62 => 'smin.4s v2, v3, v4'
// w6: $0x4ea664a3 => 'smax.4s v3, v5, v6'
// w7: $0x4eb1a840 => 'sminv s0, v2.4s'
// w8: $0x4eb0a861 => 'smaxv s1, v3.4s'
// func minmaxI32WithValidity(src, dst []int32, validity []byte)
TEXT ·minmaxI32WithValidity(SB),NOSPLIT,$0-72
    doubleBoundsWithValidityIF32($0x4ea36cec, $0x4ea46d0d, $0x4ea564ee, $0x4ea6650f, $0x4ea46c62, $0x4ea664a3, $0x4eb1a840, $0x4eb0a861, I32MaxAsI64, I32MinAsI64)

// w1: $0x4ea3c4ec => 'fminnm.4s v12, v7, v3'
// w1: $0x4ea4c50d => 'fminnm.4s v13, v8, v4'
// w1: $0x4e25c4ee => 'fmaxnm.4s v14, v7, v5'
// w1: $0x4e26c50f => 'fmaxnm.4s v15, v8, v6'
// w1: $0x4ea4c462 => 'fminnm.4s v2, v3, v4'
// w1: $0x4e26c4a3 => 'fmaxnm.4s v3, v5, v6'
// w1: $0x6eb0f840 => 'fminv s0, v2.4s'
// w1: $0x6e30f861 => 'fmaxv s1, v3.4s'
// func minmaxF32WithValidity(src, dst []float32, validity []byte)
TEXT ·minmaxF32WithValidity(SB),NOSPLIT,$0-72
    doubleBoundsWithValidityIF32($0x4ea3c4ec, $0x4ea4c50d, $0x4e25c4ee, $0x4e26c50f, $0x4ea4c462, $0x4e26c4a3, $0x6eb0f840, $0x6e30f861, F32MaxAsF64, F32MinAsF64)
