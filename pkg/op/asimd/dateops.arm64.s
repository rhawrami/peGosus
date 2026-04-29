//go:build arm64

#include "textflag.h"

// Credit to Howard Hinnant for the scalar implementations of the 
// algorithms below.
// https://howardhinnant.github.io/date_algorithms.html

// var scalarConstants = []float32{
// 	1,      // v0
// 	2,      // v1
// 	3,      // v2
// 	4,      // v3
// 	5,      // v4
// 	10,     // v5
// 	12,     // v6
// 	100,    // v7
// 	153,    // v8
// 	365,    // v9
// 	400,    // v10
// 	1460,   // v11
// 	36524,  // v12
// 	146096, // v13
// 	146097, // v14
// 	719468, // v15
// }

#define extractionShared()   \
    WORD $0x4e21da10         \ // 'scvtf.4s v16, v16'
    WORD $0x4e2fd610         \ // 'fadd.4s v16, v16, v15'
    WORD $0x4ea0ea11         \ // 'fcmlt.4s v17, v16, #0'
    WORD $0x4e2d1e32         \ // 'and.16b v18, v17, v13'
    WORD $0x4eb2d613         \ // 'fsub.4s v19, v16, v18'
    WORD $0x6e2efe71         \ // 'fdiv.4s v17, v19, v14' 
    WORD $0x4ea19a31         \ // 'frintz.4s v17, v17' 
                             \
    WORD $0x6e2ede33         \ // 'fmul.4s v19, v17, v14'
    WORD $0x4eb3d612         \ // 'fsub.4s v18, v16, v19'
    WORD $0x4ea19a52         \ // 'frintz.4s v18, v18' 
                             \
    WORD $0x6e2bfe53         \ // 'fdiv.4s v19, v18, v11'
    WORD $0x6e2cfe54         \ // 'fdiv.4s v20, v18, v12'
    WORD $0x6e2dfe55         \ // 'fdiv.4s v21, v18, v13'
    WORD $0x4ea19a73         \ // 'frintz.4s v19, v19' 
    WORD $0x4ea19a94         \ // 'frintz.4s v20, v20' 
    WORD $0x4ea19ab5         \ // 'frintz.4s v21, v21' 
    WORD $0x4eb3d656         \ // 'fsub.4s v22, v18, v19'
    WORD $0x4e34d6d7         \ // 'fadd.4s v23, v22, v20'
    WORD $0x4eb5d6f4         \ // 'fsub.4s v20, v23, v21'
    WORD $0x6e29fe93         \ // 'fdiv.4s v19, v20, v9' 
    WORD $0x4ea19a73         \ // 'frintz.4s v19, v19' 
                             \
    WORD $0x6e2ade35         \ // 'fmul.4s v21, v17, v10'
    WORD $0x4e35d674         \ // 'fadd.4s v20, v19, v21'
                             \
    WORD $0x6e29de75         \ // 'fmul.4s v21, v19, v9'
    WORD $0x6e23fe76         \ // 'fdiv.4s v22, v19, v3'
    WORD $0x6e27fe77         \ // 'fdiv.4s v23, v19, v7'
    WORD $0x4ea19ad6         \ // 'frintz.4s v22, v22' 
    WORD $0x4ea19af7         \ // 'frintz.4s v23, v23' 
    WORD $0x4e36d6b8         \ // 'fadd.4s v24, v21, v22'
    WORD $0x4eb7d716         \ // 'fsub.4s v22, v24, v23'
    WORD $0x4eb6d655         \ // 'fsub.4s v21, v18, v22'
                             \
    WORD $0x6e24deb6         \ // 'fmul.4s v22, v21, v4'
    WORD $0x4e21d6d7         \ // 'fadd.4s v23, v22, v1'
    WORD $0x6e28fef6         \ // 'fdiv.4s v22, v23, v8'
    WORD $0x4ea19ad6         \ // 'frintz.4s v22, v22' 
                             \
    WORD $0x6e28ded7         \ // 'fmul.4s v23, v22, v8'
    WORD $0x4e21d6f8         \ // 'fadd.4s v24, v23, v1'
    WORD $0x6e24ff19         \ // 'fdiv.4s v25, v24, v4'
    WORD $0x4ea19b39         \ // 'frintz.4s v25, v25'
    WORD $0x4eb9d6ba         \ // 'fsub.4s v26, v21, v25'
    WORD $0x4e20d757           // 'fadd.4s v23, v26, v0'
    

// func extractYear(src, dst []int32, scalarConstants []float32)
TEXT ·extractYear(SB),NOSPLIT,$0-72
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4

    CMP $0, R2
    BEQ exitFn

    MOVD constAddr+48(FP), R5
    VLD1R.P 4(R5), [V0.S4]
    VLD1R.P 4(R5), [V1.S4]
    VLD1R.P 4(R5), [V2.S4]
    VLD1R.P 4(R5), [V3.S4]
    VLD1R.P 4(R5), [V4.S4]
    VLD1R.P 4(R5), [V5.S4]
    VLD1R.P 4(R5), [V6.S4]
    VLD1R.P 4(R5), [V7.S4]
    VLD1R.P 4(R5), [V8.S4]
    VLD1R.P 4(R5), [V9.S4]
    VLD1R.P 4(R5), [V10.S4]
    VLD1R.P 4(R5), [V11.S4]
    VLD1R.P 4(R5), [V12.S4]
    VLD1R.P 4(R5), [V13.S4]
    VLD1R.P 4(R5), [V14.S4]
    VLD1R.P 4(R5), [V15.S4]

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V16.S4]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x6e38e439           // 'fcmle.4s v25, v24, v1'
    WORD $0x4e201f3a           // 'and.16b v26, v25, v0'
    WORD $0x4e3ad699           // 'fadd.4s v25, v20, v26'
    WORD $0x4ea1bb39           // 'fcvtzs.4s v25, v25'
    VST1.P [V25.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V16.S[0]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x6e38e439           // 'fcmle.4s v25, v24, v1'
    WORD $0x4e201f3a           // 'and.16b v26, v25, v0'
    WORD $0x4e3ad699           // 'fadd.4s v25, v20, v26'
    WORD $0x4ea1bb39           // 'fcvtzs.4s v25, v25'
    VST1 V25.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET     

// func extractMonth(src, dst []int32, scalarConstants []float32)
TEXT ·extractMonth(SB),NOSPLIT,$0-72
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4
    
    MOVD constAddr+48(FP), R5
    VLD1R.P 4(R5), [V0.S4]
    VLD1R.P 4(R5), [V1.S4]
    VLD1R.P 4(R5), [V2.S4]
    VLD1R.P 4(R5), [V3.S4]
    VLD1R.P 4(R5), [V4.S4]
    VLD1R.P 4(R5), [V5.S4]
    VLD1R.P 4(R5), [V6.S4]
    VLD1R.P 4(R5), [V7.S4]
    VLD1R.P 4(R5), [V8.S4]
    VLD1R.P 4(R5), [V9.S4]
    VLD1R.P 4(R5), [V10.S4]
    VLD1R.P 4(R5), [V11.S4]
    VLD1R.P 4(R5), [V12.S4]
    VLD1R.P 4(R5), [V13.S4]
    VLD1R.P 4(R5), [V14.S4]
    VLD1R.P 4(R5), [V15.S4]

    CMP $0, R2
    BEQ exitFn

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V16.S4]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1.P [V24.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V16.S[0]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1 V24.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET      

// func extractDay(src, dst []int32, scalarConstants []float32)
TEXT ·extractDay(SB),NOSPLIT,$0-72
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4
    
    MOVD constAddr+48(FP), R5
    VLD1R.P 4(R5), [V0.S4]
    VLD1R.P 4(R5), [V1.S4]
    VLD1R.P 4(R5), [V2.S4]
    VLD1R.P 4(R5), [V3.S4]
    VLD1R.P 4(R5), [V4.S4]
    VLD1R.P 4(R5), [V5.S4]
    VLD1R.P 4(R5), [V6.S4]
    VLD1R.P 4(R5), [V7.S4]
    VLD1R.P 4(R5), [V8.S4]
    VLD1R.P 4(R5), [V9.S4]
    VLD1R.P 4(R5), [V10.S4]
    VLD1R.P 4(R5), [V11.S4]
    VLD1R.P 4(R5), [V12.S4]
    VLD1R.P 4(R5), [V13.S4]
    VLD1R.P 4(R5), [V14.S4]
    VLD1R.P 4(R5), [V15.S4]

    CMP $0, R2
    BEQ exitFn

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V16.S4]
    extractionShared()
    WORD $0x4ea1baf7         // 'fcvtzs.4s v23, v23'
    VST1.P [V23.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V16.S[0]
    extractionShared()
    WORD $0x4ea1baf7         // 'fcvtzs.4s v23, v23'
    VST1 V23.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET     

// func truncateYear(src, dst []int32, scalarConstants []float32)
TEXT ·truncateYear(SB),NOSPLIT,$0-72
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4

    CMP $0, R2
    BEQ exitFn

    MOVD constAddr+48(FP), R5
    VLD1R.P 4(R5), [V0.S4]
    VLD1R.P 4(R5), [V1.S4]
    VLD1R.P 4(R5), [V2.S4]
    VLD1R.P 4(R5), [V3.S4]
    VLD1R.P 4(R5), [V4.S4]
    VLD1R.P 4(R5), [V5.S4]
    VLD1R.P 4(R5), [V6.S4]
    VLD1R.P 4(R5), [V7.S4]
    VLD1R.P 4(R5), [V8.S4]
    VLD1R.P 4(R5), [V9.S4]
    VLD1R.P 4(R5), [V10.S4]
    VLD1R.P 4(R5), [V11.S4]
    VLD1R.P 4(R5), [V12.S4]
    VLD1R.P 4(R5), [V13.S4]
    VLD1R.P 4(R5), [V14.S4]
    VLD1R.P 4(R5), [V15.S4]
    WORD $0x4ea0d55b // 'fsub.4s v27, v10, v0'
    WORD $0x4ea7d550 // 'fsub.4s v16, v10, v7'
    WORD $0x4e24d611 // 'fadd.4s v17, v16, v4'
    WORD $0x4e20d63c // 'fadd.4s v28, v17, v1'

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V16.S4]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x6e38e439           // 'fcmle.4s v25, v24, v1'
    WORD $0x4e201f3a           // 'and.16b v26, v25, v0'
    WORD $0x4e3ad699           // 'fadd.4s v25, v20, v26'
    WORD $0x4ea0d730           // 'fsub.4s v16, v25, v0'
    WORD $0x4ea0ea11           // 'fcmlt.4s v17, v16, #0.00'
    WORD $0x4e311f72           // 'and.16b v18, v27, v17'
    WORD $0x4eb2d613           // 'fsub.4s v19, v16, v18'
    WORD $0x6e2afe74           // 'fdiv.4s v20, v19, v10'
    WORD $0x4ea19a94           // 'frintz.4s v20, v20' // era
    WORD $0x6e2ade95           // 'fmul.4s v21, v20, v10'
    WORD $0x4eb5d616           // 'fsub.4s v22, v16, v21' // yoe
    WORD $0x6e29ded7           // 'fmul.4s v23, v22, v9'
    WORD $0x6e23fed8           // 'fdiv.4s v24, v22, v3'
    WORD $0x6e27fed9           // 'fdiv.4s v25, v22, v7'
    WORD $0x4ea19b18           // 'frintz.4s v24, v24'
    WORD $0x4ea19b39           // 'frintz.4s v25, v25'
    WORD $0x4e38d6f1           // 'fadd.4s v17, v23, v24'
    WORD $0x4eb9d632           // 'fsub.4s v18, v17, v25'
    WORD $0x4e3cd653           // 'fadd.4s v19, v18, v28' // doe
    WORD $0x6e2ede96           // 'fmul.4s v22, v20, v14'
    WORD $0x4e33d6d7           // 'fadd.4s v23, v22, v19'
    WORD $0x4eafd6f8           // 'fsub.4s v24, v23, v15'
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1.P [V24.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V16.S[0]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x6e38e439           // 'fcmle.4s v25, v24, v1'
    WORD $0x4e201f3a           // 'and.16b v26, v25, v0'
    WORD $0x4e3ad699           // 'fadd.4s v25, v20, v26'
    WORD $0x4ea0d730           // 'fsub.4s v16, v25, v0'
    WORD $0x4ea0ea11           // 'fcmlt.4s v17, v16, #0.00'
    WORD $0x4e311f72           // 'and.16b v18, v27, v17'
    WORD $0x4eb2d613           // 'fsub.4s v19, v16, v18'
    WORD $0x6e2afe74           // 'fdiv.4s v20, v19, v10'
    WORD $0x4ea19a94           // 'frintz.4s v20, v20' // era
    WORD $0x6e2ade95           // 'fmul.4s v21, v20, v10'
    WORD $0x4eb5d616           // 'fsub.4s v22, v16, v21' // yoe
    WORD $0x6e29ded7           // 'fmul.4s v23, v22, v9'
    WORD $0x6e23fed8           // 'fdiv.4s v24, v22, v3'
    WORD $0x6e27fed9           // 'fdiv.4s v25, v22, v7'
    WORD $0x4ea19b18           // 'frintz.4s v24, v24'
    WORD $0x4ea19b39           // 'frintz.4s v25, v25'
    WORD $0x4e38d6f1           // 'fadd.4s v17, v23, v24'
    WORD $0x4eb9d632           // 'fsub.4s v18, v17, v25'
    WORD $0x4e3cd653           // 'fadd.4s v19, v18, v28' // doe
    WORD $0x6e2ede96           // 'fmul.4s v22, v20, v14'
    WORD $0x4e33d6d7           // 'fadd.4s v23, v22, v19'
    WORD $0x4eafd6f8           // 'fsub.4s v24, v23, v15' 
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1 V24.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET     

// func truncateMonth(src, dst []int32, scalarConstants []float32)
TEXT ·truncateMonth(SB),NOSPLIT,$0-72
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4

    CMP $0, R2
    BEQ exitFn

    MOVD constAddr+48(FP), R5
    VLD1R.P 4(R5), [V0.S4]
    VLD1R.P 4(R5), [V1.S4]
    VLD1R.P 4(R5), [V2.S4]
    VLD1R.P 4(R5), [V3.S4]
    VLD1R.P 4(R5), [V4.S4]
    VLD1R.P 4(R5), [V5.S4]
    VLD1R.P 4(R5), [V6.S4]
    VLD1R.P 4(R5), [V7.S4]
    VLD1R.P 4(R5), [V8.S4]
    VLD1R.P 4(R5), [V9.S4]
    VLD1R.P 4(R5), [V10.S4]
    VLD1R.P 4(R5), [V11.S4]
    VLD1R.P 4(R5), [V12.S4]
    VLD1R.P 4(R5), [V13.S4]
    VLD1R.P 4(R5), [V14.S4]
    VLD1R.P 4(R5), [V15.S4]
    WORD $0x4ea0d55b // 'fsub.4s v27, v10, v0'

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V16.S4]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x4e341e99           // 'and.16b v25, v20, v20'
    WORD $0x4ea0eb30           // 'fcmlt.4s v16, v25, #0'
    WORD $0x4e3b1e11           // 'and.16b v17, v16, v27'
    WORD $0x4eb1d732           // 'fsub.4s v18, v25, v17' 
    WORD $0x6e2afe51           // 'fdiv.4s v17, v18, v10'
    WORD $0x4ea19a31           // 'frintz.4s v17, v17'     
    WORD $0x6e2ade32           // 'fmul.4s v18, v17, v10'
    WORD $0x4eb2d733           // 'fsub.4s v19, v25, v18'  
    WORD $0x6e38e434           // 'fcmle.4s v20, v24, v1'
    WORD $0x4ea2d715           // 'fsub.4s v21, v24, v2'
    WORD $0x4e341cd6           // 'and.16b v22, v6, v20'
    WORD $0x4e36d6b7           // 'fadd.4s v23, v21, v22'
    WORD $0x6e28def8           // 'fmul.4s v24, v23, v8'
    WORD $0x4e21d719           // 'fadd.4s v25, v24, v1'
    WORD $0x6e24ff3a           // 'fdiv.4s v26, v25, v4'
    WORD $0x4ea19b5a           // 'frintz.4s v26, v26'     
    WORD $0x6e29de70           // 'fmul.4s v16, v19, v9'
    WORD $0x6e23fe72           // 'fdiv.4s v18, v19, v3'
    WORD $0x6e27fe74           // 'fdiv.4s v20, v19, v7'
    WORD $0x4ea19a52           // 'frintz.4s v18, v18'
    WORD $0x4ea19a94           // 'frintz.4s v20, v20'
    WORD $0x4e32d615           // 'fadd.4s v21, v16, v18'
    WORD $0x4eb4d6b6           // 'fsub.4s v22, v21, v20'
    WORD $0x4e3ad6d0           // 'fadd.4s v16, v22, v26'  
    WORD $0x6e2ede32           // 'fmul.4s v18, v17, v14'
    WORD $0x4e32d613           // 'fadd.4s v19, v16, v18'
    WORD $0x4eafd678           // 'fsub.4s v24, v19, v15'
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1.P [V24.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V16.S[0]
    extractionShared()
    WORD $0x6e25e6d8           // 'fcmge.4s v24, v22, v5'
    WORD $0x4e22d6d9           // 'fadd.4s v25, v22, v2'
    WORD $0x4e261f1a           // 'and.16b v26, v24, v6'
    WORD $0x4ebad738           // 'fsub.4s v24, v25, v26'
    WORD $0x4e341e99           // 'and.16b v25, v20, v20'
    WORD $0x4ea0eb30           // 'fcmlt.4s v16, v25, #0'
    WORD $0x4e3b1e11           // 'and.16b v17, v16, v27'
    WORD $0x4eb1d732           // 'fsub.4s v18, v25, v17' 
    WORD $0x6e2afe51           // 'fdiv.4s v17, v18, v10'
    WORD $0x4ea19a31           // 'frintz.4s v17, v17'     
    WORD $0x6e2ade32           // 'fmul.4s v18, v17, v10'
    WORD $0x4eb2d733           // 'fsub.4s v19, v25, v18'  
    WORD $0x6e38e434           // 'fcmle.4s v20, v24, v1'
    WORD $0x4ea2d715           // 'fsub.4s v21, v24, v2'
    WORD $0x4e341cd6           // 'and.16b v22, v6, v20'
    WORD $0x4e36d6b7           // 'fadd.4s v23, v21, v22'
    WORD $0x6e28def8           // 'fmul.4s v24, v23, v8'
    WORD $0x4e21d719           // 'fadd.4s v25, v24, v1'
    WORD $0x6e24ff3a           // 'fdiv.4s v26, v25, v4'
    WORD $0x4ea19b5a           // 'frintz.4s v26, v26'     
    WORD $0x6e29de70           // 'fmul.4s v16, v19, v9'
    WORD $0x6e23fe72           // 'fdiv.4s v18, v19, v3'
    WORD $0x6e27fe74           // 'fdiv.4s v20, v19, v7'
    WORD $0x4ea19a52           // 'frintz.4s v18, v18'
    WORD $0x4ea19a94           // 'frintz.4s v20, v20'
    WORD $0x4e32d615           // 'fadd.4s v21, v16, v18'
    WORD $0x4eb4d6b6           // 'fsub.4s v22, v21, v20'
    WORD $0x4e3ad6d0           // 'fadd.4s v16, v22, v26'  
    WORD $0x6e2ede32           // 'fmul.4s v18, v17, v14'
    WORD $0x4e32d613           // 'fadd.4s v19, v16, v18'
    WORD $0x4eafd678           // 'fsub.4s v24, v19, v15'
    WORD $0x4ea1bb18           // 'fcvtzs.4s v24, v24'
    VST1 V24.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET    

#define extractionDoW()      \
    WORD $0x4ea63447         \ // 'cmlt.4s v7, v6, v2'
    WORD $0x4e271c08         \ // 'and.16b v8, v0, v7'
    WORD $0x4ea88429         \ // 'add.4s v9, v1, v8'
    WORD $0x4ea984ca         \ // 'add.4s v10, v6, v9'
                             \
    WORD $0x4e21d94b         \ // 'scvtf.4s v11, v10'
    WORD $0x6e25fd6c         \ // 'fdiv.4s v12, v11, v5'
    WORD $0x4ea1b98d         \ // 'fcvtzs.4s v13, v12'      
    WORD $0x4ead9c8e         \ // 'mul.4s v14, v4, v13'
    WORD $0x6eae854f         \ // 'sub.4s v15, v10, v14'
                             \
    WORD $0x4e271c70         \ // 'and.16b v16, v3, v7'
    WORD $0x4eb085f1           // 'add.4s v17, v15, v16'

// func extractDayOfWeek(src, dst []int32)
TEXT ·extractDayOfWeek(SB),NOSPLIT,$0-48
    MOVD srcAddr+0(FP), R0
    MOVD dstAddr+24(FP), R1
    MOVD srcLen+8(FP), R2
    EOR R3, R3
    SUB $4, R2, R4

    CMP $0, R2
    BEQ exitFn
    
    VMOVQ $0x0000000100000001, $0x0000000100000001, V0
    VMOVQ $0x0000000400000004, $0x0000000400000004, V1
    WORD $0x6ea0b822         // 'neg.4s v2, v1'
    VMOVQ $0x0000000600000006, $0x0000000600000006, V3
    VMOVQ $0x0000000700000007, $0x0000000700000007, V4
    WORD $0x4f00f785         // 'fmov.4s v5, #7.00'

    CMP $4, R2
    BLE tradLoop

vecLoop:
    VLD1.P 16(R0), [V6.S4]
    extractionDoW()
    VST1.P [V17.S4], 16(R1)  

    ADD $4, R3, R3
    CMP R4, R3 
    BLT vecLoop

tradLoop:
    VLD1 (R0), V6.S[0]
    extractionDoW()
    VST1 V17.S[0], (R1)
    ADD $4, R0, R0
    ADD $4, R1, R1
    ADD $1, R3
    CMP R2, R3
    BLT tradLoop

exitFn:
    RET     
