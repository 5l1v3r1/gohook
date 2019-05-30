package hook

import (
	"reflect"
	"syscall"
	"unsafe"
	// "fmt"
)

type CodeInfo struct {
	Origin []byte
	Fix    []CodeFix
}

func makeSliceFromPointer(p uintptr, length int) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: p,
		Len:  length,
		Cap:  length,
	}))
}

func CopyInstruction(location uintptr, data []byte) {
	f := makeSliceFromPointer(location, len(data))
	setPageWritable(location, len(data), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)
	copy(f, data[:])
	setPageWritable(location, len(data), syscall.PROT_READ|syscall.PROT_EXEC)
}

func hookFunction(mode int, target, replace, trampoline uintptr) (*CodeInfo, error) {
	info := &CodeInfo{}
	jumpcode := genJumpCode(mode, replace, target)

	insLen := len(jumpcode)
	if trampoline != uintptr(0) {
		f := makeSliceFromPointer(target, len(jumpcode)*2)
		insLen = GetInsLenGreaterThan(mode, f, len(jumpcode))
	}

	// target slice
	ts := makeSliceFromPointer(target, insLen)
	info.Origin = make([]byte, len(ts))
	copy(info.Origin, ts)

	if trampoline != uintptr(0) {
		sz := uint32(0)
		if elfInfo != nil {
			sz, _ = elfInfo.GetFuncSize(target)
		}

		/*
		   fmt.Printf("target:%x,replace:%x,trampoline:%x,sz:%d,insLen:%d,jumpcode:",target,replace,trampoline,sz, insLen)
		   for _,c := range(jumpcode) {
		       fmt.Printf(" 0x%x", c)
		   }
		   fmt.Printf("\n")
		*/

		if len(jumpcode) > 5 || sz > 0 {
			//if size of jumpcode == 5, there is no chance we will mess up with jmp instruction
			//in this case we better dont fix code if we can not get function size
			fix, err := FixTargetFuncCode(mode, target, sz, trampoline, insLen)
			if err != nil {
				return nil, err
			}

			for _, v := range fix {

				origin := makeSliceFromPointer(v.Addr, len(v.Code))
				f := make([]byte, len(v.Code))
				copy(f, origin)

				/*
				   // test code
				   fmt.Printf("addr:0x%x, code:", v.Addr)
				   for _, c := range v.Code {
				       fmt.Printf(" %x", c)
				   }
				   fmt.Printf(", origin:")
				   for _, c := range f {
				       fmt.Printf(" %x", c)
				   }
				   fmt.Printf("\n")
				   // end test code
				*/

				CopyInstruction(v.Addr, v.Code)
				v.Code = f
				info.Fix = append(info.Fix, v)
			}

		}
	}

	CopyInstruction(target, jumpcode)

	if trampoline != uintptr(0) {
		CopyInstruction(trampoline, info.Origin)
		jumpcode = genJumpCode(mode, target+uintptr(insLen), trampoline+uintptr(insLen))
		CopyInstruction(trampoline+uintptr(insLen), jumpcode)
	}

	return info, nil
}
