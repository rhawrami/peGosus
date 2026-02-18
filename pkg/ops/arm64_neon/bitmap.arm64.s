//go:build arm64

#include "textflag.h"

#define combineAndReturnSum(vOp)                           \
    MOVD src1Addr+0(FP), R0                                \  
    MOVD src2Addr+24(FP), R1                               \  
    MOVD dstAddr+48(FP), R2                                \  
    MOVD src1Len+8(FP), R3                                 \  
    EOR R4, R4                                             \
    SUB $64, R3, R5                                        \
    EOR R6, R6                                             \
    EOR R8, R8                                             \
    VEOR V4.B16, V4.B16, V4.B16                            \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \ 
                                                           \
    CMP $64, R3                                            \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.B16, V2.B16, V3.B16, V4.B16]        \
    VLD1.P 64(R1), [V5.B16, V6.B16, V7.B16, V8.B16]        \
                                                           \
    vOp V1.B16, V5.B16, V9.B16                             \
    vOp V2.B16, V6.B16, V10.B16                            \
    vOp V3.B16, V7.B16, V11.B16                            \
    vOp V4.B16, V8.B16, V12.B16                            \
                                                           \
    VCNT V9.B16, V13.B16                                   \
    VCNT V10.B16, V14.B16                                  \
    VCNT V11.B16, V15.B16                                  \
    VCNT V12.B16, V16.B16                                  \
                                                           \
    VADD V13.B16, V14.B16, V17.B16                         \
    VADD V15.B16, V16.B16, V18.B16                         \
    VADD V17.B16, V18.B16, V19.B16                         \
    VUADDLV V19.B16, V20                                   \
    VMOV V20.D[0], R7                                      \
    ADD R7, R6, R6                                         \
                                                           \
                                                           \
    VST1.P [V9.B16, V10.B16, V11.B16, V12.B16], 64(R2)     \
    ADD $64, R4                                            \
    CMP R5, R4                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1.P 1(R0), V1.B[0]                                  \
    VLD1.P 1(R1), V2.B[0]                                  \
                                                           \
    vOp V1.B16, V2.B16, V3.B16                             \
    VCNT V3.B16, V4.B16                                    \
    VMOV V4.B[0], R8                                       \
    ADD R8, R6, R6                                         \
                                                           \
    VST1.P V3.B[0], 1(R2)                                  \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    MOVD R6, sum+72(FP)                                    \
    RET

// func bitmapANDRetPopCount(src1, src2, dst []byte) uint64
TEXT ·bitmapANDRetPopCount(SB),NOSPLIT,$0-80
    combineAndReturnSum(VAND)

// func bitmapORRetPopCount(src1, src2, dst []byte) uint64
TEXT ·bitmapORRetPopCount(SB),NOSPLIT,$0-80
    combineAndReturnSum(VORR)

// func bitmapPopCount(src []byte) uint64
TEXT ·bitmapPopCount(SB),NOSPLIT,$0-32
    MOVD srcAddr+0(FP), R0
    MOVD srcLen+8(FP), R1
    EOR R2, R2
    EOR R5, R5
    EOR R6, R6
    SUB $64, R1, R3
    
    CMP $0, R1
    BEQ exitFn

    CMP $64, R1
    BLT tradLoop

vecLoop:
    VLD1.P 64(R0), [V1.B16, V2.B16, V3.B16, V4.B16]

    VCNT V1.B16, V5.B16
    VCNT V2.B16, V6.B16
    VCNT V3.B16, V7.B16
    VCNT V4.B16, V8.B16
    
    VADD V5.B16, V6.B16, V9.B16
    VADD V7.B16, V8.B16, V10.B16
    VADD V9.B16, V10.B16, V11.B16
    VUADDLV V11.B16, V12
    VMOV V12.D[0], R4
    ADD R5, R4, R4

    ADD $64, R2
    CMP R3, R2
    BLT vecLoop

tradLoop:
    VLD1.P 1(R0), V1.B[0]
    VCNT V1.B16, V2.B16
    VMOV V2.B[0], R6
    ADD R6, R4, R4

    ADD $1, R2
    CMP R1, R2
    BLT tradLoop

exitFn:
    MOVD R4, sum+24(FP)
    RET
