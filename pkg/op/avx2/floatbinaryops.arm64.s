//go:build amd64

#include "textflag.h"

#define vOpLitFloat(vOp, mOp, dSize, chnkSize, spec, spec1) \
    MOVQ srcAddr+0(FP), AX                                 \
    MOVQ dstAddr+24(FP), BX                                \
    MOVQ srcLen+8(FP), CX                                  \
    XORQ DI, DI                                            \
    SUBQ chnkSize, CX, SI                                  \
    mOp lit+48(FP), R12                                    \
    VDUP R12, V0.spec                                      \
                                                           \
    CMPQ $0, CX                                            \
    JEQ exitFn                                             \
                                                           \
    CMPQ chnkSize, CX                                      \
    JLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    VMOVUPS (AX), Y1                                       \
    VMOVUPS (AX), Y2                                       \
    VMOVUPS (AX), Y3                                       \
    VMOVUPS (AX), Y4                                       \
    vOp Y0, Y1, Y5                                         \
    vOp Y0, Y2, Y6                                         \
    vOp Y0, Y3, Y7                                         \
    vOp Y0, Y4, Y8                                         \
    VMOVUPS Y5, (BX)                                       \
    VMOVUPS Y6, (BX)                                       \
    VMOVUPS Y7, (BX)                                       \
    VMOVUPS Y8, (BX)                                       \
    ADDQ chnkSize, DI                                      \
    CMPQ SI, DI                                            \ 
    JLT vecLoop                                            \
                                                           \
tradLoop:                                                  \
    VLD1 (R0), V1.spec1[0]                                 \
    WORD w1                                                \
    VST1 V5.spec1[0], (R1)                                 \
    ADD dSize, AX                                          \
    ADD dSize, BX                                          \
    ADD $1, DI                                             \
    CMPQ R2, DI                                            \
    JLT tradLoop                                           \
                                                           \
exitFn:                                                    \
    RET

// func addF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·addF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(MOVD, $8, $8, D2, D)

// func addF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·addF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(MOVW, $4, $16, S4, S)

// func subF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·subF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(MOVD, $8, $8, D2, D)

// func subF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·subF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(MOVW, $4, $16, S4, S)

// func mulF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·mulF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(MOVD, $8, $8, D2, D)

// func mulF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·mulF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(MOVW, $4, $16, S4, S)

// func divF64VecF64Lit(src, dst []float64, lit float64)
TEXT ·divF64VecF64Lit(SB),NOSPLIT,$0-56
    vOpLitFloat(MOVD, $8, $8, D2, D)

// func divF32VecF32Lit(src, dst []float32, lit float32)
TEXT ·divF32VecF32Lit(SB),NOSPLIT,$0-52
    vOpLitFloat(MOVW, $4, $16, S4, S)

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
TEXT ·addF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($8, $8, D2, D)

// func addF32VecF32Vec(src1, src2, dst []float32)
TEXT ·addF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($4, $16, S4, S)

// func subF64VecF64Vec(src1, src2, dst []float64)
TEXT ·subF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($8, $8, D2, D)

// func subF32VecF32Vec(src1, src2, dst []float32)
TEXT ·subF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($4, $16, S4, S)

// func mulF64VecF64Vec(src1, src2, dst []float64)
TEXT ·mulF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($8, $8, D2, D)

// func mulF32VecF32Vec(src1, src2, dst []float32)
TEXT ·mulF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($4, $16, S4, S)

// func divF64VecF64Vec(src1, src2, dst []float64)
TEXT ·divF64VecF64Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($8, $8, D2, D)

// func divF32VecF32Vec(src1, src2, dst []float32)
TEXT ·divF32VecF32Vec(SB),NOSPLIT,$0-72
    vOpVecFloat($4, $16, S4, S)
