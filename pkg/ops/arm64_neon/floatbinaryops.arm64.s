//go:build arm64

#include "textflag.h"

#define vOpLitFloat(w1, w2, w3, w4, mOp, dSize, chnkSize, spec, spec1) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    mOp lit+48(FP), R5                                     \
    VDUP R5, V0.spec                                       \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V5.spec, V6.spec, V7.spec, V8.spec], 64(R1)    \  
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    VST1 V5.spec1[0], (R1)                                 \
    ADD dSize, R0, R0                                      \
    ADD dSize, R1, R1                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addF64VecF64Lit(src, dst []float64, lit float64)
// w1: $0x4e60d425 => 'fadd.2d	v5, v1, v0'
// w2: $0x4e60d446 => 'fadd.2d	v6, v2, v0'
// w3: $0x4e60d467 => 'fadd.2d	v7, v3, v0'
// w4: $0x4e60d488 => 'fadd.2d	v8, v4, v0'
TEXT ·addF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat($0x4e60d425, $0x4e60d446, $0x4e60d467, $0x4e60d488, MOVD, $8, $8, D2, D)

// func addF32VecF32Lit(src, dst []float32, lit float32)
// w1: $0x4e20d425 => 'fadd.4s	v5, v1, v0'
// w2: $0x4e20d446 => 'fadd.4s	v6, v2, v0'
// w3: $0x4e20d467 => 'fadd.4s	v7, v3, v0'
// w4: $0x4e20d488 => 'fadd.4s	v8, v4, v0'
TEXT ·addF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat($0x4e20d425, $0x4e20d446, $0x4e20d467, $0x4e20d488, MOVW, $4, $16, S4, S)

// func addF64VecF64Lit(src, dst []float64, lit float64)
// w1: $0x4ee0d425 => 'fsub.2d	v5, v1, v0'
// w2: $0x4ee0d446 => 'fsub.2d	v6, v2, v0'
// w3: $0x4ee0d467 => 'fsub.2d	v7, v3, v0'
// w4: $0x4ee0d488 => 'fsub.2d	v8, v4, v0'
TEXT ·subF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat($0x4ee0d425, $0x4ee0d446, $0x4ee0d467, $0x4ee0d488, MOVD, $8, $8, D2, D)

// func subF32VecF32Lit(src, dst []float32, lit float32)
// w1: $0x4ea0d425 => 'fsub.4s	v5, v1, v0'
// w2: $0x4ea0d446 => 'fsub.4s	v6, v2, v0'
// w3: $0x4ea0d467 => 'fsub.4s	v7, v3, v0'
// w4: $0x4ea0d488 => 'fsub.4s	v8, v4, v0'
TEXT ·subF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat($0x4ea0d425, $0x4ea0d446, $0x4ea0d467, $0x4ea0d488, MOVW, $4, $16, S4, S)

// func mulF64VecF64Lit(src, dst []float64, lit float64)
// w1: $0x6e60dc25 => 'fmul.2d	v5, v1, v0'
// w2: $0x6e60dc46 => 'fmul.2d	v6, v2, v0'
// w3: $0x6e60dc67 => 'fmul.2d	v7, v3, v0'
// w4: $0x6e60dc88 => 'fmul.2d	v8, v4, v0'
TEXT ·mulF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat($0x6e60dc25, $0x6e60dc46, $0x6e60dc67, $0x6e60dc88, MOVD, $8, $8, D2, D)

// func mulF32VecF32Lit(src, dst []float32, lit float32)
// w1: $0x6e20dc25 => 'fmul.4s	v5, v1, v0'
// w2: $0x6e20dc46 => 'fmul.4s	v6, v2, v0'
// w3: $0x6e20dc67 => 'fmul.4s	v7, v3, v0'
// w4: $0x6e20dc88 => 'fmul.4s	v8, v4, v0'
TEXT ·mulF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat($0x6e20dc25, $0x6e20dc46, $0x6e20dc67, $0x6e20dc88, MOVW, $4, $16, S4, S)

// func divFloat64VecByFloat64Lit(src, dst []float64, lit float64)
// w1: $0x6e60fc25 => 'fdiv.2d	v5, v1, v0'
// w2: $0x6e60fc46 => 'fdiv.2d	v6, v2, v0'
// w3: $0x6e60fc67 => 'fdiv.2d	v7, v3, v0'
// w4: $0x6e60fc88 => 'fdiv.2d	v8, v4, v0'
TEXT ·divFloat64VecByFloat64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat($0x6e60fc25, $0x6e60fc46, $0x6e60fc67, $0x6e60fc88, MOVD, $8, $8, D2, D)

// func divF32VecF32Lit(src, dst []float32, lit float32)
// w1: $0x6e20fc25 => 'fdiv.4s	v5, v1, v0'
// w2: $0x6e20fc46 => 'fdiv.4s	v6, v2, v0'
// w3: $0x6e20fc67 => 'fdiv.4s	v7, v3, v0'
// w4: $0x6e20fc88 => 'fdiv.4s	v8, v4, v0'
TEXT ·divF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat($0x6e20fc25, $0x6e20fc46, $0x6e20fc67, $0x6e20fc88, MOVW, $4, $16, S4, S)

#define vOpVecFloat(w1, w2, w3, w4, dSize, chnkSize, spec, spec1) \
    MOVD src1Addr+0(FP), R0                                \
    MOVD src2Addr+24(FP), R1                               \
    MOVD dstAddr+48(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB chnkSize, R3, R5                                   \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R3                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VLD1.P 64(R0), [V1.spec, V2.spec, V3.spec, V4.spec]    \
    VLD1.P 64(R1), [V5.spec, V6.spec, V7.spec, V8.spec]    \
    WORD w1                                                \
    WORD w2                                                \
    WORD w3                                                \
    WORD w4                                                \
    VST1.P [V9.spec, V10.spec, V11.spec, V12.spec], 64(R2) \  
    ADD chnkSize, R4, R4                                   \
    CMP R5, R4                                             \
    BLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    VLD1 (R1), V5.spec1[0]                                 \
    WORD w1                                                \
    VST1 V9.spec1[0], (R2)                                 \
    ADD dSize, R0, R0                                      \ 
    ADD dSize, R1, R1                                      \
    ADD dSize, R2, R2                                      \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addF64VecF64Vec(src1, src2, dst []float64)
// w1: $0x4e65d429 => 'fadd.2d v9, v1, v5'
// w2: $0x4e66d44a => 'fadd.2d v10, v2, v6'
// w3: $0x4e67d46b => 'fadd.2d v11, v3, v7'
// w4: $0x4e68d48c => 'fadd.2d v12, v4, v8'
TEXT ·addF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x4e65d429, $0x4e66d44a, $0x4e67d46b, $0x4e68d48c, $8, $8, D2, D)

// func addF32VecF32Vec(src1, src2, dst []float32)
// w1: $0x4e25d429 => 'fadd.4s v9, v1, v5'
// w2: $0x4e26d44a => 'fadd.4s v10, v2, v6'
// w3: $0x4e27d46b => 'fadd.4s v11, v3, v7'
// w4: $0x4e28d48c => 'fadd.4s v12, v4, v8'
TEXT ·addF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x4e25d429, $0x4e26d44a, $0x4e27d46b, $0x4e28d48c, $4, $16, S4, S)

// func subF64VecF64Vec(src1, src2, dst []float64)
// w1: $0x4ee5d429 => 'fsub.2d v9, v1, v5'
// w2: $0x4ee6d44a => 'fsub.2d v10, v2, v6'
// w3: $0x4ee7d46b => 'fsub.2d v11, v3, v7'
// w4: $0x4ee8d48c => 'fsub.2d v12, v4, v8'
TEXT ·subF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x4ee5d429, $0x4ee6d44a, $0x4ee7d46b, $0x4ee8d48c, $8, $8, D2, D)

// func subF32VecF32Vec(src1, src2, dst []float32)
// w1: $0x4ea5d429 => 'fsub.4s v9, v1, v5'
// w2: $0x4ea6d44a => 'fsub.4s v10, v2, v6'
// w3: $0x4ea7d46b => 'fsub.4s v11, v3, v7'
// w4: $0x4ea8d48c => 'fsub.4s v12, v4, v8'
TEXT ·subFloat32VecByFloat32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x4ea5d429, $0x4ea6d44a, $0x4ea7d46b, $0x4ea8d48c, $4, $16, S4, S)

// func mulF64VecF64Vec(src1, src2, dst []float64)
// w1: $0x6e65dc29 => 'fmul.2d v9, v1, v5'
// w2: $0x6e66dc4a => 'fmul.2d v10, v2, v6'
// w3: $0x6e67dc6b => 'fmul.2d v11, v3, v7'
// w4: $0x6e68dc8c => 'fmul.2d v12, v4, v8'
TEXT ·mulF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x6e65dc29, $0x6e66dc4a, $0x6e67dc6b, $0x6e68dc8c, $8, $8, D2, D)

// func mulF32VecF32Vec(src1, src2, dst []float32)
// w1: $0x6e25dc29 => 'fmul.4s v9, v1, v5'
// w2: $0x6e26dc4a => 'fmul.4s v10, v2, v6'
// w3: $0x6e27dc6b => 'fmul.4s v11, v3, v7'
// w4: $0x6e28dc8c => 'fmul.4s v12, v4, v8'
TEXT ·mulF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x6e25dc29, $0x6e26dc4a, $0x6e27dc6b, $0x6e28dc8c, $4, $16, S4, S)

// func divF64VecF64Vec(src1, src2, dst []float64)
// w1: $0x6e65fc29 => 'fdiv.2d v9, v1, v5'
// w2: $0x6e66fc4a => 'fdiv.2d v10, v2, v6'
// w3: $0x6e67fc6b => 'fdiv.2d v11, v3, v7'
// w4: $0x6e68fc8c => 'fdiv.2d v12, v4, v8'
TEXT ·divF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x6e65fc29, $0x6e66fc4a, $0x6e67fc6b, $0x6e68fc8c, $8, $8, D2, D)

// func divF32VecF32Vec(src1, src2, dst []float32)
// w1: $0x6e25fc29 => 'fdiv.4s v9, v1, v5'
// w2: $0x6e26fc4a => 'fdiv.4s v10, v2, v6'
// w3: $0x6e27fc6b => 'fdiv.4s v11, v3, v7'
// w4: $0x6e28fc8c => 'fdiv.4s v12, v4, v8'
TEXT ·divF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($0x6e25fc29, $0x6e26fc4a, $0x6e27fc6b, $0x6e28fc8c, $4, $16, S4, S)
