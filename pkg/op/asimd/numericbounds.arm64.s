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
// w6: $0x4ea36445 => 'smax.2d v5, v2, v3'
// w7: $0x4ea56486 => 'smax.2d v6, v4, v5'
// w8: $0x4eb0a8c7 => 'smaxv s7, v6.4s'
TEXT ·maxI32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ea46400, $0x4ea56421, $0x4ea66442, $0x4ea76463, $0x4ea16404, $0x4ea36445, $0x4ea56486, $0x4eb0a8c7, $4, $16, S4, S)

// func minI32(src, dst []int32)
// w1: $0x4ea46c00 => 'smin.4s v0, v0, v4'
// w2: $0x4ea56c21 => 'smin.4s v1, v1, v5'
// w3: $0x4ea66c42 => 'smin.4s v2, v2, v6'
// w4: $0x4ea76c63 => 'smin.4s v3, v3, v7'
// w5: $0x4ea16c04 => 'smin.4s v4, v0, v1'
// w6: $0x4ea36c45 => 'smin.2d v5, v2, v3'
// w7: $0x4ea56c86 => 'smin.2d v6, v4, v5'
// w8: $0x4eb1a8c7 => 'sminv s7, v6.4s'
TEXT ·minI32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4ea46c00, $0x4ea56c21, $0x4ea66c42, $0x4ea76c63, $0x4ea16c04, $0x4ea36c45, $0x4ea56c86, $0x4eb1a8c7, $4, $16, S4, S)

// func maxF32(src, dst []float32)
// w1: $0x4e24c400 => 'fmaxnm.4s v0, v0, v4'
// w2: $0x4e25c421 => 'fmaxnm.4s v1, v1, v5'
// w3: $0x4e26c442 => 'fmaxnm.4s v2, v2, v6'
// w4: $0x4e27c463 => 'fmaxnm.4s v3, v3, v7'
// w5: $0x4e21c404 => 'fmaxnm.4s v4, v0, v1'
// w6: $0x4e23c445 => 'fmaxnm.2d v5, v2, v3'
// w7: $0x4e25c486 => 'fmaxnm.2d v6, v4, v5'
// w8: $0x6e30f8c7 => 'fmaxv s7, v6.4s'
TEXT ·maxF32(SB),NOSPLIT,$0-48
    boundsISFSD($0x4e24c400, $0x4e25c421, $0x4e26c442, $0x4e27c463, $0x4e21c404, $0x4e23c445, $0x4e25c486, $0x6e30f8c7, $4, $16, S4, S)

// func minF32(src, dst []float32)
// w1: $0x4ea4c400 => 'fminnm.4s v0, v0, v4'
// w2: $0x4ea5c421 => 'fminnm.4s v1, v1, v5'
// w3: $0x4ea6c442 => 'fminnm.4s v2, v2, v6'
// w4: $0x4ea7c463 => 'fminnm.4s v3, v3, v7'
// w5: $0x4ea1c404 => 'fminnm.4s v4, v0, v1'
// w6: $0x4ea3c445 => 'fminnm.2d v5, v2, v3'
// w7: $0x4ea5c486 => 'fminnm.2d v6, v4, v5'
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
