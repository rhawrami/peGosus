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
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vMovOp (AX), Y1                                        \
    vCmpOp                                                 \
    VMOVMSKPD Y2, R9                                       \
    vMovOp 32(AX), Y1                                      \
    vCmpOp                                                 \
    VMOVMSKPD Y2, R10                                      \
    SHLQ $4, R10                                           \
    ORQ R10, R9                                            \
    MOVB R9, (BX)                                          \
    ADDQ $64, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVQ (AX), X1                                         \
    vCmpOp                                                 \
    VMOVQ X2, R10                                          \
    ANDQ R11, R10                                          \
    ORQ R10, R8                                            \
    SHLQ $1, R11                                           \
    ADDQ $8, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    MOVB R8, (BX)                                          \ 
exitFn:                                                    \
    RET

#define GTI64 VPCMPGTQ Y0, Y1, Y2
#define LTI64 VPCMPGTQ Y1, Y0, Y2
#define EQI64 VPCMPEQQ Y0, Y1, Y2
#define GEI64 \
        VPCMPGTQ Y0, Y1, Y3 \
        VPCMPEQQ Y0, Y1, Y4 \
        VPOR Y3, Y4, Y2
#define LEI64 \
        VPCMPGTQ Y1, Y0, Y3 \
        VPCMPEQQ Y0, Y1, Y4 \
        VPOR Y3, Y4, Y2
#define NEQI64 \
        VPCMPEQQ Y0, Y1, Y3 \
        VPXOR Y5, Y3, Y2

#define GTF64 VCMPPD $1, Y1, Y0, Y2
#define LTF64 VCMPPD $1, Y0, Y1, Y2
#define EQF64 VCMPPD $0, Y0, Y1, Y2
#define GEF64 VCMPPD $2, Y1, Y0, Y2
#define LEF64 VCMPPD $2, Y0, Y1, Y2
#define NEQF64 VCMPPD $4, Y0, Y1, Y2

// func cmpGtI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpGtI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, GTI64)

// func cmpLtI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpLtI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, LTI64)

// func cmpGeI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpGeI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, GEI64)

// func cmpLeI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpLeI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, LEI64)

// func cmpEqI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpEqI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, EQI64)

// func cmpNeqI64Lit(src []int64, dst []byte, lit int64)
TEXT ·cmpNeqI64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VPBROADCASTQ, VMOVDQU, NEQI64)

// func cmpGtF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpGtF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, GTF64)

// func cmpLtF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpLtF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, LTF64)

// func cmpGeF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpGeF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, GEF64)

// func cmpLeF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpLeF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, LEF64)

// func cmpEqF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpEqF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, EQF64)
    
// func cmpNeqF64Lit(src []float64, dst []byte, lit float64)
TEXT ·cmpNeqF64Lit(SB),NOSPLIT,$0-56
    vCmpOneLit(VBROADCASTSD, VMOVUPD, NEQF64)

#define vCmpTwoLit(vBrdCstOp, vMovOp, vCmpOp)              \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    MOVQ CX, SI                                            \
    XORQ DI, DI                                            \
    SUBQ $8, SI                                            \
    vBrdCstOp min+48(FP), Y0                               \
    vBrdCstOp max+56(FP), Y1                               \
    XORQ R8, R8                                            \ 
    MOVL $1, R11                                           \
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
    VMOVMSKPD Y3, R9                                       \
    vMovOp 32(AX), Y2                                      \
    vCmpOp                                                 \
    VMOVMSKPD Y3, R10                                      \
    SHLQ $4, R10                                           \
    ORQ R10, R9                                            \
    MOVB R9, (BX)                                          \
    ADDQ $64, AX                                           \
    ADDQ $1, BX                                            \
    ADDQ $8, DI                                            \
    CMPQ DI, SI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VMOVQ (AX), X2                                         \
    vCmpOp                                                 \
    VMOVQ X3, R10                                          \
    ANDQ R11, R10                                          \
    ORQ R10, R8                                            \
    SHLQ $1, R11                                           \
    ADDQ $8, AX                                            \
    ADDQ $1, DI                                            \
    CMPQ DI, CX                                            \
    JLT tradLoop                                           \
                                                           \
    MOVB R8, (BX)                                          \ 
exitFn:                                                    \
    RET

#define BETI64 \
        VPCMPGTQ Y0, Y2, Y4 \
        VPCMPEQQ Y0, Y2, Y5 \
        VPOR Y4, Y5, Y6     \
        VPCMPGTQ Y2, Y1, Y7 \
        VPCMPEQQ Y1, Y2, Y8 \
        VPOR Y7, Y8, Y9     \
        VPAND Y6, Y9, Y3
#define NBETI64 \
        VPCMPGTQ Y2, Y0, Y4 \
        VPCMPGTQ Y1, Y2, Y5 \
        VPOR Y4, Y5, Y3

#define BETF64 \
        VCMPPD $2, Y2, Y0, Y4 \
        VCMPPD $2, Y1, Y2, Y5 \
        VPAND Y4, Y5, Y3

#define NBETF64 \
        VCMPPD $1, Y0, Y2, Y4 \
        VCMPPD $1, Y2, Y1, Y5 \
        VPOR Y4, Y5, Y3

// func cmpBetI64Lit(src []int64, dst []byte, min int64, max int64)
TEXT ·cmpBetI64Lit(SB),NOSPLIT,$0-64
    vCmpTwoLit(VPBROADCASTQ, VMOVDQU, BETI64)

// func cmpNBetI64Lit(src []int64, dst []byte, min int64, max int64)
TEXT ·cmpNBetI64Lit(SB),NOSPLIT,$0-64
    vCmpTwoLit(VPBROADCASTQ, VMOVDQU, NBETI64)

// func cmpBetF64Lit(src []float64, dst []byte, min float64, max float64)
TEXT ·cmpBetF64Lit(SB),NOSPLIT,$0-64
    vCmpTwoLit(VBROADCASTSD, VMOVUPD, BETF64)

// func cmpNBetF64Lit(src []float64, dst []byte, min float64, max float64)
TEXT ·cmpNBetF64Lit(SB),NOSPLIT,$0-64
    vCmpTwoLit(VBROADCASTSD, VMOVUPD, NBETF64)
