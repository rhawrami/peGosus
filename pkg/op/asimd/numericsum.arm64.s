//go:build arm64

#include "textflag.h"

#define vSumOp(vVecOp, vTradOp, vReduce, dSize, chnkSize)  \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD dstAddr+24(FP), R1                                \
    MOVD srcLen+8(FP), R2                                  \
    EOR R3, R3                                             \
    SUB chnkSize, R2, R4                                   \
    VEOR V0.B16, V0.B16, V0.B16                            \
    VEOR V1.B16, V1.B16, V1.B16                            \
    VEOR V2.B16, V2.B16, V2.B16                            \
    VEOR V3.B16, V3.B16, V3.B16                            \
    VEOR V4.B16, V4.B16, V4.B16                            \
    VEOR V8.B16, V8.B16, V8.B16                            \
    VEOR V12.B16, V12.B16, V12.B16                         \
                                                           \
    CMP $0, R2                                             \
    BEQ exitFn                                             \
                                                           \
    CMP chnkSize, R2                                       \
    BLT tradLoop                                           \
                                                           \
vecLoop:                                                   \
    vVecOp                                                 \
    ADD chnkSize, R3, R3                                   \
    CMP R4, R3                                             \
    BLT vecLoop                                            \
                                                           \
    VEOR V4.B16, V4.B16, V4.B16                            \
    VEOR V8.B16, V8.B16, V8.B16                            \
tradLoop:                                                  \
    vTradOp                                                \
    ADD dSize, R0, R0                                      \
    ADD $1, R3                                             \
    CMP R2, R3                                             \
    BLT tradLoop                                           \
                                                           \
    vReduce                                                \                                                           
exitFn:                                                    \
    RET

#define I64SumVecLoopOps                            \
        VLD1.P 64(R0), [V4.D2, V5.D2, V6.D2, V7.D2] \
        WORD $0x4ee48400                            \ // 'add.2d v0, v0, v4'
        WORD $0x4ee58421                            \ // 'add.2d v1, v1, v5'
        WORD $0x4ee68442                            \ // 'add.2d v2, v2, v6'
        WORD $0x4ee78463                              // 'add.2d v3, v3, v7'

#define I64SumTradLoopOps  \
        VLD1 (R0), V4.D[0] \
        WORD $0x4ee48400     // 'add.2d v0, v0, v4'

#define I64SumReduceOps  \
        WORD $0x4ee18404 \ // 'add.2d v4, v0, v1'
        WORD $0x4ee38445 \ // 'add.2d v5, v2, v3'
        WORD $0x4ee58480 \ // 'add.2d v0, v4, v5'
        WORD $0x4ee0bc01 \ // 'addp.2d v1, v0, v0'
        VST1 V1.D[0], (R1)

#define I32SumVecLoopOps              \
        VLD1.P 32(R0), [V8.S4, V9.S4] \
        WORD $0x0f20a504              \ // 'sxtl v4.2d, v8.2s'
        WORD $0x4f20a505              \ // 'sxtl2 v5.2d, v8.4s'
        WORD $0x0f20a526              \ // 'sxtl v6.2d, v9.2s'
        WORD $0x4f20a527              \ // 'sxtl2 v7.2d, v9.4s'
        WORD $0x4ee48400              \ // 'add.2d v0, v0, v4'
        WORD $0x4ee58421              \ // 'add.2d v1, v1, v5'
        WORD $0x4ee68442              \ // 'add.2d v2, v2, v6'
        WORD $0x4ee78463                // 'add.2d v3, v3, v7'

#define I32SumTradLoopOps  \
        VLD1 (R0), V4.S[0] \
        WORD $0x0f20a485   \ // 'sxtl v5.2d, v4.2s'
        WORD $0x4ee58421     // 'add.2d v1, v1, v5'

#define F64SumVecLoopOps                            \
        VLD1.P 64(R0), [V4.D2, V5.D2, V6.D2, V7.D2] \
        WORD $0x4e64e488                            \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x4e65e4a9                            \ // 'fcmeq.2d v9, v5, v5'
        WORD $0x4e66e4ca                            \ // 'fcmeq.2d v10, v6, v6'
        WORD $0x4e67e4eb                            \ // 'fcmeq.2d v11, v7, v7'
        WORD $0x6e6c1c88                            \ // 'bsl.16b v8, v4, v12'
        WORD $0x6e6c1ca9                            \ // 'bsl.16b v9, v5, v12'
        WORD $0x6e6c1cca                            \ // 'bsl.16b v10, v6, v12'
        WORD $0x6e6c1ceb                            \ // 'bsl.16b v11, v7, v12'
        WORD $0x4e68d400                            \ // 'fadd.2d v0, v0, v8'
        WORD $0x4e69d421                            \ // 'fadd.2d v1, v1, v9'
        WORD $0x4e6ad442                            \ // 'fadd.2d v2, v2, v10'
        WORD $0x4e6bd463                              // 'fadd.2d v3, v3, v11'

#define F64SumTradLoopOps  \
        VLD1 (R0), V4.D[0] \
        WORD $0x4e64e488   \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x6e6c1c88   \ // 'bsl.16b v8, v4, v12'
        WORD $0x4e68d400     // 'fadd.2d v0, v0, v8'

#define F64SumReduceOps  \
        WORD $0x4e61d404 \ // 'fadd.2d v4, v0, v1'
        WORD $0x4e63d445 \ // 'fadd.2d v5, v2, v3'
        WORD $0x4e65d480 \ // 'fadd.2d v0, v4, v5'
        WORD $0x6e60d401 \ // 'faddp.2d v1, v0, v0'
        VST1 V1.D[0], (R1) 

#define F32SumVecLoopOps \
        VLD1.P 32(R0), [V8.S4, V9.S4] \
        WORD $0x0e617904              \ // 'fcvtl v4.2d, v8.2s'
        WORD $0x4e617905              \ // 'fcvtl2 v5.2d, v8.4s'
        WORD $0x0e617926              \ // 'fcvtl v6.2d, v9.2s'
        WORD $0x4e617927              \ // 'fcvtl2 v7.2d, v9.4s'
        WORD $0x4e64e488              \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x4e65e4a9              \ // 'fcmeq.2d v9, v5, v5'
        WORD $0x4e66e4ca              \ // 'fcmeq.2d v10, v6, v6'
        WORD $0x4e67e4eb              \ // 'fcmeq.2d v11, v7, v7'
        WORD $0x6e6c1c88              \ // 'bsl.16b v8, v4, v12'
        WORD $0x6e6c1ca9              \ // 'bsl.16b v9, v5, v12'
        WORD $0x6e6c1cca              \ // 'bsl.16b v10, v6, v12'
        WORD $0x6e6c1ceb              \ // 'bsl.16b v11, v7, v12'
        WORD $0x4e68d400              \ // 'fadd.2d v0, v0, v8'
        WORD $0x4e69d421              \ // 'fadd.2d v1, v1, v9'
        WORD $0x4e6ad442              \ // 'fadd.2d v2, v2, v10'
        WORD $0x4e6bd463                // 'fadd.2d v3, v3, v11'


#define F32SumTradLoopOps           \
        VEOR V8.B16, V8.B16, V8.B16 \
        VLD1 (R0), V8.S[0]          \
        WORD $0x0e617904            \ // 'fcvtl v4.2d, v8.2s'
        WORD $0x4e64e488            \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x6e6c1c88            \ // 'bsl.16b v8, v4, v12'
        WORD $0x4e68d400            // 'fadd.2d v0, v0, v8'

// func SumI64(src, dst []int64)
TEXT ·SumI64(SB),NOSPLIT,$0-48
    vSumOp(I64SumVecLoopOps, I64SumTradLoopOps, I64SumReduceOps, $8, $8)

// func SumI32(src []int32, dst []int64)
TEXT ·SumI32(SB),NOSPLIT,$0-48
    vSumOp(I32SumVecLoopOps, I32SumTradLoopOps, I64SumReduceOps, $4, $8)

// func SumF64(src, dst []float64)
TEXT ·SumF64(SB),NOSPLIT,$0-48
    vSumOp(F64SumVecLoopOps, F64SumTradLoopOps, F64SumReduceOps, $8, $8)

// func SumF32(src []float32, dst []float64)
TEXT ·SumF32(SB),NOSPLIT,$0-48
    vSumOp(F32SumVecLoopOps, F32SumTradLoopOps, F64SumReduceOps, $4, $8)

#define vSumOpWithValidity(vVecOp, vTradOp, vReduce, dSize) \
    MOVD srcAddr+0(FP), R0                                 \
    MOVD validityAddr+48(FP), R1                           \
    MOVD dstAddr+24(FP), R2                                \
    MOVD srcLen+8(FP), R3                                  \
    EOR R4, R4                                             \
    SUB $8, R3, R5                                         \
                                                           \
    CMP $0, R3                                             \
    BEQ exitFn                                             \
                                                           \
    VMOVQ $0x0000000000000001, $0x0000000000000002, V20    \
    VMOVQ $0x0000000000000004, $0x0000000000000008, V21    \
    VMOVQ $0x0000000000000010, $0x0000000000000020, V22    \
    VMOVQ $0x0000000000000040, $0x0000000000000080, V23    \                    
    VEOR V0.B16, V0.B16, V0.B16                            \
    VEOR V1.B16, V1.B16, V1.B16                            \
    VEOR V2.B16, V2.B16, V2.B16                            \
    VEOR V3.B16, V3.B16, V3.B16                            \
    VEOR V4.B16, V4.B16, V4.B16                            \
    VEOR V8.B16, V8.B16, V8.B16                            \
    VEOR V12.B16, V12.B16, V12.B16                         \
                                                           \
    EOR R7, R7                                             \
    EOR R9, R9                                             \
                                                           \                                                               
    CMP $8, R3                                             \
    BLT tradLoopInit                                       \
                                                           \
vecLoop:                                                   \
    VLD1R.P 1(R1), [V13.B16]                               \
    VAND V20.B16, V13.B16, V14.B16                         \
    VAND V21.B16, V13.B16, V15.B16                         \
    VAND V22.B16, V13.B16, V16.B16                         \
    VAND V23.B16, V13.B16, V17.B16                         \
    VCMEQ V12.D2, V14.D2, V24.D2                           \
    VCMEQ V12.D2, V15.D2, V25.D2                           \
    VCMEQ V12.D2, V16.D2, V26.D2                           \
    VCMEQ V12.D2, V17.D2, V27.D2                           \
    vVecOp                                                 \
                                                           \
    ADD $8, R4, R4                                         \
    CMP R5, R4                                             \ 
    BLT vecLoop                                            \
                                                           \
tradLoopInit:                                              \                                                        
    MOVB (R1), R7                                          \
    VEOR V4.B16, V4.B16, V4.B16                            \
    VEOR V8.B16, V8.B16, V8.B16                            \
tradLoop:                                                  \
    AND $1, R7, R8                                         \
    SUB R8, R9, R8                                         \
    VDUP R8, V14.D2                                        \      
    VCMEQ V12.D2, V14.D2, V24.D2                           \
    vTradOp                                                \
    LSR $1, R7                                             \
    ADD dSize, R0, R0                                      \
    ADD $1, R4                                             \
    CMP R3, R4                                             \
    BLT tradLoop                                           \
                                                           \
    vReduce                                                \
exitFn:                                                    \
    RET      

#define I64SumVecLoopOpsWithValidity                \
        VLD1.P 64(R0), [V4.D2, V5.D2, V6.D2, V7.D2] \
        VBIT V24.B16, V12.B16, V4.B16               \
        VBIT V25.B16, V12.B16, V5.B16               \
        VBIT V26.B16, V12.B16, V6.B16               \
        VBIT V27.B16, V12.B16, V7.B16               \ 
        WORD $0x4ee48400                            \ // 'add.2d v0, v0, v4'
        WORD $0x4ee58421                            \ // 'add.2d v1, v1, v5'
        WORD $0x4ee68442                            \ // 'add.2d v2, v2, v6'
        WORD $0x4ee78463                              // 'add.2d v3, v3, v7'

#define I64SumTradLoopOpsWithValidity  \
        VLD1 (R0), V4.D[0]             \
        VBIT V24.B8, V12.B8, V4.B8     \
        WORD $0x4ee48400                 // 'add.2d v0, v0, v4'

#define I64SumReduceOpsWithValidity  \
        WORD $0x4ee18404             \ // 'add.2d v4, v0, v1'
        WORD $0x4ee38445             \ // 'add.2d v5, v2, v3'
        WORD $0x4ee58480             \ // 'add.2d v0, v4, v5'
        WORD $0x4ee0bc01             \ // 'addp.2d v1, v0, v0'
        VST1 V1.D[0], (R2)

#define I32SumVecLoopOpsWithValidity  \
        VLD1.P 32(R0), [V8.S4, V9.S4] \
        WORD $0x0f20a504              \ // 'sxtl v4.2d, v8.2s'
        WORD $0x4f20a505              \ // 'sxtl2 v5.2d, v8.4s'
        WORD $0x0f20a526              \ // 'sxtl v6.2d, v9.2s'
        WORD $0x4f20a527              \ // 'sxtl2 v7.2d, v9.4s'
        VBIT V24.B16, V12.B16, V4.B16 \
        VBIT V25.B16, V12.B16, V5.B16 \
        VBIT V26.B16, V12.B16, V6.B16 \
        VBIT V27.B16, V12.B16, V7.B16 \ 
        WORD $0x4ee48400              \ // 'add.2d v0, v0, v4'
        WORD $0x4ee58421              \ // 'add.2d v1, v1, v5'
        WORD $0x4ee68442              \ // 'add.2d v2, v2, v6'
        WORD $0x4ee78463                // 'add.2d v3, v3, v7'

#define I32SumTradLoopOpsWithValidity  \
        VLD1 (R0), V4.S[0]             \
        WORD $0x0f20a485               \ // 'sxtl v5.2d, v4.2s'
        VBIT V24.B8, V12.B8, V5.B8     \
        WORD $0x4ee58421                 // 'add.2d v1, v1, v5'

#define F64SumVecLoopOpsWithValidity                            \
        VLD1.P 64(R0), [V4.D2, V5.D2, V6.D2, V7.D2] \
        WORD $0x4e64e488                            \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x4e65e4a9                            \ // 'fcmeq.2d v9, v5, v5'
        WORD $0x4e66e4ca                            \ // 'fcmeq.2d v10, v6, v6'
        WORD $0x4e67e4eb                            \ // 'fcmeq.2d v11, v7, v7'
        WORD $0x6e6c1c88                            \ // 'bsl.16b v8, v4, v12'
        WORD $0x6e6c1ca9                            \ // 'bsl.16b v9, v5, v12'
        WORD $0x6e6c1cca                            \ // 'bsl.16b v10, v6, v12'
        WORD $0x6e6c1ceb                            \ // 'bsl.16b v11, v7, v12'
        VBIT V24.B16, V12.B16, V8.B16               \
        VBIT V25.B16, V12.B16, V9.B16               \
        VBIT V26.B16, V12.B16, V10.B16              \
        VBIT V27.B16, V12.B16, V11.B16              \
        WORD $0x4e68d400                            \ // 'fadd.2d v0, v0, v8'
        WORD $0x4e69d421                            \ // 'fadd.2d v1, v1, v9'
        WORD $0x4e6ad442                            \ // 'fadd.2d v2, v2, v10'
        WORD $0x4e6bd463                              // 'fadd.2d v3, v3, v11'

#define F64SumTradLoopOpsWithValidity  \
        VLD1 (R0), V4.D[0]             \
        VBIT V24.B8, V12.B8, V4.B8     \
        WORD $0x4e64e488               \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x6e6c1c88               \ // 'bsl.16b v8, v4, v12'
        WORD $0x4e68d400                 // 'fadd.2d v0, v0, v8'

#define F64SumReduceOpsWithValidity  \
        WORD $0x4e61d404             \ // 'fadd.2d v4, v0, v1'
        WORD $0x4e63d445             \ // 'fadd.2d v5, v2, v3'
        WORD $0x4e65d480             \ // 'fadd.2d v0, v4, v5'
        WORD $0x6e60d401             \ // 'faddp.2d v1, v0, v0'
        VST1 V1.D[0], (R2) 

#define F32SumVecLoopOpsWithValidity   \
        VLD1.P 32(R0), [V8.S4, V9.S4]  \
        WORD $0x0e617904               \ // 'fcvtl v4.2d, v8.2s'
        WORD $0x4e617905               \ // 'fcvtl2 v5.2d, v8.4s'
        WORD $0x0e617926               \ // 'fcvtl v6.2d, v9.2s'
        WORD $0x4e617927               \ // 'fcvtl2 v7.2d, v9.4s'
        WORD $0x4e64e488               \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x4e65e4a9               \ // 'fcmeq.2d v9, v5, v5'
        WORD $0x4e66e4ca               \ // 'fcmeq.2d v10, v6, v6'
        WORD $0x4e67e4eb               \ // 'fcmeq.2d v11, v7, v7'
        WORD $0x6e6c1c88               \ // 'bsl.16b v8, v4, v12'
        WORD $0x6e6c1ca9               \ // 'bsl.16b v9, v5, v12'
        WORD $0x6e6c1cca               \ // 'bsl.16b v10, v6, v12'
        WORD $0x6e6c1ceb               \ // 'bsl.16b v11, v7, v12'
        VBIT V24.B16, V12.B16, V8.B16  \
        VBIT V25.B16, V12.B16, V9.B16  \
        VBIT V26.B16, V12.B16, V10.B16 \
        VBIT V27.B16, V12.B16, V11.B16 \
        WORD $0x4e68d400               \ // 'fadd.2d v0, v0, v8'
        WORD $0x4e69d421               \ // 'fadd.2d v1, v1, v9'
        WORD $0x4e6ad442               \ // 'fadd.2d v2, v2, v10'
        WORD $0x4e6bd463                 // 'fadd.2d v3, v3, v11'

#define F32SumTradLoopOpsWithValidity \
        VEOR V8.B16, V8.B16, V8.B16   \
        VLD1 (R0), V8.S[0]            \
        WORD $0x0e617904              \ // 'fcvtl v4.2d, v8.2s'
        WORD $0x4e64e488              \ // 'fcmeq.2d v8, v4, v4'
        WORD $0x6e6c1c88              \ // 'bsl.16b v8, v4, v12'
        VBIT V24.B16, V12.B16, V8.B16 \
        WORD $0x4e68d400                // 'fadd.2d v0, v0, v8'
    
// func SumI64WithValidity(src, dst []int64, validity []byte)
TEXT ·SumI64WithValidity(SB),NOSPLIT,$0-48
    vSumOpWithValidity(I64SumVecLoopOpsWithValidity, I64SumTradLoopOpsWithValidity, I64SumReduceOpsWithValidity, $8)

// func SumI32WithValidity(src []int32, dst []int64, validity []byte)
TEXT ·SumI32WithValidity(SB),NOSPLIT,$0-48
    vSumOpWithValidity(I32SumVecLoopOpsWithValidity, I32SumTradLoopOpsWithValidity, I64SumReduceOpsWithValidity, $4)

// func SumF64WithValidity(src, dst []float64, validity []byte)
TEXT ·SumF64WithValidity(SB),NOSPLIT,$0-48
    vSumOpWithValidity(F64SumVecLoopOpsWithValidity, F64SumTradLoopOpsWithValidity, F64SumReduceOpsWithValidity, $8)

// func SumF32WithValidity(src []float32, dst []float64, validity []byte)
TEXT ·SumF32WithValidity(SB),NOSPLIT,$0-48
    vSumOpWithValidity(F32SumVecLoopOpsWithValidity, F32SumTradLoopOpsWithValidity, F64SumReduceOpsWithValidity, $4)
