//go:build amd64

#include "textflag.h"

#define vIF32toFI32(vMovOp, vOp, tOp, tMovOp, tReg)        \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $32, SI                                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $32                                           \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vOp (AX), Y1                                           \
    vOp 32(AX), Y2                                         \
    vOp 64(AX), Y3                                         \
    vOp 96(AX), Y4                                         \
    vMovOp Y1, (BX)                                        \
    vMovOp Y2, 32(BX)                                      \
    vMovOp Y3, 64(BX)                                      \
    vMovOp Y4, 96(BX)                                      \
    ADDQ $128, AX                                          \
    ADDQ $128, BX                                          \
    ADDQ $32, DI                                           \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tOp                                                    \
    tMovOp tReg, (BX)                                      \
    ADDQ $4, AX                                            \
    ADDQ $4, BX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

#define I32ToF32TConv VCVTSI2SSL (AX), X0, X0
#define F32ToI32TConv VCVTTSS2SI (AX), R8

// func castI32ToF32(src []int32, dst []float32)
TEXT ·castI32ToF32(SB),NOSPLIT,$0-48
    vIF32toFI32(VMOVUPS, VCVTDQ2PS, I32ToF32TConv, VMOVSS, X0)

// func castF32ToI32(src []float32, dst []int32)
TEXT ·castF32ToI32(SB),NOSPLIT,$0-48
    vIF32toFI32(VMOVDQU, VCVTTPS2DQ, F32ToI32TConv, MOVL, R8)

#define vIF32toIF64(vMovOp, vOp, tOp, tMovOp, tReg)        \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $16, SI                                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $16                                           \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vOp (AX), Y1                                           \
    vOp 16(AX), Y2                                         \
    vOp 32(AX), Y3                                         \
    vOp 48(AX), Y4                                         \
    vMovOp Y1, (BX)                                        \
    vMovOp Y2, 32(BX)                                      \
    vMovOp Y3, 64(BX)                                      \
    vMovOp Y4, 96(BX)                                      \
    ADDQ $64, AX                                           \
    ADDQ $128, BX                                          \
    ADDQ $16, DI                                           \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tOp                                                    \
    tMovOp tReg, (BX)                                      \
    ADDQ $4, AX                                            \
    ADDQ $8, BX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

#define I32ToI64TConv VCVTSI2SSL (AX), X0, X0   \
    VPBROADCASTD (AX), X0                       \
    VPMOVSXDQ X0, X0
#define F32ToF64TConv VCVTSS2SD (AX), X0, X0

// func castI32ToI64(src []int32, dst []int64)
TEXT ·castI32ToI64(SB),NOSPLIT,$0-48
    vIF32toIF64(VMOVDQU, VPMOVSXDQ, I32ToI64TConv, VMOVQ, X0)

// func castF32ToF64(src []float32, dst []float64)
TEXT ·castF32ToF64(SB),NOSPLIT,$0-48
    vIF32toIF64(VMOVUPD, VCVTPS2PD, F32ToF64TConv, VMOVSD, X0)

// func castI32ToF64(src []int32, dst []float64)
TEXT ·castI32ToF64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $16, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $16
    JLT tradLoop

vecLoop:
    VCVTDQ2PD (AX), Y1
    VCVTDQ2PD 16(AX), Y2
    VCVTDQ2PD 32(AX), Y3
    VCVTDQ2PD 48(AX), Y4
    VMOVDQU Y1, (BX)
    VMOVDQU Y2, 32(BX)
    VMOVDQU Y3, 64(BX)
    VMOVDQU Y4, 96(BX)
    ADDQ $64, AX
    ADDQ $128, BX
    ADDQ $16, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDL (AX), X1, X1
    VMOVSD X1, (BX)
    ADDQ $4, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func castF32ToI64(src []float32, dst []int64)
TEXT ·castF32ToI64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLT tradLoop

vecLoop:
    VCVTPS2PD (AX), Y1
    VEXTRACTI128 $1, Y1, X3 
    VUNPCKHPD X1, X1, X2
    VUNPCKHPD X3, X3, X4
    VCVTTSD2SIQ X1, R8
    VCVTTSD2SIQ X2, R9
    VCVTTSD2SIQ X3, R10
    VCVTTSD2SIQ X4, R11
    VMOVQ R8, X1
    VPINSRQ $1, R9, X1, X1
    VMOVQ R10, X2
    VPINSRQ $1, R11, X2, X2
    VINSERTI128 $1, X2, Y1, Y1
    VMOVDQU Y1, (BX)
    ADDQ $16, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTTSS2SIQ (AX), R8
    MOVQ R8, (BX)
    ADDQ $4, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func castI64ToF64(src []int64, dst []float64)
TEXT ·castI64ToF64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLT tradLoop

vecLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSI2SDQ 8(AX), X2, X2
    VCVTSI2SDQ 16(AX), X3, X3
    VCVTSI2SDQ 24(AX), X4, X4
    VUNPCKLPD X2, X1, X5
    VUNPCKLPD X4, X3, X6
    VINSERTF128 $1, X6, Y5, Y1
    VMOVUPD Y1, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    MOVQ X1, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func castF64ToI64(src []float64, dst []int64)
TEXT ·castF64ToI64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VEXTRACTI128 $1, Y1, X3 
    VUNPCKHPD X1, X1, X2
    VUNPCKHPD X3, X3, X4
    VCVTTSD2SIQ X1, R8
    VCVTTSD2SIQ X2, R9
    VCVTTSD2SIQ X3, R10
    VCVTTSD2SIQ X4, R11
    VMOVQ R8, X1
    VPINSRQ $1, R9, X1, X1
    VMOVQ R10, X2
    VPINSRQ $1, R11, X2, X2
    VINSERTI128 $1, X2, Y1, Y1
    VMOVUPS Y1, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTTSD2SIQ (AX), R8
    MOVQ R8, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func castI64ToF32(src []int64, dst []float32)
TEXT ·castI64ToF32(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $4
    JLT tradLoop

vecLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSI2SDQ 8(AX), X2, X2
    VCVTSI2SDQ 16(AX), X3, X3
    VCVTSI2SDQ 24(AX), X4, X4
    VUNPCKLPD X2, X1, X5
    VUNPCKLPD X4, X3, X6
    CVTPD2PS X5, X1
    CVTPD2PS X6, X2
    VMOVQ X1, (BX)
    VMOVQ X2, 8(BX)
    ADDQ $32, AX
    ADDQ $16, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSD2SS X1, X2, X2
    VMOVSS X2, (BX)
    ADDQ $8, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// // func castF64ToI32(src []float64, dst []int32)
// TEXT ·castF64ToI32(SB),NOSPLIT,$0-48
