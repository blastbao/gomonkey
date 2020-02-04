package gomonkey

// buildJmpDirective 生成跳转到 to 代表的函数的机器码，这个机器码是一个字节序列，被用来修改代码段。
func buildJmpDirective(to uintptr) []byte {

    // 把目标函数的地址 to 切分成 8 字节序列
    d0 := byte(to)
    d1 := byte(to >> 8)
    d2 := byte(to >> 16)
    d3 := byte(to >> 24)
    d4 := byte(to >> 32)
    d5 := byte(to >> 40)
    d6 := byte(to >> 48)
    d7 := byte(to >> 56)

    // MOV rdx, to     # to 是一个指向函数指针的指针(*funcval) ，需要将其存储到 DX 寄存器，rdx 是 64bit 的 DX 寄存器
    // JMP [rdx]       # 跳转到 to 对应的实际函数的开始处执行

    return []byte{
        0x48, 0xBA, d0, d1, d2, d3, d4, d5, d6, d7, // MOV rdx, to
        0xFF, 0x22,                                 // JMP [rdx]
    }
}

