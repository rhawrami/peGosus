//go:build arm64

#include "textflag.h"

#define vCompareIF32Lit(w1, w2)                            \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVW lit+48(FP), R5                                    \
    VDUP R5, V0.S4                                         \
    VMOVQ $0x0000000200000001, $0x0000000800000004, V1     \
    VMOVQ $0x0000002000000010, $0x0000008000000040, V2     \
    EOR R6, R6                                             \
    MOVD $1, R7                                            \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V5.S4, V6.S4]                          \
    WORD w1                                                \
    WORD w2                                                \
                                                           \
    VAND V9.B16, V1.B16, V13.B16                           \
    VAND V10.B16, V2.B16, V14.B16                          \
                                                           \
    VORR V13.B16, V14.B16, V15.B16                         \
    VADDV V15.B16, V16                                     \
    VST1.P V16.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 4(R0), V5.S[0]                                  \
    WORD w1                                                \
    VMOV V9.B[0], R8                                       \
    AND R8, R7, R9                                         \
    ORR R9, R6, R6                                         \ 
                                                           \
    LSL $1, R7                                             \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    MOVBU R6, (R1)                                         \
exitFn:                                                    \
    RET

// func cmpGtI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x4ea034a9 => 'cmgt.4s v9, v5, v0'
// w2: $0x4ea034ca => 'cmgt.4s v10, v6, v0'
TEXT ·cmpGtI32VecI32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x4ea034a9, $0x4ea034ca)

// func cmpLtI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x4ea53409 => 'cmlt.4s v9, v5, v0'
// w2: $0x4ea6340a => 'cmlt.4s v10, v6, v0'
TEXT ·cmpLtI32VecI32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x4ea53409, $0x4ea6340a)

// func cmpGeI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x4ea03ca9 => 'cmge.4s v9, v5, v0'
// w2: $0x4ea03cca => 'cmge.4s v10, v6, v0'
TEXT ·cmpGeI32VecI32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x4ea03ca9, $0x4ea03cca)

// func cmpLeI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x4ea53c09 => 'cmle.4s v9, v5, v0'
// w2: $0x4ea63c0a => 'cmle.4s v10, v6, v0'
TEXT ·cmpLeI32VecI32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x4ea53c09, $0x4ea63c0a)

// func cmpEqI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x6ea08ca9 => 'cmeq.4s v9, v5, v0'
// w2: $0x6ea08cca => 'cmeq.4s v10, v6, v0'
TEXT ·cmpEqI32VecI32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x6ea08ca9, $0x6ea08cca)

// func cmpGtF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x6ea0e4a9 => 'fcmgt.4s v9, v5, v0'
// w2: $0x6ea0e4ca => 'fcmgt.4s v10, v6, v0'
TEXT ·cmpGtF32VecF32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x6ea0e4a9, $0x6ea0e4ca)

// func cmpLtF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x6ea5e409 => 'fcmlt.4s v9, v5, v0'
// w2: $0x6ea6e40a => 'fcmlt.4s v10, v6, v0'
TEXT ·cmpLtF32VecF32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x6ea5e409, $0x6ea6e40a)

// func cmpGeF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x6e20e4a9 => 'fcmge.4s v9, v5, v0'
// w2: $0x6e20e4ca => 'fcmge.4s v10, v6, v0'
TEXT ·cmpGeF32VecF32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x6e20e4a9, $0x6e20e4ca)

// func cmpLeF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x6e25e409 => 'fcmle.4s v9, v5, v0'
// w2: $0x6e26e40a => 'fcmle.4s v10, v6, v0'
TEXT ·cmpLeF32VecF32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x6e25e409, $0x6e26e40a)

// func cmpEqF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x4e20e4a9 => 'fcmeq.4s v9, v5, v0'
// w2: $0x4e20e4ca => 'fcmeq.4s v10, v6, v0'
TEXT ·cmpEqF32VecF32Lit(SB),NOSPLIT,$0-52
    vCompareIF32Lit($0x4e20e4a9, $0x4e20e4ca)

#define vNotEqIF32Lit(w1, w2, w3, w4)                      \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVW lit+48(FP), R5                                    \
    VDUP R5, V0.S4                                         \
    VMOVQ $0x0000000200000001, $0x0000000800000004, V1     \
    VMOVQ $0x0000002000000010, $0x0000008000000040, V2     \
    EOR R6, R6                                             \
    MOVD $1, R7                                            \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V5.S4, V6.S4]                          \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
                                                           \
    VAND V11.B16, V1.B16, V13.B16                          \
    VAND V12.B16, V2.B16, V14.B16                          \
                                                           \
    VORR V13.B16, V14.B16, V15.B16                         \
    VADDV V15.B16, V16                                     \
    VST1.P V16.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 4(R0), V5.S[0]                                  \
    WORD w1                                                \
    WORD w3                                                \
    VMOV V11.B[0], R8                                      \
    AND R8, R7, R9                                         \
    ORR R9, R6, R6                                         \ 
                                                           \
    LSL $1, R7                                             \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    MOVBU R6, (R1)                                         \
exitFn:                                                    \
    RET

// func cmpNeqI32VecI32Lit(src []int32, dst []byte, lit int32)
// w1: $0x6ea08ca9 => 'cmeq.4s v9, v5, v0'
// w2: $0x6ea08cca => 'cmeq.4s v10, v6, v0'
// w3: $0x6e20592b => 'not.16b v11, v9'
// w4: $0x6e20594c => 'not.16b v12, v10'
TEXT ·cmpNeqI32VecI32Lit(SB),NOSPLIT,$0-52
    vNotEqIF32Lit($0x6ea08ca9, $0x6ea08cca, $0x6e20592b, $0x6e20594c)

// func cmpNeqF32VecF32Lit(src []float32, dst []byte, lit float32)
// w1: $0x4e20e4a9 => 'fcmeq.4s v9, v5, v0'
// w2: $0x4e20e4ca => 'fcmeq.4s v10, v6, v0'
// w3: $0x6e20592b => 'not.16b v11, v9'
// w4: $0x6e20594c => 'not.16b v12, v10'
TEXT ·cmpNeqF32VecF32Lit(SB),NOSPLIT,$0-52
    vNotEqIF32Lit($0x4e20e4a9, $0x4e20e4ca, $0x6e20592b, $0x6e20594c)

#define vCompareBetIF32Lit(w1, w2, w3, w4, vBOp)           \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVW min+48(FP), R5                                    \
    VDUP R5, V0.S4                                         \
    MOVD max+52(FP), R6                                    \
    VDUP R6, V1.S4                                         \
    VMOVQ $0x0000000200000001, $0x0000000800000004, V2     \
    VMOVQ $0x0000002000000010, $0x0000008000000040, V3     \
    EOR R7, R7                                             \
    MOVD $1, R8                                            \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP $8, R2                                             \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 32(R0), [V4.S4, V5.S4]                          \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
                                                           \
    vBOp V6.B16, V8.B16, V10.B16                           \
    vBOp V7.B16, V9.B16, V11.B16                           \
                                                           \
    VAND V10.B16, V2.B16, V12.B16                          \
    VAND V11.B16, V3.B16, V13.B16                          \
                                                           \
    VORR V12.B16, V13.B16, V14.B16                         \
    VADDV V14.B16, V15                                     \
    VST1.P V15.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 4(R0), V4.S[0]                                  \
    WORD w1                                                \
    WORD w3                                                \
    vBOp V6.B16, V8.B16, V10.B16                           \
    VMOV V10.B[0], R9                                      \
    AND R9, R8, R10                                        \
    ORR R10, R7, R7                                        \ 
                                                           \
    LSL $1, R8                                             \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    MOVBU R7, (R1)                                         \
exitFn:                                                    \
    RET

// func cmpBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)
// w1: $0x4ea03c86 => 'cmge.4s v6, v4, v0'
// w2: $0x4ea03ca7 => 'cmge.4s v7, v5, v0'
// w3: $0x4ea43c28 => 'cmle.4s v8, v4, v1'
// w4: $0x4ea53c29 => 'cmle.4s v9, v5, v1'
TEXT ·cmpBetI32VecI32Lit(SB),NOSPLIT,$0-56
    vCompareBetIF32Lit($0x4ea03c86, $0x4ea03ca7, $0x4ea43c28, $0x4ea53c29, VAND)

// func cmpNBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)
// w1: $0x4ea43406 => 'cmlt.4s v6, v4, v0'
// w2: $0x4ea53407 => 'cmlt.4s v7, v5, v0'
// w3: $0x4ea13488 => 'cmgt.4s v8, v4, v1'
// w4: $0x4ea134a9 => 'cmgt.4s v9, v5, v1'
TEXT ·cmpNBetI32VecI32Lit(SB),NOSPLIT,$0-56
    vCompareBetIF32Lit($0x4ea43406, $0x4ea53407, $0x4ea13488, $0x4ea134a9, VORR)

// func cmpBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)
// w1: $0x6e20e486 => 'fcmge.4s v6, v4, v0'
// w2: $0x6e20e4a7 => 'fcmge.4s v7, v5, v0'
// w3: $0x6e24e428 => 'fcmle.4s v8, v4, v1'
// w4: $0x6e25e429 => 'fcmle.4s v9, v5, v1'
TEXT ·cmpBetF32VecF32Lit(SB),NOSPLIT,$0-56
    vCompareBetIF32Lit($0x6e20e486, $0x6e20e4a7, $0x6e24e428, $0x6e25e429, VAND)

// func cmpNBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)
// w1: $0x6ea4e406 => 'fcmlt.4s v6, v4, v0'
// w2: $0x6ea5e407 => 'fcmlt.4s v7, v5, v0'
// w3: $0x6ea1e488 => 'fcmgt.4s v8, v4, v1'
// w4: $0x6ea1e4a9 => 'fcmgt.4s v9, v5, v1'
TEXT ·cmpNBetF32VecF32Lit(SB),NOSPLIT,$0-56
    vCompareBetIF32Lit($0x6ea4e406, $0x6ea5e407, $0x6ea1e488, $0x6ea1e4a9, VORR)
