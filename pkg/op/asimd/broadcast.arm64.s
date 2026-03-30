//go:build arm64

#include "textflag.h"

#define vBCast(mOp, dSize, chnkSize, spec)                 \
    MOVD dstAddr+0(FP), R0                                 \
    MOVD dstLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+24(FP), R5                                     \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VDUP R5, V1.spec                                       \
    VDUP R5, V2.spec                                       \
    VDUP R5, V3.spec                                       \
    VDUP R5, V4.spec                                       \
    VST1.P [V1.spec, V2.spec, V3.spec, V4.spec], 64(R0)    \  
                                                           \
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    mOp R5, (R0)                                           \
    ADD dSize, R0                                          \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET     

// func BroadcastU8(dst []byte, lit byte)
TEXT ·BroadcastU8(SB),NOSPLIT,$0-25
    vBCast(MOVB, $1, $64, B16)

// func BroadcastI64(dst []int64, lit int64)
TEXT ·BroadcastI64(SB),NOSPLIT,$0-32
    vBCast(MOVD, $8, $8, D2)

// func BroadcastI32(dst []int32, lit int32)
TEXT ·BroadcastI32(SB),NOSPLIT,$0-28
    vBCast(MOVW, $4, $16, S4)

// func BroadcastF64(dst []float64, lit float64)
TEXT ·BroadcastF64(SB),NOSPLIT,$0-32
    vBCast(MOVD, $8, $8, D2)

// func BroadcastF32(dst []float32, lit float32)
TEXT ·BroadcastF32(SB),NOSPLIT,$0-28
    vBCast(MOVW, $4, $16, S4)
