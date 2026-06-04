//go:build amd64

#include "textflag.h"

#define ASCII_DIFF $32
#define ASCII_LC_MIN $97
#define ASCII_LC_MAX $122
#define ASCII_UC_MIN $65
#define ASCII_UC_MAX $90

#define vCaseASCII(min_ascii, max_ascii, vDiffOp)          \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $64, SI                                           \
                                                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    MOVQ min_ascii, R8                                     \
    VPBROADCASTB, R8, Y0                                   \
    MOVQ max_ascii, R8                                     \
    VPBROADCASTB, R8, Y1                                   \
    MOVQ ASCII_DIFF, R8                                    \
    VPBROADCASTB, R8, Y2                                   \
                                                           \
    CMPQ CX, $64                                           \
    JLE tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    VMOVDQU (AX), Y3                                       \
    VMOVDQU 32(AX), Y4                                     \
    VPMAXUB Y3, Y0, Y5                                     \
    VPMINUB Y3, Y1, Y6                                     \
    VPMAXUB Y4, Y0, Y7                                     \
    VPMINUB Y4, Y1, Y8                                     \
    VPCMPEQB Y5, Y6, Y5                                    \
    VPCMPEQB Y7, Y8, Y7                                    \
    VPAND Y5, Y2, Y5                                       \
    VPAND Y7, Y2, Y7                                       \
    vDiffOp Y2, Y3, Y3                                     \
    vDiffOp Y2, Y5, Y4                                     \
    VMOVDQU Y3, (BX)                                       \
    VMOVDQU Y4, 32(BX)                                     \
    ADDQ $64, AX                                           \
    ADDQ $64, BX                                           \
    ADDQ $64, DI                                           \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \
    XORQ R8, R8                                            \
    VPXOR Y3, Y3, Y3                                       \                                                           
tradLoop:                                                  \
    MOVB (AX), R8                                          \
    VMOVD R8, Y3                                           \
    VPMAXUB Y3, Y0, Y5                                     \
    VPMINUB Y3, Y1, Y6                                     \
    VPCMPEQB Y5, Y6, Y5                                    \
    VPAND Y5, Y2, Y5                                       \
    vDiffOp Y2, Y3, Y3                                     \
    VMOVD Y3, R8                                           \
    MOVB R8, (BX)                                          \
    ADDQ $1, AX                                            \
    ADDQ $1, BX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func toUpperASCII(src []byte, dst []byte)
TEXT ·toUpperASCII(SB),NOSPLIT,$0-48
    vCaseASCII(ASCII_LC_MIN, ASCII_LC_MAX, VPSUBB)

// func toLowerASCII(src []byte, dst []byte)
TEXT ·toLowerASCII(SB),NOSPLIT,$0-48
    vCaseASCII(ASCII_UC_MIN, ASCII_UC_MAX, VPADDB)
