//go:build amd64

#include "textflag.h"

#define vSumOp(vVecOp, vTradOp, vReduce, dSize, chnkSize)  \
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
    VPXOR Y0, Y0, Y0                                       \
    VPXOR Y1, Y1, Y1                                       \
    VPXOR Y2, Y2, Y2                                       \
    VPXOR Y3, Y3, Y3                                       \
    VPXOR Y12, Y12, Y12                                    \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    vVecOp                                                 \
    ADDQ $128, AX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \
    VPXOR Y4, Y4, Y4                                       \
tradLoop:                                                  \
    vTradOp                                                \
    ADDQ dSize, AX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define I64SumVecLoopOps      \
        VPADDQ (AX), Y0, Y0   \
        VPADDQ 32(AX), Y1, Y1 \
        VPADDQ 64(AX), Y2, Y2 \
        VPADDQ 96(AX), Y3, Y3 

#define I64SumTradLoopOps \
        VMOVQ (AX), X4    \
        VPADDQ X4, X0, X0

#define I64SumReduceOps         \
        VPADDQ Y0, Y1, Y0       \
        VPADDQ Y2, Y3, Y2       \
        VPADDQ Y0, Y2, Y0       \
        VEXTRACTI128 $1, Y0, X1 \
        VPADDQ X0, X1, X0       \
        VPUNPCKHQDQ X1, X0, X1  \
        VPADDQ X1, X0, X0       \
        VMOVQ X0, (BX)

#define I32SumVecLoopOps       \
        VPMOVSXDQ (AX), Y4     \
        VPMOVSXDQ 16(AX), Y5   \
        VPMOVSXDQ 32(AX), Y6   \
        VPMOVSXDQ 48(AX), Y7   \
        VPADDQ Y4, Y5, Y4      \
        VPADDQ Y6, Y7, Y6      \
        VPMOVSXDQ 64(AX), Y8   \
        VPMOVSXDQ 80(AX), Y9   \
        VPMOVSXDQ 96(AX), Y10  \
        VPMOVSXDQ 112(AX), Y11 \
        VPADDQ Y8, Y9, Y8      \
        VPADDQ Y10, Y11, Y10   \
        VPADDQ Y4, Y0, Y0      \ 
        VPADDQ Y6, Y1, Y1      \
        VPADDQ Y8, Y2, Y2      \
        VPADDQ Y10, Y3, Y3

#define I32SumTradLoopOps \
        VMOVD (AX), X4    \
        VPMOVSXDQ X4, X4  \
        VPADDQ X4, X0, X0 

#define F64SumVecLoopOps           \
        VMOVUPD (AX), Y4           \   
        VMOVUPD 32(AX), Y5         \    
        VMOVUPD 64(AX), Y6         \
        VMOVUPD 96(AX), Y7         \
        VCMPPD $4, Y4, Y4, Y8      \
        VCMPPD $4, Y5, Y5, Y9      \
        VCMPPD $4, Y6, Y6, Y10     \
        VCMPPD $4, Y7, Y7, Y11     \
        VPBLENDVB Y8, Y12, Y4, Y4  \
        VPBLENDVB Y9, Y12, Y5, Y5  \
        VPBLENDVB Y10, Y12, Y6, Y6 \
        VPBLENDVB Y11, Y12, Y7, Y7 \
        VADDPD Y4, Y0, Y0          \
        VADDPD Y5, Y1, Y1          \
        VADDPD Y6, Y2, Y2          \
        VADDPD Y7, Y3, Y3 

#define F64SumTradLoopOps          \
        VMOVSD (AX), X4            \
        VCMPPD $4, X4, X4, X8      \
        VPBLENDVB X8, X12, X4, X4  \
        VADDSD X4, X0, X0

#define F64SumReduceOps         \
        VADDPD Y0, Y1, Y4       \
        VADDPD Y2, Y3, Y5       \
        VADDPD Y4, Y5, Y0       \
        VEXTRACTF128 $1, Y0, X1 \
        VADDPD X0, X1, X2       \
        VUNPCKHPD X1, X2, X1    \
        VADDPD X1, X2, X0       \
        VMOVSD X0, (BX)

#define F32SumVecLoopOps           \
        VCVTPS2PD (AX), Y4         \
        VCVTPS2PD 16(AX), Y5       \
        VCVTPS2PD 32(AX), Y6       \
        VCVTPS2PD 48(AX), Y7       \
        VCMPPD $4, Y4, Y4, Y8      \
        VCMPPD $4, Y5, Y5, Y9      \
        VCMPPD $4, Y6, Y6, Y10     \
        VCMPPD $4, Y7, Y7, Y11     \
        VPBLENDVB Y8, Y12, Y4, Y4  \
        VPBLENDVB Y9, Y12, Y5, Y5  \
        VPBLENDVB Y10, Y12, Y6, Y6 \
        VPBLENDVB Y11, Y12, Y7, Y7 \
        VADDPD Y4, Y0, Y0          \
        VADDPD Y5, Y1, Y1          \
        VADDPD Y6, Y2, Y2          \
        VADDPD Y7, Y3, Y3          \
        VCVTPS2PD 64(AX), Y4       \
        VCVTPS2PD 80(AX), Y5       \
        VCVTPS2PD 96(AX), Y6       \
        VCVTPS2PD 112(AX), Y7      \
        VCMPPD $4, Y4, Y4, Y8      \
        VCMPPD $4, Y5, Y5, Y9      \
        VCMPPD $4, Y6, Y6, Y10     \
        VCMPPD $4, Y7, Y7, Y11     \
        VPBLENDVB Y8, Y12, Y4, Y4  \
        VPBLENDVB Y9, Y12, Y5, Y5  \
        VPBLENDVB Y10, Y12, Y6, Y6 \
        VPBLENDVB Y11, Y12, Y7, Y7 \
        VADDPD Y4, Y0, Y0          \
        VADDPD Y5, Y1, Y1          \
        VADDPD Y6, Y2, Y2          \
        VADDPD Y7, Y3, Y3

#define F32SumTradLoopOps          \
        VCVTSS2SD (AX), X4, X4     \
        VCMPPD $4, X4, X4, X8      \
        VPBLENDVB X8, X12, X4, X4  \
        VADDSD X4, X0, X0    

// func sumI64(src, dst []int64)
TEXT ·sumI64(SB),NOSPLIT,$0-48
    vSumOp(I64SumVecLoopOps, I64SumTradLoopOps, I64SumReduceOps, $8, $16)

// func sumI32(src []int32, dst []int64)
TEXT ·sumI32(SB),NOSPLIT,$0-48
    vSumOp(I32SumVecLoopOps, I32SumTradLoopOps, I64SumReduceOps, $4, $32)

// func sumF64(src, dst []float64)
TEXT ·sumF64(SB),NOSPLIT,$0-48
    vSumOp(F64SumVecLoopOps, F64SumTradLoopOps, F64SumReduceOps, $8, $16)

// func sumF32(src []float32, dst []float64)
TEXT ·sumF32(SB),NOSPLIT,$0-48
    vSumOp(F32SumVecLoopOps, F32SumTradLoopOps, F64SumReduceOps, $4, $32)
