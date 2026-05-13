//go:build amd64

#include "textflag.h"

#define vNegInt(xorOp, vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    VPXOR Y0, Y0, Y0                                       \
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
    vOp Y1, Y0, Y5                                         \
    vOp Y2, Y0, Y6                                         \
    vOp Y3, Y0, Y7                                         \
    vOp Y4, Y0, Y8                                         \
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
    tMovOp (AX), R10                                       \
    xorOp R9, R9                                           \
    tOp R10, R9                                            \
    tMovOp R9, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func negI64(src, dst []int64)
TEXT ·negI64(SB),NOSPLIT,$0-48
    vNegInt(XORQ, VMOVDQU, VPSUBQ, MOVQ, SUBQ, $8, $16)
    
// func negI32(src, dst []int32)
TEXT ·negI32(SB),NOSPLIT,$0-48
    vNegInt(XORL, VMOVDQU, VPSUBD, MOVL, SUBL, $4, $32)

// func sqI32(src, dst []int32)
TEXT ·sqI32(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLE tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VPMULLD Y1, Y1, Y5
    VPMULLD Y2, Y2, Y6
    VPMULLD Y3, Y3, Y7
    VPMULLD Y4, Y4, Y8
    VMOVDQU Y5, (BX)
    VMOVDQU Y6, 32(BX)
    VMOVDQU Y7, 64(BX)
    VMOVDQU Y8, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    MOVL (AX), R8
    IMULL R8, R8
    MOVL R8, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func sqrtI64(src []int64, dst []float64)
TEXT ·sqrtI64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLE tradLoop

vecLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSI2SDQ 8(AX), X2, X2
    VCVTSI2SDQ 16(AX), X3, X3
    VCVTSI2SDQ 24(AX), X4, X4
    VUNPCKLPD X2, X1, X5
    VUNPCKLPD X4, X3, X6
    VINSERTF128 $1, X6, Y5, Y7
    VSQRTPD Y7, Y8
    VMOVUPD Y8, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    VSQRTSD X1, X1, X2
    VMOVSD X2, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func sqrtI32(src []int32, dst []float32)
TEXT ·sqrtI32(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLE tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VCVTDQ2PS Y1, Y5
    VCVTDQ2PS Y2, Y6
    VCVTDQ2PS Y3, Y7
    VCVTDQ2PS Y4, Y8
    VSQRTPS Y5, Y9
    VSQRTPS Y6, Y10
    VSQRTPS Y7, Y11
    VSQRTPS Y8, Y12
    VMOVUPS Y9, (BX)
    VMOVUPS Y10, 32(BX)
    VMOVUPS Y11, 64(BX)
    VMOVUPS Y12, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    MOVL (AX), R9
    VCVTSI2SSL R9, X1, X1
    VSQRTSS X1, X1, X2
    VMOVSS X2, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func recipI64(src []int64, dst []float64)
TEXT ·recipI64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI
    MOVSD $1.00, X0
    VBROADCASTSD X0, Y0

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLE tradLoop

vecLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSI2SDQ 8(AX), X2, X2
    VCVTSI2SDQ 16(AX), X3, X3
    VCVTSI2SDQ 24(AX), X4, X4
    VUNPCKLPD X2, X1, X5
    VUNPCKLPD X4, X3, X6
    VINSERTF128 $1, X6, Y5, Y7
    VDIVPD Y7, Y0, Y8
    VMOVUPD Y8, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    VDIVSD X1, X0, X2
    VMOVSD X2, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func recipI32(src []int32, dst []float32)
TEXT ·recipI32(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI
    MOVSS $1.00, X0
    VBROADCASTSS X0, Y0

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLE tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VCVTDQ2PS Y1, Y5
    VCVTDQ2PS Y2, Y6
    VCVTDQ2PS Y3, Y7
    VCVTDQ2PS Y4, Y8
    VDIVPS Y5, Y0, Y9
    VDIVPS Y6, Y0, Y10
    VDIVPS Y7, Y0, Y11
    VDIVPS Y8, Y0, Y12
    VMOVUPS Y9, (BX)
    VMOVUPS Y10, 32(BX)
    VMOVUPS Y11, 64(BX)
    VMOVUPS Y12, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    MOVL (AX), R9
    VCVTSI2SSL R9, X1, X1
    VDIVSS X1, X0, X2
    VMOVSS X2, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET
    
// func absI32(src, dst []int32)
TEXT ·absI32(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLE tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VPABSD Y1, Y5
    VPABSD Y2, Y6
    VPABSD Y3, Y7
    VPABSD Y4, Y8
    VMOVUPS Y5, (BX)
    VMOVUPS Y6, 32(BX)
    VMOVUPS Y7, 64(BX)
    VMOVUPS Y8, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VMOVD (AX), X1
    VPABSD X1, X2
    VMOVD X2, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

