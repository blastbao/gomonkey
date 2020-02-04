package gomonkey

import "syscall"



// 代码自修改，是指可以在运行时动态地 `修改` 或 `扩展` 程序的一种方法。


// 编译器会把程序的代码放在 .text 段，即代码段，这段地址是只读的，
// 系统在加载的时候会把相应的代码数据附上只读属性，这样当对其修改的时候就会引发异常，以此实现程序的保护。
//
// 但是，系统提供了 mprotect 系统调用，它可以修改内存的属性（读/写/执行）。
// 通过调用 mprotect 把代码段变成可写的，就可以对程序代码段进行修改，从而实现代码的自修改。
//
// 原型：
//  syscall.Mprotect(b []byte, prot int) (err error)
// 参数：
//  b 为内存区间的起始地址（虚拟内存地址），区间地址必须和整个系统页大小对齐，而区间长度必须是页大小的整数倍。
//  prot 为指定的新权限标记。
//

func modifyBinary(target uintptr, bytes []byte) {

    function := entryAddress(target, len(bytes))
    page := entryAddress(pageStart(target), syscall.Getpagesize())

    // 设置代码段为 可读/可写/可执行
    err := syscall.Mprotect(page, syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)
    if err != nil {
        panic(err)
    }

    // 修改代码段
    copy(function, bytes)

    // 恢复代码段为 可读/可执行
    err = syscall.Mprotect(page, syscall.PROT_READ|syscall.PROT_EXEC)
    if err != nil {
        panic(err)
    }
}
