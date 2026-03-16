//go:build amd64

#include "textflag.h"

#define vCmpOneLit(vBrdCstOp, vMovOp, vCmpOp)              \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
    vBrdCstOp lit+48(FP), Y0                               \
    VPBROADCASTB $255, Y5                                  \
    XORQ R8, R8                                            \ 
    MOVL $1, R9                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vCmpOp                                                 \
    VMOVSKPS Y2, R9                                        \
    MOVB R9, (BX)                                          \
    ADDQ $32, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVD (AX), Y1                                         \
    vCmpOp                                                 \
    VMOVD Y2, R10                                          \
    ANDQ R9, R10                                           \
    ORQ R10, R8                                            \
    SHL $1, R8                                             \
    ADDQ $4, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    MOVB R8, (BX)                                          \ 
exitFn:                                                    \
    RET

#define GTI32 VPCMPGTD Y0, Y1, Y2
#define LTI32 VPCMPGTD Y1, Y0, Y2
#define EQI32 VPCMPEQD Y0, Y1, Y2
#define GEI32 \
        VPCMPGTD Y0, Y1, Y3 \
        VPCMPEQD Y0, Y1, Y4 \
        VPOR Y3, Y4, Y2
#define LEI32 \
        VPCMPGTD Y1, Y0, Y3 \
        VPCMPEQD Y0, Y1, Y4 \
        VPOR Y3, Y4, Y2
#define NEQI32 \
        VPCMPEQD Y0, Y1, Y3 \
        VPXOR Y5, Y3, Y2

#define GTF32 VCMPPS $1 Y1, Y0, Y2
#define LTF32 VCMPPS $1 Y0, Y1, Y2
#define EQF32 VCMPPS $0 Y0, Y1, Y2
#define GEF32 VCMPPS $2 Y1, Y0, Y2
#define LEF32 VCMPPS $2 Y0, Y1, Y2
#define NEQF32 VCMPPS $4 Y0, Y1, Y2

// func cmpGtI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpGtI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, GTI32)

// func cmpLtI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpLtI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, LTI32)

// func cmpGeI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpGeI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, GEI32)

// func cmpLeI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpLeI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, LEI32)

// func cmpEqI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpEqI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, EQI32)

// func cmpNeqI32VecI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpNeqI32VecI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVD, NEQI32)

// func cmpGtF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpGtF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, GTF32)

// func cmpLtF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpLtF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, LTF32)

// func cmpGeF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpGeF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, GEF32)

// func cmpLeF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpLeF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, LEF32)

// func cmpEqF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpEqF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, EQF32)
    
// func cmpNeqF32VecF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpNeqF32VecF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVSS, NEQF32)

#define vCmpTwoLit(vBrdCstOp, vMovOp, vCmpOp)              \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
    vBrdCstOp min+48(FP), Y0                               \
    vBrdCstOp max+52(FP), Y1                               \
    XORQ R8, R8                                            \ 
    MOVL $1, R9                                            \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $8                                            \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y2                                        \
    vCmpOp                                                 \
    VMOVSKPS Y3, R9                                        \
    MOVB R9, (BX)                                          \
    ADDQ $32, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVD (AX), Y2                                         \
    vCmpOp                                                 \
    VMOVD Y3, R10                                          \
    ANDQ R9, R10                                           \
    ORQ R10, R8                                            \
    SHL $1, R8                                             \
    ADDQ $4, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    MOVB R8, (BX)                                          \ 
exitFn:                                                    \
    RET

#define BETI32 \
        VPCMPGTD Y0, Y2, Y4 \
        VPCMPEQD Y0, Y2, Y5 \
        VPOR Y4, Y5, Y6     \
        VPCMPGTD Y2, Y1, Y7 \
        VPCMPEQD Y1, Y2, Y8 \
        VPOR Y7, Y8, Y9     \
        VPAND Y6, Y9, Y3
#define NBETI32 \
        VPCMPGTD Y2, Y0, Y4 \
        VPCMPGTD Y1, Y2, Y5 \
        VPOR Y4, Y5, Y3

#define BETF32 \
        VCMPPS $2 Y2, Y0, Y4 \
        VCMPPS $2 Y1, Y2, Y5 \
        VPAND Y4, Y5, Y3

#define NBETF32 \
        VCMPPS $1 Y0, Y2, Y4 \
        VCMPPS $1 Y2, Y1, Y5 \
        VPOR Y4, Y5, Y3

// func cmpBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)
TEXT ·cmpBetI32VecI32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VPBROADCASTD, VMOVD, BETI32)

// func cmpNBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)
TEXT ·cmpNBetI32VecI32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VPBROADCASTD, VMOVD, NBETI32)

// func cmpBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)
TEXT ·cmpBetF32VecF32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VBROADCASTSS, VMOVSS, BETF32)

// func cmpNBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)
TEXT ·cmpNBetF32VecF32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VBROADCASTSS, VMOVSS, NBETF32)
