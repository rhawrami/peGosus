//go:build amd64

#include "textflag.h"

#define I64MaxV $0x7fffffffffffffff
#define I64MinV $0x8000000000000000

#define F64MaxV $0x7ff0000000000000
#define F64MinV $0xfff0000000000000

#define I32MaxV $0x7fffffff
#define I32MinV $0x80000000

#define F32MaxV $0x7f800000
#define F32MinV $0xff800000

#define vBoundsOp(bReg, tMovOp, initBrdCstOp, vBrdCstOp, vMovOp, vOp, vReduce, dSize, chnkSize, initVal) \
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
    tMovOp initVal, bReg                                   \
    initBrdCstOp bReg, Y0                                  \
    initBrdCstOp bReg, Y1                                  \
    initBrdCstOp bReg, Y2                                  \
    initBrdCstOp bReg, Y3                                  \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y4                                        \
    vMovOp 32(AX), Y5                                      \
    vMovOp 64(AX), Y6                                      \
    vMovOp 96(AX), Y7                                      \
    vOp Y0, Y4, Y0                                         \
    vOp Y1, Y5, Y1                                         \
    vOp Y2, Y6, Y2                                         \
    vOp Y3, Y7, Y3                                         \
    ADDQ $128, AX                                          \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    vBrdCstOp (AX), Y4                                     \
    vOp Y0, Y4, Y0                                         \
    ADDQ dSize, AX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define reduceOpX32(vOp)        \
        vOp Y0, Y1, Y0          \
        vOp Y2, Y3, Y2          \
        vOp Y0, Y2, Y0          \
        VEXTRACTI128 $1, Y0, X1 \
        vOp X1, X0, X0          \
        VUNPCKHPD X1, X0, X1    \
        vOp X1, X0, X0          \
        VMOVQ X0, R8            \
        SHRQ $32, R8            \
        VMOVD R8, X1            \
        vOp X1, X0, X0          \
        VMOVD X0, (BX)

#define reduceOpF64(vOp)        \
        vOp Y0, Y1, Y0          \
        vOp Y2, Y3, Y2          \
        vOp Y0, Y2, Y0          \
        VEXTRACTF128 $1, Y0, X1 \
        vOp X1, X0, X0          \
        VUNPCKHPD X1, X0, X1    \
        vOp X1, X0, X0          \
        VMOVSD X0, (BX)

// func maxI32(src, dst []int32)
TEXT ·maxI32(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVL, VPBROADCASTD, VPBROADCASTD, VMOVDQU, VPMAXSD, reduceOpX32(VPMAXSD), $4, $32, I32MinV)

// func minI32(src, dst []int32)
TEXT ·minI32(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVL, VPBROADCASTD, VPBROADCASTD, VMOVDQU, VPMINSD, reduceOpX32(VPMINSD), $4, $32, I32MaxV)

// func maxF64(src, dst []float64)
TEXT ·maxF64(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVQ, VPBROADCASTQ, VBROADCASTSD, VMOVUPD, VMAXPD, reduceOpF64(VMAXPD), $8, $16, F64MinV)

// func minF64(src, dst []float64)
TEXT ·minF64(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVQ, VPBROADCASTQ, VBROADCASTSD, VMOVUPD, VMINPD, reduceOpF64(VMINPD), $8, $16, F64MaxV)

// func maxF32(src, dst []float32)
TEXT ·maxF32(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVL, VPBROADCASTD, VBROADCASTSS, VMOVUPS, VMAXPS, reduceOpX32(VMAXPS), $4, $32, F32MinV)

// func minF32(src, dst []float32)
TEXT ·minF32(SB),NOSPLIT,$0-48
    vBoundsOp(R8, MOVL, VPBROADCASTD, VBROADCASTSS, VMOVUPS, VMINPS, reduceOpX32(VMINPS), $4, $32, F32MaxV)

#define vBoundsI64(vOrd1, vOrd2, vOrd3, vOrd4, vOrd5, vOrd6, vOrd7, vOrd8) \ 
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
    MOVQ (AX), R8                                          \ 
    VPBROADCASTQ R8, Y0                                    \ 
    VPBROADCASTQ R8, Y1                                    \  
    VPBROADCASTQ R8, Y2                                    \                                     
    VPBROADCASTQ R8, Y3                                    \ 
                                                           \
    CMPQ CX, $16                                           \ 
    JLT tradLoop                                           \ 
                                                           \
vecLoop:                                                   \ 
    VMOVDQU (AX), Y4                                       \ 
    VMOVDQU 32(AX), Y5                                     \ 
    VMOVDQU 64(AX), Y6                                     \  
    VMOVDQU 96(AX), Y7                                     \ 
    VPCMPGTQ vOrd1, Y8                                     \ 
    VPCMPGTQ vOrd2, Y9                                     \ 
    VPCMPGTQ vOrd3, Y10                                    \ 
    VPCMPGTQ vOrd4, Y11                                    \
    VPBLENDVB Y8, Y4, Y0, Y0                               \ 
    VPBLENDVB Y9, Y5, Y1, Y1                               \ 
    VPBLENDVB Y10, Y6, Y2, Y2                              \ 
    VPBLENDVB Y11, Y7, Y3, Y3                              \ 
    ADDQ $128, AX                                          \ 
    ADDQ $16, DI                                           \ 
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \ 
                                                           \
tradLoop:                                                  \ 
    VPBROADCASTQ (AX), Y4                                  \ 
    VPCMPGTQ vOrd1, Y8                                     \ 
    VPBLENDVB Y8, Y4, Y0, Y0                               \ 
    ADDQ $8, AX                                            \ 
    ADDQ $1, DI                                            \ 
    CMPQ DI, CX                                            \ 
    JLT tradLoop                                           \ 
                                                           \
    VPCMPGTQ vOrd5, Y4                                     \
    VPCMPGTQ vOrd6, Y5                                     \ 
    VPBLENDVB Y4, Y1, Y0, Y0                               \ 
    VPBLENDVB Y5, Y3, Y2, Y2                               \ 
    VPCMPGTQ vOrd7, Y4                                     \ 
    VPBLENDVB Y4, Y2, Y0, Y0                               \ 
    VEXTRACTI128 $1, Y0, X1                                \ 
    VPCMPGTQ vOrd8, X2                                     \ 
    VPBLENDVB X2, X1, X0, X0                               \ 
    VUNPCKHPD X1, X0, X1                                   \ 
    VPCMPGTQ vOrd8, X2                                     \ 
    VPBLENDVB X2, X1, X0, X0                               \ 
    VMOVQ X0, (BX)                                         \ 
exitFn:                                                    \ 
    RET 

#define ord1MaxI64 Y0, Y4
#define ord2MaxI64 Y1, Y5
#define ord3MaxI64 Y2, Y6
#define ord4MaxI64 Y3, Y7
#define ord5MaxI64 Y0, Y1
#define ord6MaxI64 Y2, Y3
#define ord7MaxI64 Y0, Y2
#define ord8MaxI64 X0, X1

#define ord1MinI64 Y4, Y0
#define ord2MinI64 Y5, Y1
#define ord3MinI64 Y6, Y2
#define ord4MinI64 Y7, Y3
#define ord5MinI64 Y1, Y0
#define ord6MinI64 Y3, Y2
#define ord7MinI64 Y2, Y0
#define ord8MinI64 X1, X0

// func maxI64(src, dst []int64)
TEXT ·maxI64(SB),NOSPLIT,$0-48
    vBoundsI64(ord1MaxI64, ord2MaxI64, ord3MaxI64, ord4MaxI64, ord5MaxI64, ord6MaxI64, ord7MaxI64, ord8MaxI64)

// func minI64(src, dst []int64)
TEXT ·minI64(SB),NOSPLIT,$0-48
    vBoundsI64(ord1MinI64, ord2MinI64, ord3MinI64, ord4MinI64, ord5MinI64, ord6MinI64, ord7MinI64, ord8MinI64)

#define vDoubleBoundsOp(bReg, tMovOp, initBrdCstOp, vBrdCstOp, vMovOp, vMinOp, vMaxOp, vReduce, dSize, chnkSize, initVal1, initVal2) \
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
    tMovOp initVal1, bReg                                  \
    initBrdCstOp bReg, Y0                                  \
    initBrdCstOp bReg, Y1                                  \
    tMovOp initVal2, bReg                                  \
    initBrdCstOp bReg, Y2                                  \
    initBrdCstOp bReg, Y3                                  \
                                                           \
    CMPQ CX, chnkSize                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y4                                        \
    vMovOp 32(AX), Y5                                      \
    vMinOp Y0, Y4, Y0                                      \
    vMinOp Y1, Y5, Y1                                      \
    vMaxOp Y2, Y4, Y2                                      \
    vMaxOp Y3, Y5, Y3                                      \
    ADDQ $64, AX                                           \
    ADDQ chnkSize, DI                                      \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    vBrdCstOp (AX), Y4                                     \
    vMinOp Y0, Y4, Y0                                      \
    vMaxOp Y2, Y4, Y2                                      \
    ADDQ dSize, AX                                         \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define mmReduceOpX32(vMinOp, vMaxOp) \
        vMinOp Y0, Y1, Y0             \
        vMaxOp Y2, Y3, Y2             \
        VEXTRACTI128 $1, Y0, X1       \
        VEXTRACTI128 $1, Y2, X3       \
        vMinOp X1, X0, X0             \
        vMaxOp X3, X2, X2             \
        VUNPCKHPD X1, X0, X1          \
        VUNPCKHPD X3, X2, X3          \
        vMinOp X1, X0, X0             \
        vMaxOp X3, X2, X2             \
        VMOVQ X0, R8                  \
        VMOVQ X2, R9                  \
        SHRQ $32, R8                  \
        SHRQ $32, R9                  \
        VMOVD R8, X1                  \
        VMOVD R9, X3                  \
        vMinOp X1, X0, X0             \
        vMaxOp X3, X2, X2             \
        VMOVD X0, (BX)                \
        VMOVD X2, 4(BX)

#define mmReduceOpF64(vMinOp, vMaxOp) \
        vMinOp Y0, Y1, Y0             \
        vMaxOp Y2, Y3, Y2             \
        VEXTRACTF128 $1, Y0, X1       \
        VEXTRACTF128 $1, Y2, X3       \
        vMinOp X1, X0, X0             \
        vMaxOp X3, X2, X2             \
        VUNPCKHPD X1, X0, X1          \
        VUNPCKHPD X3, X2, X3          \
        vMinOp X1, X0, X0             \
        vMaxOp X3, X2, X2             \
        VMOVSD X0, (BX)               \
        VMOVSD X2, 8(BX) 

// func minmaxI32(src, dst []int32)
TEXT ·minmaxI32(SB),NOSPLIT,$0-48
    vDoubleBoundsOp(R8, MOVL, VPBROADCASTD, VPBROADCASTD, VMOVDQU, VPMINSD, VPMAXSD, mmReduceOpX32(VPMINSD, VPMAXSD), $4, $16, I32MaxV, I32MinV)

// func minmaxF64(src, dst []float64)
TEXT ·minmaxF64(SB),NOSPLIT,$0-48
    vDoubleBoundsOp(R8, MOVQ, VPBROADCASTQ, VBROADCASTSD, VMOVUPD, VMINPD, VMAXPD, mmReduceOpF64(VMINPD, VMAXPD), $8, $8, F64MaxV, F64MinV)

// func minmaxF32(src, dst []float32)
TEXT ·minmaxF32(SB),NOSPLIT,$0-48
    vDoubleBoundsOp(R8, MOVL, VPBROADCASTD, VBROADCASTSS, VMOVUPS, VMINPS, VMAXPS, mmReduceOpX32(VMINPS, VMAXPS), $4, $16, F32MaxV, F32MinV)

// func minmaxI64(src, dst []int64)
TEXT ·minmaxI64(SB),NOSPLIT,$0-48
    MOVQ srcAddr+0(FP), AX 
    MOVQ dstAddr+24(FP), BX 
    MOVQ srcLen+8(FP), CX 
    MOVQ CX, SI 
    XORQ DI, DI 
    SUBQ $8, SI

    TESTQ CX, CX 
    JEQ exitFn 

    MOVQ (AX), R8 
    VPBROADCASTQ R8, Y0 
    VPBROADCASTQ R8, Y1  
    VPBROADCASTQ R8, Y2                                     
    VPBROADCASTQ R8, Y3 

    CMPQ CX, $8
    JLT tradLoop 

vecLoop: 
    VMOVDQU (AX), Y4 
    VMOVDQU 32(AX), Y5 
    VPCMPGTQ Y4, Y0, Y8 
    VPCMPGTQ Y5, Y1, Y9 
    VPCMPGTQ Y2, Y4, Y10 
    VPCMPGTQ Y3, Y5, Y11
    VPBLENDVB Y8, Y4, Y0, Y0 
    VPBLENDVB Y9, Y5, Y1, Y1 
    VPBLENDVB Y10, Y4, Y2, Y2 
    VPBLENDVB Y11, Y5, Y3, Y3 
    ADDQ $64, AX 
    ADDQ $8, DI 
    CMPQ DI, SI 
    JLT vecLoop 

tradLoop: 
    VPBROADCASTQ (AX), Y4 
    VPCMPGTQ Y4, Y0, Y8 
    VPCMPGTQ Y2, Y4, Y10 
    VPBLENDVB Y8, Y4, Y0, Y0 
    VPBLENDVB Y10, Y4, Y2, Y2 
    ADDQ $8, AX 
    ADDQ $1, DI 
    CMPQ DI, CX 
    JLT tradLoop 

    VPCMPGTQ Y1, Y0, Y4
    VPCMPGTQ Y2, Y3, Y5
    VPBLENDVB Y4, Y1, Y0, Y0 
    VPBLENDVB Y5, Y3, Y2, Y2 
    VEXTRACTI128 $1, Y0, X1 
    VEXTRACTI128 $1, Y2, X3 
    VPCMPGTQ X1, X0, X4
    VPCMPGTQ X2, X3, X5
    VPBLENDVB X4, X1, X0, X0 
    VPBLENDVB X5, X3, X2, X2 
    VUNPCKHPD X1, X0, X1 
    VUNPCKHPD X3, X2, X3 
    VPCMPGTQ X1, X0, X4
    VPCMPGTQ X2, X3, X5 
    VPBLENDVB X4, X1, X0, X0 
    VPBLENDVB X5, X3, X2, X2 
    VMOVQ X0, (BX) 
    VMOVQ X2, 8(BX) 
exitFn: 
    RET 

#define vBoundsWithValidityIF32(vBrdCstOp, vMovOp, vOp, vReduce, initVal) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ validityAddr+48(FP), R10                          \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    MOVQ $0x0000000200000001, R8                           \
    VMOVQ R8, X0                                           \
    MOVQ $0x0000000800000004, R8                           \
    VPINSRQ $1, R8, X0, X0                                 \
    MOVQ $0x0000002000000010, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000008000000040, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    VINSERTI128 $1, X1, Y0, Y0                             \
    MOVL initVal, R8                                       \
    VPBROADCASTD R8, Y1                                    \
    VPXOR Y6, Y6, Y6                                       \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y2                                        \
    VPBROADCASTB (R10), Y4                                 \
    vOp Y1, Y2, Y3                                         \
    VPAND Y4, Y0, Y5                                       \
    VPCMPGTD Y6, Y5, Y7                                    \
    VPBLENDVB Y7, Y3, Y1, Y1                               \
    ADDQ $32, AX                                           \
    ADDQ $1, R10                                           \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \ 
    XORQ R9, R9                                            \
    MOVB (R10), R9                                         \                                                        
tradLoop:                                                  \
    vBrdCstOp (AX), Y2                                     \
    vOp Y1, Y2, Y3                                         \
    MOVQ $1, R11                                           \
    ANDQ R9, R11                                           \
    XORQ R12, R12                                          \
    SUBQ R11, R12                                          \
    VPBROADCASTB R12, Y4                                   \
    VPBLENDVB Y4, Y3, Y1, Y1                               \
    SHRQ $1, R9                                            \
    ADDQ $4, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define reduceOpX32WithValidity(vOp) \
        VEXTRACTI128 $1, Y1, X2      \
        vOp X2, X1, X1               \
        VUNPCKHPD X1, X1, X2         \
        vOp X2, X1, X1               \
        VMOVQ X1, R8                 \
        SHRQ $32, R8                 \
        VMOVD R8, X2                 \
        vOp X2, X1, X1               \
        VMOVD X1, (BX)

// func maxI32WithValidity(src, dst []int32, validity []byte)
TEXT ·maxI32WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF32(VPBROADCASTD, VMOVDQU, VPMAXSD, reduceOpX32WithValidity(VPMAXSD), I32MinV)

// func minI32WithValidity(src, dst []int32, validity []byte)
TEXT ·minI32WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF32(VPBROADCASTD, VMOVDQU, VPMINSD, reduceOpX32WithValidity(VPMINSD), I32MaxV)

// func maxF32WithValidity(src, dst []float32, validity []byte)
TEXT ·maxF32WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF32(VBROADCASTSS, VMOVUPS, VMAXPS, reduceOpX32WithValidity(VMAXPS), F32MinV)

// func minF32WithValidity(src, dst []float32, validity []byte)
TEXT ·minF32WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF32(VBROADCASTSS, VMOVUPS, VMINPS, reduceOpX32WithValidity(VMINPS), F32MaxV)

#define vBoundsWithValidityIF64(vBrdCstOp, vMovOp, wBoundsOpVecLoop, wBoundsOpTradLoop, vReduce, initVal) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ validityAddr+48(FP), R10                          \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    MOVQ $0x0000000000000001, R8                           \
    VMOVQ R8, X0                                           \
    MOVQ $0x0000000000000002, R8                           \
    VPINSRQ $1, R8, X0, X0                                 \
    MOVQ $0x0000000000000004, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000000000000008, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    VINSERTI128 $1, X1, Y0, Y0                             \
    MOVQ $0x0000000000000010, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000000000000020, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    MOVQ $0x0000000000000040, R8                           \
    VMOVQ R8, X2                                           \
    MOVQ $0x0000000000000080, R8                           \
    VPINSRQ $1, R8, X2, X2                                 \
    VINSERTI128 $1, X2, Y1, Y1                             \
                                                           \
    MOVQ initVal, R8                                       \
    VPBROADCASTQ R8, Y2                                    \
    VPBROADCASTQ R8, Y3                                    \
    VPXOR Y10, Y10, Y10                                    \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y4                                        \
    vMovOp 32(AX), Y5                                      \
    wBoundsOpVecLoop                                       \
    VPBROADCASTB (R10), Y6                                 \
    VMOVDQA Y6, Y7                                         \
    VPAND Y6, Y0, Y6                                       \
    VPAND Y7, Y1, Y7                                       \
    VPCMPGTQ Y10, Y6, Y11                                  \
    VPCMPGTQ Y10, Y7, Y12                                  \
    VPBLENDVB Y11, Y8, Y2, Y2                              \
    VPBLENDVB Y12, Y9, Y3, Y3                              \
    ADDQ $64, AX                                           \
    ADDQ $1, R10                                           \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \ 
    XORQ R9, R9                                            \
    MOVB (R10), R9                                         \                                                        
tradLoop:                                                  \
    vBrdCstOp (AX), Y4                                     \
    wBoundsOpTradLoop                                      \
    MOVQ $1, R11                                           \
    ANDQ R9, R11                                           \
    XORQ R12, R12                                          \
    SUBQ R11, R12                                          \
    VPBROADCASTB R12, Y6                                   \
    VPBLENDVB Y6, Y8, Y2, Y2                               \
    SHRQ $1, R9                                            \
    ADDQ $8, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define I64MWithValidityVecLoopOps(vr1, vr2, vr3, vr4) \
        VPCMPGTQ vr1, vr2, Y6                          \
        VPCMPGTQ vr3, vr4, Y7                          \
        VPBLENDVB Y6, Y4, Y2, Y8                       \
        VPBLENDVB Y7, Y5, Y3, Y9                       

#define I64MWithValidityTradLoopOps(vr1, vr2) \
        VPCMPGTQ vr1, vr2, Y6                 \
        VPBLENDVB Y6, Y4, Y2, Y8              

#define I64MWithValidityReduceOps(vr1, vr2, vr3, vr4, vr5, vr6) \
        VPCMPGTQ vr1, vr2, Y0                                   \
        VPBLENDVB Y0, Y3, Y2, Y1                                \
        VEXTRACTI128 $1, Y1, X2                                 \
        VPCMPGTQ vr3, vr4, X0                                   \
        VPBLENDVB X0, X2, X1, X3                                \
        VUNPCKHPD X2, X3, X2                                    \
        VPCMPGTQ vr5, vr6, X0                                   \
        VPBLENDVB X0, X3, X2, X1                                \
        VMOVQ X1, (BX)

#define F64MWithValidityVecLoopOps(bOp) \
        bOp Y2, Y4, Y8                  \
        bOp Y3, Y5, Y9                  

#define F64MWithValidityTradLoopOps(bOp) \
        bOp Y2, Y4, Y8                   

#define F64MWithValidityReduceOps(bOp) \
        bOp Y2, Y3, Y0                 \
        VEXTRACTF128 $1, Y0, X1        \
        bOp X0, X1, X2                 \
        VUNPCKHPD X3, X2, X3           \
        bOp X3, X2, X0                 \
        VMOVSD X0, (BX)

// func maxI64WithValidity(src, dst []int64, validity []byte)
TEXT ·maxI64WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF64(VPBROADCASTQ, VMOVDQU, I64MWithValidityVecLoopOps(Y2, Y4, Y3, Y5), I64MWithValidityTradLoopOps(Y2, Y4), I64MWithValidityReduceOps(Y2, Y3, X1, X2, X2, X3), I64MinV)

// func minI64WithValidity(src, dst []int64, validity []byte)
TEXT ·minI64WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF64(VPBROADCASTQ, VMOVDQU, I64MWithValidityVecLoopOps(Y4, Y2, Y5, Y3), I64MWithValidityTradLoopOps(Y4, Y2), I64MWithValidityReduceOps(Y3, Y2, X2, X1, X3, X2), I64MaxV)

// func maxF64WithValidity(src, dst []float64, validity []byte)
TEXT ·maxF64WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF64(VBROADCASTSD, VMOVUPD, F64MWithValidityVecLoopOps(VMAXPD), F64MWithValidityTradLoopOps(VMAXPD), F64MWithValidityReduceOps(VMAXPD), F64MinV)    

// func minF64WithValidity(src, dst []float64, validity []byte)
TEXT ·minF64WithValidity(SB),NOSPLIT,$0-72
    vBoundsWithValidityIF64(VBROADCASTSD, VMOVUPD, F64MWithValidityVecLoopOps(VMINPD), F64MWithValidityTradLoopOps(VMINPD), F64MWithValidityReduceOps(VMINPD), F64MaxV)

#define vDoubleBoundsWithValidityIF32(vBrdCstOp, vMovOp, vMinOp, vMaxOp, vReduce, initVal1, initVal2) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ validityAddr+48(FP), R10                          \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    MOVQ $0x0000000200000001, R8                           \
    VMOVQ R8, X0                                           \
    MOVQ $0x0000000800000004, R8                           \
    VPINSRQ $1, R8, X0, X0                                 \
    MOVQ $0x0000002000000010, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000008000000040, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    VINSERTI128 $1, X1, Y0, Y0                             \
                                                           \
    MOVL initVal1, R8                                      \
    VPBROADCASTD R8, Y1                                    \
    MOVL initVal2, R8                                      \
    VPBROADCASTD R8, Y13                                   \
    VPXOR Y6, Y6, Y6                                       \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y2                                        \
    VPBROADCASTB (R10), Y4                                 \
    vMinOp Y1, Y2, Y3                                      \
    vMaxOp Y13, Y2, Y14                                    \
    VPAND Y4, Y0, Y5                                       \
    VPCMPGTD Y6, Y5, Y7                                    \
    VPBLENDVB Y7, Y3, Y1, Y1                               \
    VPBLENDVB Y7, Y14, Y13, Y13                            \
    ADDQ $32, AX                                           \
    ADDQ $1, R10                                           \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \ 
    XORQ R9, R9                                            \
    MOVB (R10), R9                                         \                                                        
tradLoop:                                                  \
    vBrdCstOp (AX), Y2                                     \
    vMinOp Y1, Y2, Y3                                      \
    vMaxOp Y13, Y2, Y14                                    \
    MOVQ $1, R11                                           \
    ANDQ R9, R11                                           \
    XORQ R12, R12                                          \
    SUBQ R11, R12                                          \
    VPBROADCASTB R12, Y4                                   \
    VPBLENDVB Y4, Y3, Y1, Y1                               \
    VPBLENDVB Y4, Y14, Y13, Y13                            \
    SHRQ $1, R9                                            \
    ADDQ $4, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define reduceMMOpX32WithValidity(vMinOp, vMaxOp) \
        VEXTRACTI128 $1, Y1, X0      \
        VEXTRACTI128 $1, Y13, X12    \
        vMinOp X1, X0, X0            \
        vMaxOp X13, X12, X12         \
        VUNPCKHPD X0, X0, X1         \
        VUNPCKHPD X12, X12, X13      \
        vMinOp X1, X0, X0            \
        vMaxOp X13, X12, X12         \
        VMOVQ X0, R8                 \
        VMOVQ X12, R9                \
        SHRQ $32, R8                 \
        SHRQ $32, R9                 \
        VMOVD R8, X1                 \
        VMOVD R9, X13                \
        vMinOp X1, X0, X0            \
        vMaxOp X13, X12, X12         \
        VMOVD X0, (BX)               \
        VMOVD X12, 4(BX)             \

// func minmaxI32WithValidity(src, dst []int32, validity []byte)
TEXT ·minmaxI32WithValidity(SB),NOSPLIT,$0-72
    vDoubleBoundsWithValidityIF32(VPBROADCASTD, VMOVDQU, VPMINSD, VPMAXSD, reduceMMOpX32WithValidity(VPMINSD, VPMAXSD), I32MaxV, I32MinV)

// func minmaxF32WithValidity(src, dst []float32, validity []byte)
TEXT ·minmaxF32WithValidity(SB),NOSPLIT,$0-72
    vDoubleBoundsWithValidityIF32(VBROADCASTSS, VMOVUPS, VMINPS, VMAXPS, reduceMMOpX32WithValidity(VMINPS, VMAXPS), F32MaxV, F32MinV)

#define vDoubleBoundsWithValidityIF64(vBrdCstOp, vMovOp, wBoundsOpVecLoop, wBoundsOpTradLoop, vReduce, initVal1, initVal2) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ validityAddr+48(FP), R10                          \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    MOVQ $0x0000000000000001, R8                           \
    VMOVQ R8, X0                                           \
    MOVQ $0x0000000000000002, R8                           \
    VPINSRQ $1, R8, X0, X0                                 \
    MOVQ $0x0000000000000004, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000000000000008, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    VINSERTI128 $1, X1, Y0, Y0                             \
    MOVQ $0x0000000000000010, R8                           \
    VMOVQ R8, X1                                           \
    MOVQ $0x0000000000000020, R8                           \
    VPINSRQ $1, R8, X1, X1                                 \
    MOVQ $0x0000000000000040, R8                           \
    VMOVQ R8, X2                                           \
    MOVQ $0x0000000000000080, R8                           \
    VPINSRQ $1, R8, X2, X2                                 \
    VINSERTI128 $1, X2, Y1, Y1                             \
                                                           \
    MOVQ initVal1, R8                                      \
    VPBROADCASTQ R8, Y2                                    \
    VPBROADCASTQ R8, Y3                                    \
    MOVQ initVal2, R8                                      \
    VPBROADCASTQ R8, Y12                                   \
    VPBROADCASTQ R8, Y13                                   \
    VPXOR Y10, Y10, Y10                                    \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y4                                        \
    vMovOp 32(AX), Y5                                      \
    VPBROADCASTB (R10), Y6                                 \
    VMOVDQA Y6, Y7                                         \
    VPAND Y6, Y0, Y6                                       \
    VPAND Y7, Y1, Y7                                       \
    VPCMPGTQ Y10, Y6, Y11                                  \
    VPCMPGTQ Y10, Y7, Y14                                  \
    wBoundsOpVecLoop                                       \
    VPBLENDVB Y11, Y6, Y2, Y2                              \
    VPBLENDVB Y11, Y7, Y12, Y12                            \
    VPBLENDVB Y14, Y8, Y3, Y3                              \
    VPBLENDVB Y14, Y9, Y13, Y13                            \
    ADDQ $64, AX                                           \
    ADDQ $1, R10                                           \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \ 
    XORQ R9, R9                                            \
    MOVB (R10), R9                                         \                                                        
tradLoop:                                                  \
    vBrdCstOp (AX), Y4                                     \
    wBoundsOpTradLoop                                      \
    MOVQ $1, R11                                           \
    ANDQ R9, R11                                           \
    XORQ R12, R12                                          \
    SUBQ R11, R12                                          \
    VPBROADCASTB R12, Y6                                   \
    VPBLENDVB Y6, Y8, Y2, Y2                               \
    VPBLENDVB Y6, Y9, Y12, Y12                             \
    SHRQ $1, R9                                            \
    ADDQ $8, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define I64MMWithValidityVecLoopOps \
        VPCMPGTQ Y4, Y2, Y6         \
        VPCMPGTQ Y12, Y4, Y7        \
        VPCMPGTQ Y5, Y3, Y8         \
        VPCMPGTQ Y13, Y5, Y9        \
        VPBLENDVB Y6, Y4, Y2, Y6    \
        VPBLENDVB Y7, Y4, Y12, Y7   \
        VPBLENDVB Y8, Y5, Y3, Y8    \
        VPBLENDVB Y9, Y5, Y13, Y9

#define I64MMWithValidityTradLoopOps \
        VPCMPGTQ Y4, Y2, Y6          \
        VPCMPGTQ Y12, Y4, Y7         \
        VPBLENDVB Y6, Y4, Y2, Y8     \
        VPBLENDVB Y7, Y4, Y12, Y9

#define I64MMWithValidityReduceOps     \
        VPCMPGTQ Y3, Y2, Y1            \
        VPCMPGTQ Y12, Y13, Y11         \
        VPBLENDVB Y1, Y3, Y2, Y2       \
        VPBLENDVB Y11, Y13, Y12, Y12   \
        VEXTRACTI128 $1, Y2, X3        \
        VEXTRACTI128 $1, Y12, X13      \
        VPCMPGTQ X3, X2, X1            \
        VPCMPGTQ X12, X13, X11         \
        VPBLENDVB X1, X3, X2, X2       \
        VPBLENDVB X11, X13, X12, X12   \
        VUNPCKHPD X3, X2, X3           \
        VUNPCKHPD X13, X12, X13        \
        VPCMPGTQ X3, X2, X1            \
        VPCMPGTQ X12, X13, X11         \
        VPBLENDVB X1, X3, X2, X2       \
        VPBLENDVB X11, X13, X12, X12   \
        VMOVQ X2, (BX)                 \
        VMOVQ X12, 8(BX)

#define F64MMWithValidityVecLoopOps \
        VMINPD Y2, Y4, Y6           \
        VMAXPD Y12, Y4, Y7          \
        VMINPD Y3, Y5, Y8           \
        VMAXPD Y13, Y5, Y9     

#define F64MMWithValidityTradLoopOps \
        VMINPD Y2, Y4, Y8            \
        VMAXPD Y12, Y4, Y9     

#define F64MMWithValidityReduceOps \
        VMINPD Y2, Y3, Y2          \
        VMAXPD Y12, Y13, Y12       \
        VEXTRACTF128 $1, Y2, X3    \
        VEXTRACTF128 $1, Y12, X13  \
        VMINPD X2, X3, X2          \
        VMAXPD X12, X13, X12       \
        VUNPCKHPD X3, X2, X3       \
        VUNPCKHPD X13, X12, X13    \
        VMINPD X2, X3, X2          \
        VMAXPD X12, X13, X12       \
        VMOVSD X2, (BX)            \
        VMOVSD X12, 8(BX)

// func minmaxF64WithValidity(src, dst []float64, validity []byte)
TEXT ·minmaxF64WithValidity(SB),NOSPLIT,$0-72
    vDoubleBoundsWithValidityIF64(VBROADCASTSD, VMOVUPD, F64MMWithValidityVecLoopOps, F64MMWithValidityTradLoopOps, F64MMWithValidityReduceOps, F64MaxV, F64MinV)    

// func minmaxI64WithValidity(src, dst []int64, validity []byte)
TEXT ·minmaxI64WithValidity(SB),NOSPLIT,$0-72
    vDoubleBoundsWithValidityIF64(VPBROADCASTQ, VMOVDQU, I64MMWithValidityVecLoopOps, I64MMWithValidityTradLoopOps, I64MMWithValidityReduceOps, I64MaxV, I64MinV)    
