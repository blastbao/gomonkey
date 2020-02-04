package gomonkey

import (
	"fmt"
	"reflect"
	"syscall"
	"unsafe"
)

type Patches struct {
	originals map[reflect.Value][]byte
	values map[reflect.Value]reflect.Value
}

type Params []interface{}
type OutputCell struct {
	Values Params
	Times  int
}

func ApplyFunc(target, double interface{}) *Patches {
	return create().ApplyFunc(target, double)
}

func ApplyMethod(target reflect.Type, methodName string, double interface{}) *Patches {
	return create().ApplyMethod(target, methodName, double)
}

func ApplyGlobalVar(target, double interface{}) *Patches {
	return create().ApplyGlobalVar(target, double)
}

func ApplyFuncVar(target, double interface{}) *Patches {
	return create().ApplyFuncVar(target, double)
}

func ApplyFuncSeq(target interface{}, outputs []OutputCell) *Patches {
	return create().ApplyFuncSeq(target, outputs)
}

func ApplyMethodSeq(target reflect.Type, methodName string, outputs []OutputCell) *Patches {
	return create().ApplyMethodSeq(target, methodName, outputs)
}

func ApplyFuncVarSeq(target interface{}, outputs []OutputCell) *Patches {
	return create().ApplyFuncVarSeq(target, outputs)
}

func create() *Patches {
	return &Patches{originals: make(map[reflect.Value][]byte), values: make(map[reflect.Value]reflect.Value)}
}

func NewPatches() *Patches {
	return create()
}



func (this *Patches) ApplyFunc(target, double interface{}) *Patches {
	t := reflect.ValueOf(target)
	d := reflect.ValueOf(double)
	return this.ApplyCore(t, d)
}



func (this *Patches) ApplyMethod(target reflect.Type, methodName string, double interface{}) *Patches {
	m, ok := target.MethodByName(methodName)
	if !ok {
		panic("retrieve method by name failed")
	}
	d := reflect.ValueOf(double)
	return this.ApplyCore(m.Func, d)
}

func (this *Patches) ApplyGlobalVar(target, double interface{}) *Patches {
	t := reflect.ValueOf(target)
	if t.Type().Kind() != reflect.Ptr {
		panic("target is not a pointer")
	}

	this.values[t] = reflect.ValueOf(t.Elem().Interface())
	d := reflect.ValueOf(double)
	t.Elem().Set(d)
	return this
}

func (this *Patches) ApplyFuncVar(target, double interface{}) *Patches {
	t := reflect.ValueOf(target)
	d := reflect.ValueOf(double)
	if t.Type().Kind() != reflect.Ptr {
		panic("target is not a pointer")
	}
	this.check(t.Elem(), d)
	return this.ApplyGlobalVar(target, double)
}

func (this *Patches) ApplyFuncSeq(target interface{}, outputs []OutputCell) *Patches {
	funcType := reflect.TypeOf(target)
	t := reflect.ValueOf(target)
	d := getDoubleFunc(funcType, outputs)
	return this.ApplyCore(t, d)
}

func (this *Patches) ApplyMethodSeq(target reflect.Type, methodName string, outputs []OutputCell) *Patches {
	m, ok := target.MethodByName(methodName)
	if !ok {
		panic("retrieve method by name failed")
	}
	d := getDoubleFunc(m.Type, outputs)
	return this.ApplyCore(m.Func, d)
}

func (this *Patches) ApplyFuncVarSeq(target interface{}, outputs []OutputCell) *Patches {
	t := reflect.ValueOf(target)
	if t.Type().Kind() != reflect.Ptr {
		panic("target is not a pointer")
	}
	if t.Elem().Kind() != reflect.Func {
		panic("target is not a func")
	}

	funcType := reflect.TypeOf(target).Elem()
	double := getDoubleFunc(funcType, outputs).Interface()
	return this.ApplyGlobalVar(target, double)
}

func (this *Patches) Reset() {
	for target, bytes := range this.originals {
		modifyBinary(*(*uintptr)(getPointer(target)), bytes)
		delete(this.originals, target)
	}

	for target, variable := range this.values {
		target.Elem().Set(variable)
	}
}

func (this *Patches) ApplyCore(target, replacement reflect.Value) *Patches {
	this.check(target, replacement)
	if _, ok := this.originals[target]; ok {
		panic("patch has been existed")
	}
	original := replace(*(*uintptr)(getPointer(target)), uintptr(getPointer(replacement)))
	this.originals[target] = original
	return this
}

func (this *Patches) check(target, double reflect.Value) {
	if target.Kind() != reflect.Func {
		panic("target is not a func")
	}

	if double.Kind() != reflect.Func {
		panic("double is not a func")
	}

	if target.Type() != double.Type() {
		panic(fmt.Sprintf("target type(%s) and double type(%s) are different", target.Type(), double.Type()))
	}
}

// 当执行 replace(target, replacement) 时，会动态将函数 target 的执行指令替换成 `MOV rdx, replacement; JMP [rdx]`，
//
// 后续在调用函数 target 时，会执行 call 指令，这时候会把传递给函数 target 的参数保存到栈上，并且将返回地址保存到指定的寄存器 RA 中；
// 因为函数 target 被替换成上诉两条指令，因此会跳转到函数 replacement 中执行，这时候函数 replacement 可以直接使用栈上的参数（这就要求两个函数要有相同的函数签名）；
// 因为 morestack 操作是在函数开始执行的时候进行检查的，因此不会有栈溢出的问题。
//
// 当函数 replacement 执行完成时，会执行 ret 指令，这时候会把返回值保存到栈上，同时将 RA 中的返回地址弹出到 PC 寄存器中；
// 对于函数调用者来说，整个过程是透明的。

func replace(target, replacement uintptr) []byte {

	// buildJmpDirective() 生成 `jmp replacement` 的机器码，用于替换 target
	mockCode := buildJmpDirective(replacement)

	// 取出旧函数 target 的开始 len(mockCode) 个字节的机器码，用于备份，以便恢复。
	bytes := entryAddress(target, len(mockCode))

	// 保存被替换的机器码到 original 数组中
	original := make([]byte, len(bytes))
	copy(original, bytes)

	// 使用生成的机器码 mockCode 替换 target 函数
	modifyBinary(target, mockCode)

	// 返回被替换的旧代码
	return original
}

func getDoubleFunc(funcType reflect.Type, outputs []OutputCell) reflect.Value {
	if funcType.NumOut() != len(outputs[0].Values) {
		panic(fmt.Sprintf("func type has %v return values, but only %v values provided as double",
			funcType.NumOut(), len(outputs[0].Values)))
	}

	slice := make([]Params, 0)
	for _, output := range outputs {
		t := 0
		if output.Times <= 1 {
			t = 1
		} else {
			t = output.Times
		}
		for j := 0; j < t; j++ {
			slice = append(slice, output.Values)
		}
	}

	i := 0
	len := len(slice)
	return reflect.MakeFunc(funcType, func(_ []reflect.Value) []reflect.Value {
		if i < len {
			i++
			return GetResultValues(funcType, slice[i-1]...)
		}
		panic("double seq is less than call seq")
	})
}

func GetResultValues(funcType reflect.Type, results ...interface{}) []reflect.Value {
	var resultValues []reflect.Value
	for i, r := range results {
		var resultValue reflect.Value
		if r == nil {
			resultValue = reflect.Zero(funcType.Out(i))
		} else {
			v := reflect.New(funcType.Out(i))
			v.Elem().Set(reflect.ValueOf(r))
			resultValue = v.Elem()
		}
		resultValues = append(resultValues, resultValue)
	}
	return resultValues
}

type funcValue struct {
	_ uintptr
	p unsafe.Pointer
}

func getPointer(v reflect.Value) unsafe.Pointer {
	return (*funcValue)(unsafe.Pointer(&v)).p
}

func entryAddress(p uintptr, l int) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: p, Len: l, Cap: l}))
}

func pageStart(ptr uintptr) uintptr {
	return ptr & ^(uintptr(syscall.Getpagesize() - 1))
}
