//go:build arm64

#include "textflag.h"

#define vCastIFXToFIX(w1, w2, w3, w4, dSize, chnkSize, spec, spec1) \
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

// func castI64ToF64(src []int64, dst []float64)
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x4e61d846 => 'scvtf.2d v6, v2'
// w3: $0x4e61d867 => 'scvtf.2d v7, v3'
// w4: $0x4e61d888 => 'scvtf.2d v8, v4'
TEXT ·castI64ToF64(SB),NOSPLIT,$0-48
    vCastIFXToFIX($0x4e61d825, $0x4e61d846, $0x4e61d867, $0x4e61d888, $8, $8, D2, D)

// func castI32ToF32(src []int32, dst []float32)
// w1: $0x4e21d825 => 'scvtf.4s v5, v1'
// w2: $0x4e21d846 => 'scvtf.4s v6, v2'
// w3: $0x4e21d867 => 'scvtf.4s v7, v3'
// w4: $0x4e21d888 => 'scvtf.4s v8, v4'
TEXT ·castI32ToF32(SB),NOSPLIT,$0-48
    vCastIFXToFIX($0x4e21d825, $0x4e21d846, $0x4e21d867, $0x4e21d888, $4, $16, S4, S)

// func castF64ToI64(src []float64, dst []int64)
// w1: $0x4ee1b825 => 'fcvtzs.2d v5, v1'
// w2: $0x4ee1b846 => 'fcvtzs.2d v6, v2'
// w3: $0x4ee1b867 => 'fcvtzs.2d v7, v3'
// w4: $0x4ee1b888 => 'fcvtzs.2d v8, v4'
TEXT ·castF64ToI64(SB),NOSPLIT,$0-48
    vCastIFXToFIX($0x4ee1b825, $0x4ee1b846, $0x4ee1b867, $0x4ee1b888, $8, $8, D2, D)

// func castF32ToI32(src []float32, dst []int32)
// w1: $0x4ea1b825 => 'fcvtzs.4s v5, v1'
// w2: $0x4ea1b846 => 'fcvtzs.4s v6, v2'
// w3: $0x4ea1b867 => 'fcvtzs.4s v7, v3'
// w4: $0x4ea1b888 => 'fcvtzs.4s v8, v4'
TEXT ·castF32ToI32(SB),NOSPLIT,$0-48
    vCastIFXToFIX($0x4ea1b825, $0x4ea1b846, $0x4ea1b867, $0x4ea1b888, $4, $16, S4, S)

#define vCastIFSToIFD(w1, w2, w3, w4)                      \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V1.S4, V2.S4]                          \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V3.D2, V4.D2, V5.D2, V6.D2], 64(R1)            \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.S[0]                                     \
    WORD w1                                                \
    VST1 V3.D[0], (R1)                                     \
    ADD $4, R0, R0                                         \ 
    ADD $8, R1, R1                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func castF32ToF64(src []float32, dst []float64)
// w1: $0x0e617823 =>  'fcvtl v3.2d, v1.2s'
// w2: $0x4e617824 =>  'fcvtl2 v4.2d, v1.4s'
// w3: $0x0e617845 =>  'fcvtl v5.2d, v2.2s'
// w4: $0x4e617846 =>  'fcvtl2 v6.2d, v2.4s'
TEXT ·castF32ToF64(SB),NOSPLIT,$0-48
    vCastIFSToIFD($0x0e617823, $0x4e617824, $0x0e617845, $0x4e617846)

// func castF32ToF64(src []float32, dst []float64)
// w1: $0x0f20a423 =>  'sxtl v3.2d, v1.2s'
// w2: $0x4f20a424 =>  'sxtl2 v4.2d, v1.4s'
// w3: $0x0f20a445 =>  'sxtl v5.2d, v2.2s'
// w4: $0x4f20a446 =>  'sxtl2 v6.2d, v2.4s'
TEXT ·castI32ToI64(SB),NOSPLIT,$0-48
    vCastIFSToIFD($0x0f20a423, $0x4f20a424, $0x0f20a445, $0x4f20a446)

#define vCastIFSToFID(w1, w2, w3, w4, w5, w6, w7, w8)      \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V1.S4, V2.S4]                          \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P [V7.D2, V8.D2, V9.D2, V10.D2], 64(R1)            \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.S[0]                                     \
    WORD w1                                                \
    WORD w5                                                \
    VST1 V7.D[0], (R1)                                     \
    ADD $4, R0, R0                                         \ 
    ADD $8, R1, R1                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func castI32ToF64(src []int32, dst []float64)
// w1: $0x0f20a423 => 'sxtl v3.2d, v1.2s'
// w2: $0x4f20a424 => 'sxtl2 v4.2d, v1.4s'
// w3: $0x0f20a445 => 'sxtl v5.2d, v2.2s'
// w4: $0x4f20a446 => 'sxtl2 v6.2d, v2.4s'
// w5: $0x4e61d867 => 'scvtf.2d v7, v3'
// w6: $0x4e61d888 => 'scvtf.2d v8, v4'
// w7: $0x4e61d8a9 => 'scvtf.2d v9, v5'
// w8: $0x4e61d8ca => 'scvtf.2d v10, v6'
TEXT ·castI32ToF64(SB),NOSPLIT,$0-48
    vCastIFSToFID($0x0f20a423, $0x4f20a424, $0x0f20a445, $0x4f20a446, $0x4e61d867, $0x4e61d888, $0x4e61d8a9, $0x4e61d8ca)

// func castF32ToI64(src []float32, dst []int64)
// w1: $0x0e617823 => 'fcvtl v3.2d, v1.2s'
// w2: $0x4e617824 => 'fcvtl2 v4.2d, v1.4s'
// w3: $0x0e617845 => 'fcvtl v5.2d, v2.2s'
// w4: $0x4e617846 => 'fcvtl2 v6.2d, v2.4s'
// w5: $0x4ee1b867 => 'fcvtzs.2d v7, v3'
// w6: $0x4ee1b888 => 'fcvtzs.2d v8, v4'
// w7: $0x4ee1b8a9 => 'fcvtzs.2d v9, v5'
// w8: $0x4ee1b8ca => 'fcvtzs.2d v10, v6'
TEXT ·castF32ToI64(SB),NOSPLIT,$0-48
    vCastIFSToFID($0x0e617823, $0x4e617824, $0x0e617845, $0x4e617846, $0x4ee1b867, $0x4ee1b888, $0x4ee1b8a9, $0x4ee1b8ca)

#define vCastIFDToFIS(w1, w2, w3, w4, w5, w6, w7, w8)      \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.D2, V2.D2, V3.D2, V4.D2]            \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
    VST1.P [V9.S4, V10.S4], 32(R1)                         \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.D[0]                                     \
    WORD w1                                                \
    WORD w5                                                \
    VST1 V9.S[0], (R1)                                     \
    ADD $8, R0, R0                                         \ 
    ADD $4, R1, R1                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func castI64ToF32(src []int64, dst []float32)
// w1: $0x4e61d825 => 'scvtf.2d v5, v1'
// w2: $0x4e61d846 => 'scvtf.2d v6, v2'
// w3: $0x4e61d867 => 'scvtf.2d v7, v3'
// w4: $0x4e61d888 => 'scvtf.2d v8, v4'
// w5: $0x0e6168a9 => 'fcvtn v9.2s, v5.2d'
// w6: $0x4e6168c9 => 'fcvtn2 v9.4s, v6.2d'
// w7: $0x0e6168ea => 'fcvtn v10.2s, v7.2d'
// w8: $0x4e61690a => 'fcvtn2 v10.4s, v8.2d'
TEXT ·castI64ToF32(SB),NOSPLIT,$0-48
    vCastIFDToFIS($0x4e61d825, $0x4e61d846, $0x4e61d867, $0x4e61d888, $0x0e6168a9, $0x4e6168c9, $0x0e6168ea, $0x4e61690a)

// func castF64ToI32(src []float64, dst []int32)
// w1: $0x4ee1b825 => 'fcvtzs.2d v5, v1'
// w2: $0x4ee1b846 => 'fcvtzs.2d v6, v2'
// w3: $0x4ee1b867 => 'fcvtzs.2d v7, v3'
// w4: $0x4ee1b888 => 'fcvtzs.2d v8, v4'
// w5: $0x0ea148a9 => 'sqxtn v9.2s, v5.2d'
// w6: $0x4ea148c9 => 'sqxtn2 v9.4s, v6.2d'
// w7: $0x0ea148ea => 'sqxtn v10.2s, v7.2d'
// w8: $0x4ea1490a => 'sqxtn2 v10.4s, v8.2d'
TEXT ·castF64ToI32(SB),NOSPLIT,$0-48
    vCastIFDToFIS($0x4ee1b825, $0x4ee1b846, $0x4ee1b867, $0x4ee1b888, $0x0ea148a9, $0x4ea148c9, $0x0ea148ea, $0x4ea1490a)
