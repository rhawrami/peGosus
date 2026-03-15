//go:build amd64

#include "textflag.h"   

#define vOpLitInt(vBrdCstOp, vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
    tMovOp lit+48(FP), R9                                  \
    vBrdCstOp R9, Y0                                       \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
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
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (AX), R10                                       \
    tOp R9, R10                                            \
    tMovOp R10, (BX)                                       \
    ADDQ dSize, AX                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addI64VecI64Lit(src, dst []int64, lit int64)
TEXT ·addI64VecI64Lit(SB),NOSPLIT,$0-56
    vOpLitInt(VPBROADCASTQ, VMOVDQU, VPADDQ, MOVQ, ADDQ, $8, $16)

// func addI32VecI32Lit(src, dst []int32, lit int32)
TEXT ·addI32VecI32Lit(SB),NOSPLIT,$0-52
    vOpLitInt(VPBROADCASTD, VMOVDQU, VPADDD, MOVL, ADDL, $4, $32)

// func subI64VecI64Lit(src, dst []int64, lit int64)
TEXT ·subI64VecI64Lit(SB),NOSPLIT,$0-56
    vOpLitInt(VPBROADCASTQ, VMOVDQU, VPSUBQ, MOVQ, SUBQ, $8, $16)

// func subI32VecI32Lit(src, dst []int32, lit int32)
TEXT ·subI32VecI32Lit(SB),NOSPLIT,$0-52
    vOpLitInt(VPBROADCASTD, VMOVDQU, VPSUBD, MOVL, SUBL, $4, $32)

// func mulI32VecI32Lit(src, dst []int32, lit int32)
TEXT ·mulI32VecI32Lit(SB),NOSPLIT,$0-52
    vOpLitInt(VPBROADCASTD, VMOVDQU, VPMULLD, MOVL, IMULL, $4, $32)

#define vOpVecInt(vMovOp, vOp, tMovOp, tOp, dSize, chnkSize) \
    MOVQ src1Addr+0(FP), AX                                \
    MOVQ src2Addr+24(FP), R8                               \
    MOVQ dstAddr+48(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ chnkSize, SI                                      \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vMovOp 32(AX), Y2                                      \
    vMovOp 64(AX), Y3                                      \
    vMovOp 96(AX), Y4                                      \
    vMovOp (R8), Y5                                        \
    vMovOp 32(R8), Y6                                      \
    vMovOp 64(R8), Y7                                      \
    vMovOp 96(R8), Y8                                      \
    vOp Y5, Y1, Y9                                         \
    vOp Y6, Y2, Y10                                        \
    vOp Y7, Y3, Y11                                        \
    vOp Y8, Y4, Y12                                        \
    vMovOp Y9, (BX)                                        \
    vMovOp Y10, 32(BX)                                     \
    vMovOp Y11, 64(BX)                                     \
    vMovOp Y12, 96(BX)                                     \
    ADDQ $128, AX                                          \
    ADDQ $128, R8                                          \
    ADDQ $128, BX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    tMovOp (AX), R9                                        \
    tMovOp (R8), R10                                       \
    tOp R10, R9                                            \
    tMovOp R9, (BX)                                        \
    ADDQ dSize, AX                                         \
    ADDQ dSize, R8                                         \
    ADDQ dSize, BX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addI64VecI64Vec(src1, src2, dst []float64)
TEXT ·addI64VecI64Vec(SB),NOSPLIT,$0-72
    vOpVecInt(VMOVDQU, VPADDQ, MOVQ, ADDQ, $8, $16)

// func addI32VecI32Vec(src1, src2, dst []float32)
TEXT ·addI32VecI32Vec(SB),NOSPLIT,$0-72
    vOpVecInt(VMOVDQU, VPADDD, MOVL, ADDL, $4, $32)

// func subI64VecI64Vec(src1, src2, dst []float64)
TEXT ·subI64VecI64Vec(SB),NOSPLIT,$0-72
    vOpVecInt(VMOVDQU, VPSUBQ, MOVQ, SUBQ, $8, $16)

// func subI32VecI32Vec(src1, src2, dst []float32)
TEXT ·subI32VecI32Vec(SB),NOSPLIT,$0-72
    vOpVecInt(VMOVDQU, VPSUBD, MOVL, SUBL, $4, $32)

// func mulI32VecI32Vec(src1, src2, dst []float32)
TEXT ·mulI32VecI32Vec(SB),NOSPLIT,$0-72
    vOpVecInt(VMOVDQU, VPMULLD, MOVL, IMULL, $4, $32)

// func divI32VecI32Lit(src []int32, dst []float32, lit float32)
TEXT ·divI32VecI32Lit(SB),NOSPLIT,$0-56
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI
    VBROADCASTSS lit+48(FP), Y0

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VCVTDQ2PS Y1, Y5
    VCVTDQ2PS Y2, Y6
    VCVTDQ2PS Y3, Y7
    VCVTDQ2PS Y4, Y8
    VDIVPS Y0, Y5, Y9
    VDIVPS Y0, Y6, Y10
    VDIVPS Y0, Y7, Y11
    VDIVPS Y0, Y8, Y12
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
    VDIVSS X0, X1, X2
    VMOVSS X2, (BX)
    ADDQ $4, AX
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func divI32VecI32Vec(src1, src2 []int32, dst []float32)
TEXT ·divI32VecI32Vec(SB),NOSPLIT,$0-72
    MOVQ src1Addr+0(FP), AX
    MOVQ src2Addr+24(FP), R8
    MOVQ dstAddr+48(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $32, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y1
    VMOVDQU 32(AX), Y2
    VMOVDQU 64(AX), Y3
    VMOVDQU 96(AX), Y4
    VMOVDQU (R8), Y5
    VMOVDQU 32(R8), Y6
    VMOVDQU 64(R8), Y7
    VMOVDQU 96(R8), Y8
    VCVTDQ2PS Y1, Y1
    VCVTDQ2PS Y2, Y2
    VCVTDQ2PS Y3, Y3
    VCVTDQ2PS Y4, Y4
    VCVTDQ2PS Y5, Y5
    VCVTDQ2PS Y6, Y6
    VCVTDQ2PS Y7, Y7
    VCVTDQ2PS Y8, Y8
    VDIVPS Y5, Y1, Y9
    VDIVPS Y6, Y2, Y10
    VDIVPS Y7, Y3, Y11
    VDIVPS Y8, Y4, Y12
    VMOVDQU Y9, (BX)
    VMOVDQU Y10, 32(BX)
    VMOVDQU Y11, 64(BX)
    VMOVDQU Y12, 96(BX)
    ADDQ $128, AX
    ADDQ $128, R8
    ADDQ $128, BX
    ADDQ $32, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    MOVL (AX), R9
    MOVL (R8), R10
    VCVTSI2SSL R9, X1, X1
    VCVTSI2SSL R10, X2, X2
    VDIVSS X2, X1, X3
    VMOVSS X3, (BX)
    ADDQ $4, AX
    ADDQ $4, R8
    ADDQ $4, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func divI64VecI64Lit(src []int64, dst []float64, lit float64)
TEXT ·divI64VecI64Lit(SB),NOSPLIT,$0-56
    MOVQ srcAddr+0(FP), AX
    MOVQ dstAddr+24(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI
    VBROADCASTSD lit+48(FP), Y0

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
    VINSERTF128 $1, X6, Y5, Y7
    VDIVPD Y0, Y7, Y8
    VMOVUPD Y8, (BX)
    ADDQ $32, AX
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    VDIVSD X0, X1, X2
    VMOVSD X2, (BX)
    ADDQ $8, AX
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// func divI64VecI64Vec(src1, src2 []int64, dst []float64)
TEXT ·divI64VecI64Vec(SB),NOSPLIT,$0-72
    MOVQ src1Addr+0(FP), AX
    MOVQ src2Addr+24(FP), R8
    MOVQ dstAddr+48(FP), BX
    MOVQ srcLen+8(FP), CX
    MOVQ CX, SI
    XORQ DI, DI
    SUBQ $4, SI

    TESTQ CX, CX
    JEQ exitFn

    CMPQ CX, $32
    JLT tradLoop

vecLoop:
    VMOVDQU (AX), Y1 
    VMOVDQU (R8), Y2 
    VEXTRACTI128 $1, Y1, X3 
    VEXTRACTI128 $1, Y2, X4 
    VPEXTRQ $0, X1, R9
    VPEXTRQ $1, X1, R10
    VPEXTRQ $0, X3, R11
    VPEXTRQ $1, X3, R12
    VCVTSI2SDQ R9, X5, X5
    VCVTSI2SDQ R10, X6, X6
    VCVTSI2SDQ R11, X7, X7
    VCVTSI2SDQ R12, X8, X8
    VPEXTRQ $0, X2, R9
    VPEXTRQ $1, X2, R10
    VPEXTRQ $0, X4, R11
    VPEXTRQ $1, X4, R12
    VPUNPCKLQDQ X6, X5, X5
    VPUNPCKLQDQ X8, X7, X7
    VCVTSI2SDQ R9, X9, X9
    VCVTSI2SDQ R10, X10, X10
    VCVTSI2SDQ R11, X11, X11
    VCVTSI2SDQ R12, X12, X12
    VPUNPCKLQDQ X10, X9, X9
    VPUNPCKLQDQ X12, X11, X11
    VINSERTF128 $1, X7, Y5, Y1
    VINSERTF128 $1, X11, Y9, Y2
    VDIVPD Y2, Y1, Y3
    VMOVUPD Y3, (BX)
    ADDQ $32, AX
    ADDQ $32, R8
    ADDQ $32, BX
    ADDQ $4, DI
    CMPQ DI, SI 
    JLT vecLoop

tradLoop:
    VCVTSI2SDQ (AX), X1, X1
    VCVTSI2SDQ (R8), X2, X2
    VDIVSD X2, X1, X3
    VMOVSD X3, (BX)
    ADDQ $8, AX
    ADDQ $8, R8
    ADDQ $8, BX
    ADDQ $1, DI
    CMPQ DI, CX
    JLT tradLoop

exitFn:
    RET

// // func mulI64VecI64Lit(src, dst []int64, lit int64)
// TEXT ·mulI64VecI64Lit(SB),NOSPLIT,$0-56

// // func mulI64VecI64Vec(src1, src2, dst []int64)
// TEXT ·mulI64VecI64Vec(SB),NOSPLIT,$0-72
