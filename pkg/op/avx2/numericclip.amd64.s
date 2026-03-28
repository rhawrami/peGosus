//go:build amd64

#include "textflag.h"

#define vClipFloat(maxOff, vBrdCstOp, vMovOp, vMinOp, vMaxOp, tMovOp, tMinOp, tMaxOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    vBrdCstOp min+48(FP), Y0                               \
    vBrdCstOp max+maxOff(FP), Y1                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y2                                        \
    vMovOp 32(AX), Y3                                      \
    vMovOp 64(AX), Y4                                      \
    vMovOp 96(AX), Y5                                      \
    vMaxOp Y0, Y2, Y6                                      \
    vMaxOp Y0, Y3, Y7                                      \
    vMaxOp Y0, Y4, Y8                                      \
    vMaxOp Y0, Y5, Y9                                      \
    vMinOp Y1, Y6, Y10                                     \
    vMinOp Y1, Y7, Y11                                     \
    vMinOp Y1, Y8, Y12                                     \
    vMinOp Y1, Y9, Y13                                     \
    vMovOp Y10, (BX)                                       \
    vMovOp Y11, 32(BX)                                     \
    vMovOp Y12, 64(BX)                                     \
    vMovOp Y13, 96(BX)                                     \
    ADDQ $128, AX                                          \
    ADDQ $128, BX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (AX), X2                                        \
    tMaxOp X0, X2, X3                                      \
    tMinOp X1, X3, X4                                      \
    tMovOp X4, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func clipF64WithF64Bounds(src, dst []float64, lower, upper float64)
TEXT ·clipF64WithF64Bounds(SB),NOSPLIT,$0-64
    vClipFloat(56, VBROADCASTSD, VMOVUPD, VMINPD, VMAXPD, MOVSD, VMINSD, VMAXSD, $8, $16)

// func clipF32WithF32Bounds(src, dst []float32, lower, upper float32)
TEXT ·clipF32WithF32Bounds(SB),NOSPLIT,$0-56
    vClipFloat(52, VBROADCASTSS, VMOVUPS, VMINPS, VMAXPS, MOVSS, VMINSS, VMAXSS, $4, $32)
    
// func clipI32WithI32Bounds(src, dst []int32, lower, upper int32)
TEXT ·clipI32WithI32Bounds(SB),NOSPLIT,$0-56
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI
    VPBROADCASTD min+48(FP), Y0
    VPBROADCASTD max+52(FP), Y1

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y2
    VMOVDQU 32(AX), Y3
    VMOVDQU 64(AX), Y4
    VMOVDQU 96(AX), Y5
    VPMAXSD Y0, Y2, Y6
    VPMAXSD Y0, Y3, Y7
    VPMAXSD Y0, Y4, Y8
    VPMAXSD Y0, Y5, Y9
    VPMINSD Y1, Y6, Y10
    VPMINSD Y1, Y7, Y11
    VPMINSD Y1, Y8, Y12
    VPMINSD Y1, Y9, Y13
    VMOVDQU Y10, (BX)
    VMOVDQU Y11, 32(BX)
    VMOVDQU Y12, 64(BX)
    VMOVDQU Y13, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VMOVD (AX), X2
    VPMAXSD X0, X2, X3
    VPMINSD X1, X3, X4
    VMOVD X4, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET
    
// func clipI64WithI64Bounds(src, dst []int64, lower, upper int64)
TEXT ·clipI64WithI64Bounds(SB),NOSPLIT,$0-64
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $16, SI
    VPBROADCASTQ min+48(FP), Y0
    VPBROADCASTQ max+56(FP), Y1

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $16
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y2
    VMOVDQU 32(AX), Y3
    VMOVDQU 64(AX), Y4
    VMOVDQU 96(AX), Y5
    VPCMPGTQ Y2, Y0, Y6
    VPCMPGTQ Y3, Y0, Y7
    VPCMPGTQ Y4, Y0, Y8
    VPCMPGTQ Y5, Y0, Y9
    VPBLENDVB Y6, Y0, Y2, Y2
    VPBLENDVB Y7, Y0, Y3, Y3
    VPBLENDVB Y8, Y0, Y4, Y4
    VPBLENDVB Y9, Y0, Y5, Y5
    VPCMPGTQ Y1, Y2, Y6
    VPCMPGTQ Y1, Y3, Y7
    VPCMPGTQ Y1, Y4, Y8
    VPCMPGTQ Y1, Y5, Y9
    VPBLENDVB Y6, Y1, Y2, Y2
    VPBLENDVB Y7, Y1, Y3, Y3
    VPBLENDVB Y8, Y1, Y4, Y4
    VPBLENDVB Y9, Y1, Y5, Y5
    VMOVDQU Y2, (BX)
    VMOVDQU Y3, 32(BX)
    VMOVDQU Y4, 64(BX)
    VMOVDQU Y5, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $16, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VMOVQ (AX), X2
    VPCMPGTQ X2, X0, X3
    VPBLENDVB X3, X0, X2, X2
    VPCMPGTQ X1, X2, X3
    VPBLENDVB X3, X1, X2, X2
    VMOVQ X2, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func clipI64WithF64Bounds(src []int64, dst []float64, lower, upper float64)
TEXT ·clipI64WithF64Bounds(SB),NOSPLIT,$0-64
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI
    VBROADCASTSD min+48(FP), Y0
    VBROADCASTSD max+56(FP), Y1

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLT tradLoop

vecLoop:
    VCVTSI2SDQ (AX), X2, X2
    VCVTSI2SDQ 8(AX), X3, X3
    VCVTSI2SDQ 16(AX), X4, X4
    VCVTSI2SDQ 24(AX), X5, X5
    VUNPCKLPD X3, X2, X6
    VUNPCKLPD X5, X4, X7
    VINSERTF128 $1, X7, Y6, Y8
    VMAXPD Y0, Y8, Y2
    VMINPD Y1, Y2, Y3
    VMOVUPD Y3, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X2, X2
    VMINSD X1, X2, X3
    VMAXSD X0, X3, X4
    VMOVSD X4, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET
    
// func clipI32WithF32Bounds(src []int32, dst []float32, lower, upper float32)
TEXT ·clipI32WithF32Bounds(SB),NOSPLIT,$0-56
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI
    VBROADCASTSS min+48(FP), Y0
    VBROADCASTSS max+52(FP), Y1

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y2
    VMOVDQU 32(AX), Y3
    VMOVDQU 64(AX), Y4
    VMOVDQU 96(AX), Y5
    VCVTDQ2PS Y2, Y6
    VCVTDQ2PS Y3, Y7
    VCVTDQ2PS Y4, Y8
    VCVTDQ2PS Y5, Y9
    VMAXPS Y0, Y6, Y10
    VMAXPS Y0, Y7, Y11
    VMAXPS Y0, Y8, Y12
    VMAXPS Y0, Y9, Y13
    VMINPS Y1, Y10, Y2
    VMINPS Y1, Y11, Y3
    VMINPS Y1, Y12, Y4
    VMINPS Y1, Y13, Y5
    VMOVUPS Y2, (BX)
    VMOVUPS Y3, 32(BX)
    VMOVUPS Y4, 64(BX)
    VMOVUPS Y5, 96(BX)
    ADDQ $128, AX
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    MOVL (AX), R9
    VCVTSI2SSL R9, X2, X2
    VMAXSS X0, X2, X3
    VMINSS X1, X3, X4
    VMOVSS X4, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET
