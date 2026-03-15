//go:build amd64

#include "textflag.h"

#define vBCast(tReg, vBrdCstOp, vMovOp, tMovOp, dSize, chnkSize) \
    MOVQ dstAddr+0(FP), AX                                 \
    MOVQ dstLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    tMovOp lit+24(FP), tReg                                \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vBrdCstOp tReg, Y1                                     \
    vBrdCstOp tReg, Y2                                     \
    vBrdCstOp tReg, Y3                                     \
    vBrdCstOp tReg, Y4                                     \
    vMovOp Y1, (AX)                                        \
    vMovOp Y2, 32(AX)                                      \
    vMovOp Y3, 64(AX)                                      \
    vMovOp Y4, 96(AX)                                      \
    ADDQ $128, AX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp tReg, (AX)                                      \
    ADDQ dSize, AX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func broadcastB(dst []byte, lit byte)
TEXT ·broadcastB(SB),NOSPLIT,$0-25
    vBCast(R9, VPBROADCASTB, VMOVDQU, MOVB, $1, $128)

// func broadcastI64(dst []int64, lit int64)
TEXT ·broadcastI64(SB),NOSPLIT,$0-32
    vBCast(R9, VPBROADCASTQ, VMOVDQU, MOVQ, $8, $16)

// func broadcastI32(dst []int32, lit int32)
TEXT ·broadcastI32(SB),NOSPLIT,$0-28
    vBCast(R9, VPBROADCASTD, VMOVDQU, MOVL, $4, $32)

// func broadcastF64(dst []float64, lit float64)
TEXT ·broadcastF64(SB),NOSPLIT,$0-32
    vBCast(X0, VBROADCASTSD, VMOVUPD, VMOVSD, $8, $16)

// func broadcastF32(dst []float32, lit float32)
TEXT ·broadcastF32(SB),NOSPLIT,$0-28
    vBCast(X0, VBROADCASTSS, VMOVUPS, VMOVSS, $4, $32)
