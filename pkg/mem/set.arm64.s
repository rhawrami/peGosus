//go:build arm64

#include "textflag.h"

#define vSet(mOp, dSize, chnkSize, spec)                   \
    MOVD dstAddr+0(FP), R0                                 \
    MOVD dstLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+16(FP), R5                                     \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLE tradLoop                                           \
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

// func setU8(a *byte, l uint64, v byte)
TEXT ·setU8(SB),NOSPLIT,$0-17
    vSet(MOVB, $1, $64, B16)

// func setU32(a *byte, l uint64, v uint32)
TEXT ·setU32(SB),NOSPLIT,$0-20
    vSet(MOVW, $4, $16, S4)

// func setU64(a *byte, l uint64, v uint64)
TEXT ·setU64(SB),NOSPLIT,$0-24
    vSet(MOVD, $8, $8, D2)