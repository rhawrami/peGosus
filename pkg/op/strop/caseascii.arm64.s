//go:build arm64

#include "textflag.h"

#define ASCII_DIFF $32
#define ASCII_LC_MIN $97
#define ASCII_LC_MAX $122
#define ASCII_UC_MIN $65
#define ASCII_UC_MAX $90

#define vCaseASCII(ascii_min, ascii_max, vDiffOp)          \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB $64, R2, R4                                        \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    VMOVI ascii_min, V0.B16                                \
    VMOVI ascii_max, V1.B16                                \
    VMOVI ASCII_DIFF, V2.B16                               \
                                                           \
    CMP $64, R2                                            \
    BLE tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V3.B16, V4.B16, V5.B16, V6.B16]        \
                                                           \
    WORD $0x4e203c67                                       \ // 'cmge.16b v7, v3, v0'
    WORD $0x4e203c88                                       \ // 'cmge.16b v8, v4, v0'
    WORD $0x4e203ca9                                       \ // 'cmge.16b v9, v5, v0'
    WORD $0x4e203cca                                       \ // 'cmge.16b v10, v6, v0'
    WORD $0x4e233c2b                                       \ // 'cmle.16b v11, v3, v1'
    WORD $0x4e243c2c                                       \ // 'cmle.16b v12, v4, v1'
    WORD $0x4e253c2d                                       \ // 'cmle.16b v13, v5, v1'
    WORD $0x4e263c2e                                       \ // 'cmle.16b v14, v6, v1'
    VAND V11.B16, V7.B16, V7.B16                           \
    VAND V12.B16, V8.B16, V8.B16                           \
    VAND V13.B16, V9.B16, V9.B16                           \
    VAND V14.B16, V10.B16, V10.B16                         \
    VAND V2.B16, V7.B16, V7.B16                            \
    VAND V2.B16, V8.B16, V8.B16                            \
    VAND V2.B16, V9.B16, V9.B16                            \
    VAND V2.B16, V10.B16, V10.B16                          \
    vDiffOp V7.B16, V3.B16, V3.B16                         \
    vDiffOp V8.B16, V4.B16, V4.B16                         \
    vDiffOp V9.B16, V5.B16, V5.B16                         \
    vDiffOp V10.B16, V6.B16, V6.B16                        \  
                                                           \
    VST1.P [V3.B16, V4.B16, V5.B16, V6.B16], 64(R1)        \  
    ADD $64, R3, R3                                        \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V3.B[0]                                     \
                                                           \
    WORD $0x4e203c67                                       \ // 'cmge.16b v7, v3, v0'
    WORD $0x4e233c2b                                       \ // 'cmle.16b v11, v3, v1'
    VAND V11.B16, V7.B16, V7.B16                           \
    VAND V2.B16, V7.B16, V7.B16                            \
    vDiffOp V7.B16, V3.B16, V3.B16                         \
                                                           \
    VST1 V3.B[0], (R1)                                     \
    ADD $1, R0, R0                                         \
    ADD $1, R1, R1                                         \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \  
    RET

// func ToUpperASCII(src []byte, dst []byte)
TEXT ·ToUpperASCII(SB),NOSPLIT,$0-48
    vCaseASCII(ASCII_LC_MIN, ASCII_LC_MAX, VSUB)

// func ToLowerASCII(src []byte, dst []byte)
TEXT ·ToLowerASCII(SB),NOSPLIT,$0-48
    vCaseASCII(ASCII_UC_MIN, ASCII_UC_MAX, VADD)
