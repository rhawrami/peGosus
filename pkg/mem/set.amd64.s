//go:build amd64

#include "textflag.h"

#define vSet(tReg, vBrdCstOp, vMovOp, tMovOp, dSize, chnkSize) \
    MOVQ dstAddr+0(FP), AX                                 \
    MOVQ dstLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    tMovOp lit+16(FP), tReg                                \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLE tradLoop                                           \
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

// func setU8UnalignedAVX2(a *byte, n int, v uint8)
TEXT ·setU8UnalignedAVX2(SB),NOSPLIT,$0-17
    vSet(R9, VPBROADCASTB, VMOVDQU, MOVB, $1, $128)

// func setU32UnalignedAVX2(a *byte, n int, v uint32)
TEXT ·setU32UnalignedAVX2(SB),NOSPLIT,$0-20
    vSet(R9, VPBROADCASTD, VMOVDQU, MOVL, $4, $32)

// func setU64UnalignedAVX2(a *byte, n int, v uint64)
TEXT ·setU64UnalignedAVX2(SB),NOSPLIT,$0-24
    vSet(R9, VPBROADCASTQ, VMOVDQU, MOVQ, $8, $16)
