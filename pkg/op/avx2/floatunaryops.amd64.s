//go:build amd64

#include "textflag.h"

#define vOpUnaryTwoOprFloat(vr1, vr2, vr3, vr4, vr5, cScalar, vBrdCstOp, vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    tMovOp cScalar, X0                                     \
    vBrdCstOp X0, Y0                                       \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vMovOp 32(AX), Y2                                      \
    vMovOp 64(AX), Y3                                      \
    vMovOp 96(AX), Y4                                      \
    vOp Y1, vr1, Y5                                        \
    vOp Y2, vr2, Y6                                        \
    vOp Y3, vr3, Y7                                        \
    vOp Y4, vr4, Y8                                        \
    vMovOp Y5, (BX)                                        \
    vMovOp Y6, 32(BX)                                      \
    vMovOp Y7, 64(BX)                                      \
    vMovOp Y8, 96(BX)                                      \
    ADDQ $128, AX                                          \
    ADDQ $128, BX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (AX), X1                                        \
    tOp X1, vr5, X5                                        \
    tMovOp X5, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func sqF64(src, dst []float64)
TEXT ·sqF64(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y1, Y2, Y3, Y4, X1, $0.00, VBROADCASTSD, VMOVUPD, VMULPD, MOVSD, VMULSD, $8, $16)

// func sqF32(src, dst []float32)
TEXT ·sqF32(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y1, Y2, Y3, Y4, X1, $0.00, VBROADCASTSS, VMOVUPS, VMULPS, MOVSS, VMULSS, $4, $32)

// func negF64(src, dst []float64)
TEXT ·negF64(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y0, Y0, Y0, Y0, X0, $0.00, VBROADCASTSD, VMOVUPD, VSUBPD, MOVSD, VSUBSD, $8, $16)

// func negF32(src, dst []float32)
TEXT ·negF32(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y0, Y0, Y0, Y0, X0, $0.00, VBROADCASTSS, VMOVUPS, VSUBPS, MOVSS, VSUBSS, $4, $32)

// func recipF64(src, dst []float64)
TEXT ·recipF64(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y0, Y0, Y0, Y0, X0, $1.00, VBROADCASTSD, VMOVUPD, VDIVPD, MOVSD, VDIVSD, $8, $16)

// func recipF32(src, dst []float32)
TEXT ·recipF32(SB),NOSPLIT,$0-48
    vOpUnaryTwoOprFloat(Y0, Y0, Y0, Y0, X0, $1.00, VBROADCASTSS, VMOVUPS, VDIVPS, MOVSS, VDIVSS, $4, $32)

#define vOpUnaryOneOprFloat(vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vMovOp 32(AX), Y2                                      \
    vMovOp 64(AX), Y3                                      \
    vMovOp 96(AX), Y4                                      \
    vOp Y1, Y5                                             \
    vOp Y2, Y6                                             \
    vOp Y3, Y7                                             \
    vOp Y4, Y8                                             \
    vMovOp Y5, (BX)                                        \
    vMovOp Y6, 32(BX)                                      \
    vMovOp Y7, 64(BX)                                      \
    vMovOp Y8, 96(BX)                                      \
    ADDQ $128, AX                                          \
    ADDQ $128, BX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (AX), X1                                        \
    tOp X1, X1,  X5                                        \
    tMovOp X5, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func sqrtF64(src, dst []float64)
TEXT ·sqrtF64(SB),NOSPLIT,$0-48
    vOpUnaryOneOprFloat(VMOVUPD, VSQRTPD, MOVSD, VSQRTSD, $8, $16)

// func sqrtF32(src, dst []float32)
TEXT ·sqrtF32(SB),NOSPLIT,$0-48
    vOpUnaryOneOprFloat(VMOVUPS, VSQRTPS, MOVSS, VSQRTSS, $4, $32)

#define vAbsFloat(iMovOp, cScalar, notOp, vBrdCstOp, vMovOp, vOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    iMovOp cScalar, R8                                     \
    notOp R8                                               \
    vBrdCstOp R8, Y0                                       \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vMovOp 32(AX), Y2                                      \
    vMovOp 64(AX), Y3                                      \
    vMovOp 96(AX), Y4                                      \
    vOp Y0, Y1, Y5                                         \
    vOp Y0, Y2, Y6                                         \
    vOp Y0, Y3, Y7                                         \
    vOp Y0, Y4, Y8                                         \
    vMovOp Y5, (BX)                                        \
    vMovOp Y6, 32(BX)                                      \
    vMovOp Y7, 64(BX)                                      \
    vMovOp Y8, 96(BX)                                      \
    ADDQ $128, AX                                          \
    ADDQ $128, BX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    iMovOp (AX), R9                                        \
    tOp R8, R9                                             \
    iMovOp R9, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func absF64(src, dst []float64)
TEXT ·absF64(SB),NOSPLIT,$0-48
    vAbsFloat(MOVQ, $0x8000000000000000, NOTQ, VPBROADCASTQ, VMOVDQU, VPAND, ANDQ, $8, $16)

// func absF32(src, dst []float32)
TEXT ·absF32(SB),NOSPLIT,$0-48
    vAbsFloat(MOVL, $0x80000000, NOTQ, VPBROADCASTD, VMOVDQU, VPAND, ANDQ, $4, $32)
