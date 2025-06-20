// +build linux

package core_engine

import "unsafe" // Required for unsafe.Offsetof if used, or general pointer safety awareness

// This file defines Go structures that mirror KVM C structures,
// particularly KvmRun and related constants for vCPU execution.

// KvmRun represents a simplified conceptual Go version of the C `struct kvm_run`.
// The actual structure is much larger, architecture-dependent, and uses C unions.
// This Go struct is primarily for conceptual use in VCPU.Run() to understand
// exit reasons and access I/O exit data conceptually.
// For direct memory access to the mmap'd region, unsafe.Pointer and offsets
// derived from C headers would be necessary in a real implementation.
type KvmRun struct {
	// Input fields (written by userspace before KVM_RUN)
	RequestInterruptWindow uint8
	ImmediateExit          uint8
	// Padding to align, example:
	// padding1 [6]uint8

	// Output fields (populated by KVM after KVM_RUN returns)
	ExitReason                 uint32
	ReadyForInterruptInjection uint8
	IfFlag                     uint8 // In-kernel IF state
	// padding2 [2]uint8 // Example

	// --- Conceptual representation of the C union for exit data ---
	// For KVM_EXIT_IO:
	Io struct {
		Direction  uint8  // As per KVM_EXIT_IO_IN or KVM_EXIT_IO_OUT
		Size       uint8  // 1, 2, or 4 bytes typically for PIO (sometimes 8)
		Port       uint16 // I/O Port number
		Count      uint32 // Number of PIO operations (usually 1 for emulated devices)
		DataOffset uint64 // Offset from the start of the KvmRun mmap'd region to the data payload
	}

	// For KVM_EXIT_MMIO (conceptual placeholder):
	// Mmio struct {
	//    PhysAddr uint64
	//    Data     [8]uint8
	//    Len      uint32
	//    IsWrite  uint8
	// }

	// For KVM_EXIT_INTERNAL_ERROR (conceptual placeholder):
	Internal struct {
		Suberror uint32
		// Ndata    uint32   // Number of uint64 data elements that follow
		// Data     [16]uint64 // Actual data (up to 16 uint64s)
	}

	// For KVM_EXIT_FAIL_ENTRY (conceptual placeholder):
	// FailEntry struct {
	//    HardwareEntryFailureReason uint64
	// }


	// This field is a conceptual placeholder for the actual data buffer area
	// that KVM uses for I/O operations when KVM_EXIT_IO occurs.
	// The Io.DataOffset would point within the mmap'd region, which might
	// be conceptually thought of as starting where this RawDataPayload is.
	// Its size needs to be sufficient for the largest possible I/O data (e.g., 8 bytes for QWORD PIO).
	RawDataPayload [64]byte // Conceptual buffer for PIO data
}

// KVM Exit Reasons (subset from <linux/kvm.h>)
// These values are standard.
const (
	KVM_EXIT_UNKNOWN          uint32 = 0
	KVM_EXIT_EXCEPTION        uint32 = 1
	KVM_EXIT_IO               uint32 = 2
	KVM_EXIT_HYPERCALL        uint32 = 3
	KVM_EXIT_DEBUG            uint32 = 4
	KVM_EXIT_HLT              uint32 = 5
	KVM_EXIT_MMIO             uint32 = 6
	KVM_EXIT_IRQ_WINDOW_OPEN  uint32 = 7
	KVM_EXIT_SHUTDOWN         uint32 = 8
	KVM_EXIT_FAIL_ENTRY       uint32 = 9
	KVM_EXIT_INTERNAL_ERROR   uint32 = 17
	KVM_EXIT_SYSTEM_EVENT     uint32 = 24
	// ... other reasons can be added as needed
)

// KVM_EXIT_IO direction constants (from <linux/kvm.h>)
const (
	KVM_EXIT_IO_IN  uint8 = 0 // Data is read from device into guest
	KVM_EXIT_IO_OUT uint8 = 1 // Data is written from guest to device
)

// KVM_SYSTEM_EVENT types (for KVM_EXIT_SYSTEM_EVENT, from <linux/kvm.h>)
// These are used with v.kvmRun.SYSTEM_EVENT.Type if that part of union is defined.
const (
	KVM_SYSTEM_EVENT_SHUTDOWN_TYPE uint32 = 1
	KVM_SYSTEM_EVENT_RESET_TYPE    uint32 = 2
	KVM_SYSTEM_EVENT_CRASH_TYPE    uint32 = 3
)


// --- Other Arch-Specific Structs (from previous User Sub-Issue 1.1 of this plan) ---
// These are for KVM_SET_REGS, KVM_SET_SREGS, KVM_SET_CPUID2, KVM_SET_MSRS

type kvm_regs struct {
	RAX, RBX, RCX, RDX, RSI, RDI, RSP, RBP uint64
	R8, R9, R10, R11, R12, R13, R14, R15 uint64
	RIP, RFLAGS                         uint64
}

type kvm_segment struct {
	Base     uint64; Limit    uint32; Selector uint16; Type     uint8;
	Present  uint8;  DPL      uint8;  DB       uint8;  S        uint8;
	L        uint8;  G        uint8;  Unusable uint8; Padding  uint8;
}
type kvm_dtable struct { Base uint64; Limit uint16; Padding [3]uint16; }

type kvm_sregs struct {
	CS, DS, ES, FS, GS, SS kvm_segment
	TR, LDT                kvm_segment
	GDT                    kvm_dtable
	IDT                    kvm_dtable
	CR0, CR2, CR3, CR4, CR8 uint64
	EFER                   uint64
	ApicBase               uint64
}

type kvm_cpuid_entry2 struct {
	Function, Index, Flags, Eax, Ebx, Ecx, Edx uint32
}

type kvm_msr_entry struct {
	Index, Reserved uint32; Data uint64;
}

// MSR and CPU register bit constants from previous steps
const (
    MSR_EFER            = 0xC0000080
    EFER_SCE uint64 = 1 << 0
    EFER_LME uint64 = 1 << 8
    EFER_LMA uint64 = 1 << 10
    EFER_NXE uint64 = 1 << 11

    CR0_PE uint64 = 1 << 0
    CR0_MP uint64 = 1 << 1
    CR0_ET uint64 = 1 << 4
    CR0_NE uint64 = 1 << 5
    CR0_WP uint64 = 1 << 16
    CR0_AM uint64 = 1 << 18
    CR0_PG uint64 = 1 << 31

    CR4_PAE  uint64 = 1 << 5
    CR4_MCE  uint64 = 1 << 6
    CR4_PGE  uint64 = 1 << 7
    CR4_OSXSAVE uint64 = 1 << 18
)
