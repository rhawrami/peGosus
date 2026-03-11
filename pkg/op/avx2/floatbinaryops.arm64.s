//go:build amd64

#include "textflag.h"

#define vOpLitFloat(vBrdCstOp, vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    XORQ DI, DI                                            \
    SUBQ chnkSize, CX, SI                                  \
    vBrdCstOp lit+48(FP), Y0                               \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ chnkSize, CX                                      \
    JLT tradLoop                                           \
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
    CMPQ SI, DI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (R0), X1                                        \
    tOp X0, X1, X5                                         \
    tMovOp X1, (R1)                                        \
    ADD dSize, AX                                          \
    ADD dSize, BX                                          \
    ADD $1, DI                                             \
    CMPQ R2, DI                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·addF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(VBROADCASTSD, VMOVUPD, VADDPD, VMOVSD, VADDSD, $8, $16)

// func addF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·addF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(VBROADCASTSS, VMOVUPS, VADDPS, VMOVSS, VADDSS, $4, $32)

// func subF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·subF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(VBROADCASTSD, VMOVUPD, VSUBPD, VMOVSD, VSUBSD, $8, $16)

// func subF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·subF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(VBROADCASTSS, VMOVUPS, VSUBPS, VMOVSS, VSUBSS, $4, $32)

// func mulF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·mulF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(VBROADCASTSD, VMOVUPD, VMULPD, VMOVSD, VMULSD, $8, $16)

// func mulF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·mulF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(VBROADCASTSS, VMOVUPS, VMULPS, VMOVSS, VMULSS, $4, $32)

// func divF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·divF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(VBROADCASTSD, VMOVUPD, VDIVPD, VMOVSD, VDIVSD, $8, $16)

// func divF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·divF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(VBROADCASTSS, VMOVUPS, VDIVPS, VMOVSS, VDIVSS, $4, $32)

