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
    MOVB $0xFF, R8                                         \
    VPBROADCASTB R8, Y5                                    \
    XORQ R8, R8                                            \ 
    MOVQ $1, R11                                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $8                                            \
    JLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vCmpOp                                                 \
    VMOVMSKPS Y2, R9                                       \
    MOVB R9, (BX)                                          \
    ADDQ $32, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVD (AX), X1                                         \
    vCmpOp                                                 \
    VMOVD X2, R10                                          \
    ANDQ R11, R10                                          \
    ORQ R10, R8                                            \
    SHLQ $1, R11                                           \
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

#define GTF32 VCMPPS $1, Y1, Y0, Y2
#define LTF32 VCMPPS $1, Y0, Y1, Y2
#define EQF32 VCMPPS $0, Y0, Y1, Y2
#define GEF32 VCMPPS $2, Y1, Y0, Y2
#define LEF32 VCMPPS $2, Y0, Y1, Y2
#define NEQF32 VCMPPS $4, Y0, Y1, Y2

// func cmpGtI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpGtI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, GTI32)

// func cmpLtI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpLtI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, LTI32)

// func cmpGeI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpGeI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, GEI32)

// func cmpLeI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpLeI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, LEI32)

// func cmpEqI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpEqI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, EQI32)

// func cmpNeqI32Lit(src []int32, dst []byte, lit int32)
TEXT ·cmpNeqI32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VPBROADCASTD, VMOVDQU, NEQI32)

// func cmpGtF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpGtF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, GTF32)

// func cmpLtF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpLtF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, LTF32)

// func cmpGeF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpGeF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, GEF32)

// func cmpLeF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpLeF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, LEF32)

// func cmpEqF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpEqF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, EQF32)
    
// func cmpNeqF32Lit(src []float32, dst []byte, lit float32)
TEXT ·cmpNeqF32Lit(SB),NOSPLIT,$0-52
    vCmpOneLit(VBROADCASTSS, VMOVUPS, NEQF32)

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
    MOVL $1, R11                                           \
                                                           \
    TESTQ CX, CX                                           \
    JEQ exitFn                                             \
                                                           \
    CMPQ CX, $8                                            \
    JLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y2                                        \
    vCmpOp                                                 \
    VMOVMSKPS Y3, R9                                       \
    MOVB R9, (BX)                                          \
    ADDQ $32, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVD (AX), X2                                         \
    vCmpOp                                                 \
    VMOVD X3, R10                                          \
    ANDQ R11, R10                                          \
    ORQ R10, R8                                            \
    SHLQ $1, R11                                           \
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
        VCMPPS $2, Y2, Y0, Y4 \
        VCMPPS $2, Y1, Y2, Y5 \
        VPAND Y4, Y5, Y3

#define NBETF32 \
        VCMPPS $1, Y0, Y2, Y4 \
        VCMPPS $1, Y2, Y1, Y5 \
        VPOR Y4, Y5, Y3

// func cmpBetI32Lit(src []int32, dst []byte, min int32, max int32)
TEXT ·cmpBetI32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VPBROADCASTD, VMOVDQU, BETI32)

// func cmpNBetI32Lit(src []int32, dst []byte, min int32, max int32)
TEXT ·cmpNBetI32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VPBROADCASTD, VMOVDQU, NBETI32)

// func cmpBetF32Lit(src []float32, dst []byte, min float32, max float32)
TEXT ·cmpBetF32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VBROADCASTSS, VMOVUPS, BETF32)

// func cmpNBetF32Lit(src []float32, dst []byte, min float32, max float32)
TEXT ·cmpNBetF32Lit(SB),NOSPLIT,$0-56
    vCmpTwoLit(VBROADCASTSS, VMOVUPS, NBETF32)
