package hypervisor

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	KVMIO = 0xAE
)

// KVM API version
var KVM_GET_API_VERSION = IOR(KVMIO, 0x00)

// VM ioctls
var KVM_CREATE_VM = IOR(KVMIO, 0x01)
// KVM_GET_MSR_INDEX_LIST defined below
var KVM_CHECK_EXTENSION = IOR(KVMIO, 0x03)
// KVM_GET_VCPU_MMAP_SIZE defined below
var KVM_SET_USER_MEMORY_REGION = IOW(KVMIO, 0x46, unsafe.Sizeof(KvmUserspaceMemoryRegion{}))

// VCPU ioctls
var KVM_CREATE_VCPU = IOR(KVMIO, 0x41)
var KVM_RUN = IOR(KVMIO, 0x80)
var KVM_GET_REGS = IOR(KVMIO, 0x81)
var KVM_SET_REGS = IOW(KVMIO, 0x82, unsafe.Sizeof(KvmRegs{}))
var KVM_GET_SREGS = IOR(KVMIO, 0x83)
var KVM_SET_SREGS = IOW(KVMIO, 0x84, unsafe.Sizeof(KvmSregs{}))
// KVM_GET_MP_STATE defined below
// KVM_SET_MP_STATE defined below

// KvmUserspaceMemoryRegion is used for KVM_SET_USER_MEMORY_REGION
type KvmUserspaceMemoryRegion struct {
	Slot          uint32
	Flags         uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	UserspaceAddr uint64
}

// KvmRegs is used for KVM_GET_REGS / KVM_SET_REGS
// This needs to match the kernel definition for the target architecture (e.g., x86_64)
type KvmRegs struct {
	Rax, Rbx, Rcx, Rdx    uint64
	Rsi, Rdi, Rsp, Rbp    uint64
	R8, R9, R10, R11      uint64
	R12, R13, R14, R15    uint64
	Rip, Rflags           uint64
}

// KvmSregs is used for KVM_GET_SREGS / KVM_SET_SREGS
type KvmSregs struct {
	Cs, Ds, Es, Fs, Gs, Ss KvmSegment
	Tr, Ldt                KvmSegment
	Gdt                    KvmDtable
	Idt                    KvmDtable
	Cr0, Cr2, Cr3, Cr4, Cr8 uint64
	Efer                   uint64
	ApicBase               uint64
	InterruptBitmap        [KVM_NR_INTERRUPTS / 64]uint64
}

// KvmSegment represents a segment register.
type KvmSegment struct {
	Base     uint64
	Limit    uint32
	Selector uint16
	Type     uint8
	Present  uint8
	Dpl      uint8
	Db       uint8
	S        uint8
	L        uint8
	G        uint8
	Avl      uint8
	Unusable uint8
	Padding  uint8
}

// KvmDtable represents a descriptor table (GDT or IDT).
type KvmDtable struct {
	Base    uint64
	Limit   uint16
	Padding [3]uint16 // Kernel struct has 3x uint16 padding
}

const KVM_NR_INTERRUPTS = 256


// ioctl helpers
func IOR(typ, nr int) uint {
	return IOC(IOC_READ, typ, nr, 0)
}

func IOW(typ, nr int, size uintptr) uint {
	return IOC(IOC_WRITE, typ, nr, int(size))
}

func IOWR(typ, nr int, size uintptr) uint {
	return IOC(IOC_READ|IOC_WRITE, typ, nr, int(size))
}

func IOC(dir, typ, nr, size int) uint {
	return (uint(dir) << IOC_DIRSHIFT) | (uint(typ) << IOC_TYPESHIFT) |
		(uint(nr) << IOC_NRSHIFT) | (uint(size) << IOC_SIZESHIFT)
}

const (
	IOC_NONE  = 0
	IOC_WRITE = 1
	IOC_READ  = 2

	IOC_NRBITS   = 8
	IOC_TYPEBITS = 8
	IOC_SIZEBITS = 14 // This can vary by architecture, 14 is common
	IOC_DIRBITS  = 2  // This can vary, 2 is common

	IOC_NRSHIFT   = 0
	IOC_TYPESHIFT = IOC_NRSHIFT + IOC_NRBITS
	IOC_SIZESHIFT = IOC_TYPESHIFT + IOC_TYPEBITS
	IOC_DIRSHIFT  = IOC_SIZESHIFT + IOC_SIZEBITS
)

// KVM API Wrappers

func KVM_CREATE_VM(kvmFD int) (int, error) {
	vmFD, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFD), uintptr(KVM_CREATE_VM), 0)
	if errno != 0 {
		return 0, fmt.Errorf("KVM_CREATE_VM ioctl failed: %s", errno.Error())
	}
	return int(vmFD), nil
}

func KVM_GET_VCPU_MMAP_SIZE() (int, error) {
	kvmFD, err := syscall.Open("/dev/kvm", syscall.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/kvm for KVM_GET_VCPU_MMAP_SIZE: %w", err)
	}
	defer syscall.Close(kvmFD)

	size, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFD), uintptr(IOR(KVMIO, 0x04)), 0) // KVM_GET_VCPU_MMAP_SIZE is 0x04
	if errno != 0 {
		return 0, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE ioctl failed: %s", errno.Error())
	}
	return int(size), nil
}


func KVM_SET_USER_MEMORY_REGION(vmFD int, slot uint32, guestPhysAddr uint64, memorySize uint64, userspaceAddr uintptr) error {
	region := KvmUserspaceMemoryRegion{
		Slot:          slot,
		Flags:         0, // Optional flags like KVM_MEM_LOG_DIRTY_PAGES
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    memorySize,
		UserspaceAddr: uint64(userspaceAddr),
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFD), uintptr(KVM_SET_USER_MEMORY_REGION), uintptr(unsafe.Pointer(&region)))
	if errno != 0 {
		return fmt.Errorf("KVM_SET_USER_MEMORY_REGION ioctl failed: %s", errno.Error())
	}
	return nil
}


func KVM_CREATE_VCPU(vmFD int, vcpuID int) (int, error) {
	vcpuFD, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFD), uintptr(KVM_CREATE_VCPU), uintptr(vcpuID))
	if errno != 0 {
		return 0, fmt.Errorf("KVM_CREATE_VCPU ioctl failed: %s", errno.Error())
	}
	return int(vcpuFD), nil
}

func KVM_RUN(vcpuFD int) (uintptr, error) {
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), uintptr(KVM_RUN), 0)
	// KVM_RUN can return -1 with errno EINTR if interrupted by a signal,
	// or other errors. Specific error handling might be needed.
	if errno != 0 && errno != syscall.EINTR {
		return ret, fmt.Errorf("KVM_RUN ioctl failed: %s", errno.Error())
	}
	return ret, nil // Return value of KVM_RUN itself is usually 0 on success or -1 on error
}

func KVM_GET_REGS(vcpuFD int) (*KvmRegs, error) {
	var regs KvmRegs
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), uintptr(KVM_GET_REGS), uintptr(unsafe.Pointer(&regs)))
	if errno != 0 {
		return nil, fmt.Errorf("KVM_GET_REGS ioctl failed: %s", errno.Error())
	}
	return &regs, nil
}

func KVM_SET_REGS(vcpuFD int, regs *KvmRegs) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), uintptr(KVM_SET_REGS), uintptr(unsafe.Pointer(regs)))
	if errno != 0 {
		return fmt.Errorf("KVM_SET_REGS ioctl failed: %s", errno.Error())
	}
	return nil
}

func KVM_GET_SREGS(vcpuFD int) (*KvmSregs, error) {
	var sregs KvmSregs
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), uintptr(KVM_GET_SREGS), uintptr(unsafe.Pointer(&sregs)))
	if errno != 0 {
		return nil, fmt.Errorf("KVM_GET_SREGS ioctl failed: %s", errno.Error())
	}
	return &sregs, nil
}

func KVM_SET_SREGS(vcpuFD int, sregs *KvmSregs) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vcpuFD), uintptr(KVM_SET_SREGS), uintptr(unsafe.Pointer(sregs)))
	if errno != 0 {
		return fmt.Errorf("KVM_SET_SREGS ioctl failed: %s", errno.Error())
	}
	return nil
}

func KVM_CHECK_EXTENSION(kvmFD int, cap int) (int, error) {
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFD), uintptr(KVM_CHECK_EXTENSION), uintptr(cap))
	if errno != 0 {
		// KVM_CHECK_EXTENSION returns 0 if extension is not available, >0 if available.
		// An errno usually means a more fundamental ioctl error.
		return 0, fmt.Errorf("KVM_CHECK_EXTENSION ioctl failed for cap %d: %s", cap, errno.Error())
	}
	return int(ret), nil
}
