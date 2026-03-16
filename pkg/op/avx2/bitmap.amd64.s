//go:build amd64

#include "textflag.h"

#define combineAndReturnSum(bOpQ, bOpW)                    \
    MOVQ src1Addr+0(FP), AX                                \
    MOVQ src2Addr+24(FP), BX                               \
    MOVQ dstAddr+48(FP), R8                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $16, SI                                           \
    XORQ R13, R13                                          \
    XORQ R9, R9                                            \
    XORQ R10, R10                                          \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $16                                           \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    MOVQ (AX), R9                                          \
    MOVQ (BX), R10                                         \
    MOVQ 8(AX), R11                                        \
    MOVQ 8(BX), R12                                        \
    bOpQ R9, R10                                           \
    bOpQ R11, R12                                          \
    POPCNTQ R10, R9                                        \
    POPCNTQ R12, R11                                       \
    ADDQ R9, R13                                           \
    ADDQ R11, R13                                          \
    MOVQ R10, (R8)                                         \
    MOVQ R12, 8(R8)                                        \
    ADDQ $16, AX                                           \
    ADDQ $16, BX                                           \
    ADDQ $16, R8                                           \
    ADDQ $16, DI                                           \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
    XORQ R9, R9                                            \
    XORQ R10, R10                                          \
tradLoop:                                                  \
    MOVB (AX), R9                                          \
    MOVB (BX), R10                                         \
    bOpW R9, R10                                           \
    POPCNTW R10, R9                                        \
    ADDQ R9, R13                                           \
    MOVB R10, (R8)                                         \
    ADDQ $1, AX                                            \
    ADDQ $1, BX                                            \
    ADDQ $1, R8                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    MOVQ R13, cnt+72(FP)                                   \
    RET

// func bitmapANDRetPopCount(src1, src2, dst []byte) uint64
TEXT ·bitmapANDRetPopCount(SB),NOSPLIT,$0-80
    combineAndReturnSum(ANDQ, ANDW)

// func bitmapORRetPopCount(src1, src2, dst []byte) uint64
TEXT ·bitmapORRetPopCount(SB),NOSPLIT,$0-80
    combineAndReturnSum(ORQ, ORW)
    
// func bitmapPopCount(src []byte) uint64
TEXT ·bitmapPopCount(SB),NOSPLIT,$0-32
    MOVQ srcAddr+0(FP), AX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $24, SI
    XORQ BX, BX
    XORQ R9, R9
    XORQ R10, R10
    XORQ R12, R12
    XORQ R13, R13

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $24
    JLT tradLoop

vecLoop:
    POPCNTQ (AX), R8
    POPCNTQ 8(AX), R9
    POPCNTQ 16(AX), R10
    ADDQ R8, BX
    ADDQ R9, R12
    ADDQ R10, R13
    ADDQ $24, AX
    ADDQ $24, DI
    CMPQ DI, SI 
    JLT vecLoop

    XORQ R9, R9
    XORQ R10, R10
tradLoop:
    MOVB (AX), R9
    POPCNTW R9, R10
    ADDQ R10, BX 
    ADDQ $1, AX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    ADDQ BX, R12
    ADDQ R12, R13
    MOVQ R13, cnt+24(FP)
    RET
