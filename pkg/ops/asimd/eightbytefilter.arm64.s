//go:build arm64

#include "textflag.h"

#define vCompareIF64Lit(w1, w2, w3, w4)                    \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVD lit+48(FP), R5                                    \
    VDUP R5, V0.D2                                         \
    VMOVQ $0x0000000000000001, $0x0000000000000002, V1     \
    VMOVQ $0x0000000000000004, $0x0000000000000008, V2     \
    VMOVQ $0x0000000000000010, $0x0000000000000020, V3     \
    VMOVQ $0x0000000000000040, $0x0000000000000080, V4     \
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
    VLD1.P 64(R0), [V5.D2, V6.D2, V7.D2, V8.D2]            \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
                                                           \
    VAND V9.B16, V1.B16, V13.B16                           \
    VAND V10.B16, V2.B16, V14.B16                          \
    VAND V11.B16, V3.B16, V15.B16                          \
    VAND V12.B16, V4.B16, V16.B16                          \
                                                           \
    VORR V13.B16, V14.B16, V17.B16                         \
    VORR V15.B16, V16.B16, V18.B16                         \
    VORR V17.B16, V18.B16, V19.B16                         \
    VADDV V19.B16, V20                                     \
    VST1.P V20.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 8(R0), V5.D[0]                                  \
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

// func cmpGtI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x4ee034a9 => 'cmgt.2d v9, v5, v0'
// w2: $0x4ee034ca => 'cmgt.2d v10, v6, v0'
// w3: $0x4ee034eb => 'cmgt.2d v11, v7, v0'
// w4: $0x4ee0350c => 'cmgt.2d v12, v8, v0'
TEXT ·cmpGtI64VecI64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x4ee034a9, $0x4ee034ca, $0x4ee034eb, $0x4ee0350c)

// func cmpLtI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x4ee53409 => 'cmlt.2d v9, v5, v0'
// w2: $0x4ee6340a => 'cmlt.2d v10, v6, v0'
// w3: $0x4ee7340b => 'cmlt.2d v11, v7, v0'
// w4: $0x4ee8340c => 'cmlt.2d v12, v8, v0'
TEXT ·cmpLtI64VecI64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x4ee53409, $0x4ee6340a, $0x4ee7340b, $0x4ee8340c)

// func cmpGeI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x4ee03ca9 => 'cmge.2d v9, v5, v0'
// w2: $0x4ee03cca => 'cmge.2d v10, v6, v0'
// w3: $0x4ee03ceb => 'cmge.2d v11, v7, v0'
// w4: $0x4ee03d0c => 'cmge.2d v12, v8, v0'
TEXT ·cmpGeI64VecI64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x4ee03ca9, $0x4ee03cca, $0x4ee03ceb, $0x4ee03d0c)

// func cmpLeI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x4ee53c09 => 'cmle.2d v9, v5, v0'
// w2: $0x4ee63c0a => 'cmle.2d v10, v6, v0'
// w3: $0x4ee73c0b => 'cmle.2d v11, v7, v0'
// w4: $0x4ee83c0c => 'cmle.2d v12, v8, v0'
TEXT ·cmpLeI64VecI64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x4ee53c09, $0x4ee63c0a, $0x4ee73c0b, $0x4ee83c0c)

// func cmpEqI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x6ee08ca9 => 'cmeq.2d v9, v5, v0'
// w2: $0x6ee08cca => 'cmeq.2d v10, v6, v0'
// w3: $0x6ee08ceb => 'cmeq.2d v11, v7, v0'
// w4: $0x6ee08d0c => 'cmeq.2d v12, v8, v0'
TEXT ·cmpEqI64VecI64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x6ee08ca9, $0x6ee08cca, $0x6ee08ceb, $0x6ee08d0c)

// func cmpGtF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x6ee0e4a9 => 'fcmgt.2d v9, v5, v0'
// w2: $0x6ee0e4ca => 'fcmgt.2d v10, v6, v0'
// w3: $0x6ee0e4eb => 'fcmgt.2d v11, v7, v0'
// w4: $0x6ee0e50c => 'fcmgt.2d v12, v8, v0'
TEXT ·cmpGtF64VecF64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x6ee0e4a9, $0x6ee0e4ca, $0x6ee0e4eb, $0x6ee0e50c)

// func cmpLtF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x6ee5e409 => 'fcmlt.2d v9, v5, v0'
// w2: $0x6ee6e40a => 'fcmlt.2d v10, v6, v0'
// w3: $0x6ee7e40b => 'fcmlt.2d v11, v7, v0'
// w4: $0x6ee8e40c => 'fcmlt.2d v12, v8, v0'
TEXT ·cmpLtF64VecF64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x6ee5e409, $0x6ee6e40a, $0x6ee7e40b, $0x6ee8e40c)

// func cmpGeF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x6e60e4a9 => 'fcmge.2d v9, v5, v0'
// w2: $0x6e60e4ca => 'fcmge.2d v10, v6, v0'
// w3: $0x6e60e4eb => 'fcmge.2d v11, v7, v0'
// w4: $0x6e60e50c => 'fcmge.2d v12, v8, v0'
TEXT ·cmpGeF64VecF64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x6e60e4a9, $0x6e60e4ca, $0x6e60e4eb, $0x6e60e50c)

// func cmpLeF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x6e65e409 => 'fcmle.2d v9, v5, v0'
// w2: $0x6e66e40a => 'fcmle.2d v10, v6, v0'
// w3: $0x6e67e40b => 'fcmle.2d v11, v7, v0'
// w4: $0x6e68e40c => 'fcmle.2d v12, v8, v0'
TEXT ·cmpLeF64VecF64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x6e65e409, $0x6e66e40a, $0x6e67e40b, $0x6e68e40c)

// func cmpEqF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x4e60e4a9 => 'fcmeq.2d v9, v5, v0'
// w2: $0x4e60e4ca => 'fcmeq.2d v10, v6, v0'
// w3: $0x4e60e4eb => 'fcmeq.2d v11, v7, v0'
// w4: $0x4e60e50c => 'fcmeq.2d v12, v8, v0'
TEXT ·cmpEqF64VecF64Lit(SB),NOSPLIT,$0-56
    vCompareIF64Lit($0x4e60e4a9, $0x4e60e4ca, $0x4e60e4eb, $0x4e60e50c)

#define vNotEqIF64Lit(w1, w2, w3, w4, w5, w6, w7, w8)      \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVD lit+48(FP), R5                                    \
    VDUP R5, V0.D2                                         \
    VMOVQ $0x0000000000000001, $0x0000000000000002, V1     \
    VMOVQ $0x0000000000000004, $0x0000000000000008, V2     \
    VMOVQ $0x0000000000000010, $0x0000000000000020, V3     \
    VMOVQ $0x0000000000000040, $0x0000000000000080, V4     \
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
    VLD1.P 64(R0), [V5.D2, V6.D2, V7.D2, V8.D2]            \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
                                                           \
    VAND V13.B16, V1.B16, V17.B16                          \
    VAND V14.B16, V2.B16, V18.B16                          \
    VAND V15.B16, V3.B16, V19.B16                          \
    VAND V16.B16, V4.B16, V20.B16                          \
                                                           \
    VORR V17.B16, V18.B16, V21.B16                         \
    VORR V19.B16, V20.B16, V22.B16                         \
    VORR V21.B16, V22.B16, V23.B16                         \
    VADDV V23.B16, V24                                     \
    VST1.P V24.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 8(R0), V5.D[0]                                  \
    WORD w1                                                \
    WORD w5                                                \
    VMOV V13.B[0], R8                                      \
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

// func cmpNeqI64VecI64Lit(src []int64, dst []byte, lit int64)
// w1: $0x6ee08ca9 => 'cmle.2d v9, v5, v0'
// w2: $0x6ee08cca => 'cmle.2d v10, v6, v0'
// w3: $0x6ee08ceb => 'cmle.2d v11, v7, v0'
// w4: $0x6ee08d0c => 'cmle.2d v12, v8, v0'
// w5: $0x6e20592d => 'not.16b v13, v9'
// w6: $0x6e20594e => 'not.16b v14, v10'
// w7: $0x6e20596f => 'not.16b v15, v11'
// w8: $0x6e205990 => 'not.16b v16, v12'
TEXT ·cmpNeqI64VecI64Lit(SB),NOSPLIT,$0-56
    vNotEqIF64Lit($0x6ee08ca9, $0x6ee08cca, $0x6ee08ceb, $0x6ee08d0c, $0x6e20592d, $0x6e20594e, $0x6e20596f, $0x6e205990)

// func cmpNeqF64VecF64Lit(src []float64, dst []byte, lit float64)
// w1: $0x4e60e4a9 => 'fcmeq.2d v9, v5, v0'
// w2: $0x4e60e4ca => 'fcmeq.2d v10, v6, v0'
// w3: $0x4e60e4eb => 'fcmeq.2d v11, v7, v0'
// w4: $0x4e60e50c => 'fcmeq.2d v12, v8, v0'
// w5: $0x6e20592d => 'not.16b v13, v9'
// w6: $0x6e20594e => 'not.16b v14, v10'
// w7: $0x6e20596f => 'not.16b v15, v11'
// w8: $0x6e205990 => 'not.16b v16, v12'
TEXT ·cmpNeqF64VecF64Lit(SB),NOSPLIT,$0-56
    vNotEqIF64Lit($0x4e60e4a9, $0x4e60e4ca, $0x4e60e4eb, $0x4e60e50c,  $0x6e20592d, $0x6e20594e, $0x6e20596f, $0x6e205990)

#define vCompareBetIF64Lit(w1, w2, w3, w4, w5, w6, w7, w8, vBOp) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $8, R2, R4                                         \
    MOVD min+48(FP), R5                                    \
    VDUP R5, V0.D2                                         \
    MOVD max+56(FP), R6                                    \
    VDUP R6, V1.D2                                         \
    VMOVQ $0x0000000000000001, $0x0000000000000002, V2     \
    VMOVQ $0x0000000000000004, $0x0000000000000008, V3     \
    VMOVQ $0x0000000000000010, $0x0000000000000020, V4     \
    VMOVQ $0x0000000000000040, $0x0000000000000080, V5     \
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
    VLD1.P 64(R0), [V6.D2, V7.D2, V8.D2, V9.D2]            \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    WORD w5                                                \
    WORD w6                                                \
    WORD w7                                                \
    WORD w8                                                \
                                                           \
    vBOp V10.B16, V14.B16, V18.B16                         \
    vBOp V11.B16, V15.B16, V19.B16                         \
    vBOp V12.B16, V16.B16, V20.B16                         \
    vBOp V13.B16, V17.B16, V21.B16                         \
                                                           \
    VAND V18.B16, V2.B16, V22.B16                          \
    VAND V19.B16, V3.B16, V23.B16                          \
    VAND V20.B16, V4.B16, V24.B16                          \
    VAND V21.B16, V5.B16, V25.B16                          \
                                                           \
    VORR V22.B16, V23.B16, V26.B16                         \
    VORR V24.B16, V25.B16, V27.B16                         \
    VORR V26.B16, V27.B16, V28.B16                         \
    VADDV V28.B16, V29                                     \
    VST1.P V29.B[0], 1(R1)                                 \
                                                           \
    ADD $8, R3, R3                                         \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 8(R0), V6.D[0]                                  \
    WORD w1                                                \
    WORD w5                                                \
    vBOp V10.B16, V14.B16, V18.B16                         \
    VMOV V18.B[0], R9                                      \
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

// func cmpBetI64VecI64Lit(src []int64, dst []byte, min int64, max int64)
// w1: $0x4ee03cca => 'cmge.2d v10, v6, v0'
// w2: $0x4ee03ceb => 'cmge.2d v11, v7, v0'
// w3: $0x4ee03d0c => 'cmge.2d v12, v8, v0'
// w4: $0x4ee03d2d => 'cmge.2d v13, v9, v0'
// w5: $0x4ee63c2e => 'cmle.2d v14, v6, v1'
// w6: $0x4ee73c2f => 'cmle.2d v15, v7, v1'
// w7: $0x4ee83c30 => 'cmle.2d v16, v8, v1'
// w8: $0x4ee93c31 => 'cmle.2d v17, v9, v1'
TEXT ·cmpBetI64VecI64Lit(SB),NOSPLIT,$0-64
    vCompareBetIF64Lit($0x4ee03cca, $0x4ee03ceb, $0x4ee03d0c, $0x4ee03d2d, $0x4ee63c2e, $0x4ee73c2f, $0x4ee83c30, $0x4ee93c31, VAND)

// func cmpNBetI64VecI64Lit(src []int64, dst []byte, min int64, max int64)
// w1: $0x4ee6340a => 'cmlt.2d v10, v6, v0'
// w2: $0x4ee7340b => 'cmlt.2d v11, v7, v0'
// w3: $0x4ee8340c => 'cmlt.2d v12, v8, v0'
// w4: $0x4ee9340d => 'cmlt.2d v13, v9, v0'
// w5: $0x4ee134ce => 'cmgt.2d v14, v6, v1'
// w6: $0x4ee134ef => 'cmgt.2d v15, v7, v1'
// w7: $0x4ee13510 => 'cmgt.2d v16, v8, v1'
// w8: $0x4ee13531 => 'cmgt.2d v17, v9, v1'
TEXT ·cmpNBetI64VecI64Lit(SB),NOSPLIT,$0-64
    vCompareBetIF64Lit($0x4ee6340a, $0x4ee7340b, $0x4ee8340c, $0x4ee9340d, $0x4ee134ce, $0x4ee134ef, $0x4ee13510, $0x4ee13531, VORR)

// func cmpBetI64VecI64Lit(src []int64, dst []byte, min int64, max int64)
// w1: $0x6e60e4ca => 'fcmge.2d v10, v6, v0'
// w2: $0x6e60e4eb => 'fcmge.2d v11, v7, v0'
// w3: $0x6e60e50c => 'fcmge.2d v12, v8, v0'
// w4: $0x6e60e52d => 'fcmge.2d v13, v9, v0'
// w5: $0x6e66e42e => 'fcmle.2d v14, v6, v1'
// w6: $0x6e67e42f => 'fcmle.2d v15, v7, v1'
// w7: $0x6e68e430 => 'fcmle.2d v16, v8, v1'
// w8: $0x6e69e431 => 'fcmle.2d v17, v9, v1'
TEXT ·cmpBetF64VecF64Lit(SB),NOSPLIT,$0-64
    vCompareBetIF64Lit($0x6e60e4ca, $0x6e60e4eb, $0x6e60e50c, $0x6e60e52d, $0x6e66e42e, $0x6e67e42f, $0x6e68e430, $0x6e69e431, VAND)

// func cmpNBetF64VecF64Lit(src []float64, dst []byte, min float64, max float64)
// w1: $0x6ee6e40a => 'fcmlt.2d v10, v6, v0'
// w2: $0x6ee7e40b => 'fcmlt.2d v11, v7, v0'
// w3: $0x6ee8e40c => 'fcmlt.2d v12, v8, v0'
// w4: $0x6ee9e40d => 'fcmlt.2d v13, v9, v0'
// w5: $0x6ee1e4ce => 'fcmgt.2d v14, v6, v1'
// w6: $0x6ee1e4ef => 'fcmgt.2d v15, v7, v1'
// w7: $0x6ee1e510 => 'fcmgt.2d v16, v8, v1'
// w8: $0x6ee1e531 => 'fcmgt.2d v17, v9, v1'
TEXT ·cmpNBetF64VecF64Lit(SB),NOSPLIT,$0-64
    vCompareBetIF64Lit($0x6ee6e40a, $0x6ee7e40b, $0x6ee8e40c, $0x6ee9e40d, $0x6ee1e4ce, $0x6ee1e4ef, $0x6ee1e510, $0x6ee1e531, VORR)
