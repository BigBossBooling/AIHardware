package core_engine

// This file contains conceptual Go representations of KVM structures,
// particularly the KvmRun structure needed for VCPU execution,
// and related constants for exit reasons.

// KvmRun represents a simplified version of the C `struct kvm_run`.
// The actual structure is architecture-dependent and much larger.
// This Go struct is for conceptual use in VCPU.Run() to understand exit reasons
// and access I/O exit data. A real implementation would use cgo or carefully
// aligned Go structs based on <linux/kvm.h>.
type KvmRun struct {
	// --- Fields directly read/written by userspace before KVM_RUN ---
	// RequestInterruptWindow: Guest can request an interrupt window.
	// ImmediateExit: Request KVM to exit immediately without executing guest code.
	// ... other input fields ...

	// --- Fields populated by KVM after KVM_RUN returns ---
	ExitReason                 uint32 // The primary reason for VM exit.
	ReadyForInterruptInjection uint8  // Guest is ready for an interrupt to be injected.
	IfFlag                     uint8  // Interrupt flag (IF) state inside the guest.
	// padding2 [2]uint8 // Example padding

	// Conceptual representation of the I/O exit data portion of the C union.
	// In a real mmap'd structure, these fields would be at specific offsets within
	// the union part of kvm_run, and their validity depends on ExitReason.
	// For this conceptual Go struct, we embed it.
	Io struct {
		Direction   uint8  // KVM_EXIT_IO_IN or KVM_EXIT_IO_OUT
		Size        uint8  // 1, 2, or 4 bytes for PIO (sometimes 8)
		Port        uint16 // I/O Port number
		Count       uint32 // Number of PIO operations (usually 1 for emulated devices)
		DataOffset  uint64 // Offset from start of KvmRun struct to data payload within the KvmRun shared memory region
	}

	// Conceptual placeholder for where KVM would write data for PIO (KVM_EXIT_IO_OUT from guest)
	// or where the VMM writes data for the guest to read (KVM_EXIT_IO_IN to guest).
	// This is highly simplified. The actual data is part of the mmap'd kvm_run page,
	// pointed to by Io.DataOffset from the start of the KvmRun mmap'd region.
	// For this struct, we add a small byte array to represent this general data area conceptually,
	// assuming Io.DataOffset might point within this array or just beyond the struct fields
	// into the larger mmap'd area.
	RawDataPayload [64]byte // Conceptual buffer for PIO data within KvmRun struct

	// Conceptual placeholder for other exit reason data structures
	// Example for KVM_EXIT_INTERNAL_ERROR:
	InternalError struct {
	   Suberror uint32
	   // Ndata    uint32 // Number of uint64 data elements
	   // Data     [16]uint64 // Actual data
	}
	// Note: The actual kvm_run structure has a union for these.
}

// KVM Exit Reasons (subset from <linux/kvm.h>)
const (
	KVM_EXIT_UNKNOWN          uint32 = 0
	KVM_EXIT_EXCEPTION        uint32 = 1
	KVM_EXIT_IO               uint32 = 2
	KVM_EXIT_HYPERCALL        uint32 = 3
	KVM_EXIT_DEBUG            uint32 = 4
	KVM_EXIT_HLT              uint32 = 5
	KVM_EXIT_MMIO             uint32 = 6 // Not handled in this subtask
	KVM_EXIT_IRQ_WINDOW_OPEN  uint32 = 7
	KVM_EXIT_SHUTDOWN         uint32 = 8 // Guest explicitly requested power off
	KVM_EXIT_FAIL_ENTRY       uint32 = 9
	KVM_EXIT_INTERNAL_ERROR   uint32 = 17 // KVM internal error
	KVM_EXIT_SYSTEM_EVENT     uint32 = 24 // Check system_event.type for SHUTDOWN/RESET
	// ... other reasons can be added as needed
)

// KVM_EXIT_IO direction constants (from <linux/kvm.h>)
const (
	KVM_EXIT_IO_IN  uint8 = 0 // Data is read from device into guest
	KVM_EXIT_IO_OUT uint8 = 1 // Data is written from guest to device
)

// KVM_SYSTEM_EVENT types (for KVM_EXIT_SYSTEM_EVENT, from <linux/kvm.h>)
const (
	KVM_SYSTEM_EVENT_SHUTDOWN uint32 = 1 // Guest initiated shutdown
	KVM_SYSTEM_EVENT_RESET    uint32 = 2 // Guest initiated reset
	KVM_SYSTEM_EVENT_CRASH    uint32 = 3 // Guest crashed
)

// Conceptual offsets for accessing fields if KvmRun were a raw byte buffer.
// These are for ILLUSTRATION ONLY and would be derived from <linux/kvm.h>.
// Using direct struct field access is preferred if the Go struct KvmRun is accurately defined.
/*
const (
    ConceptualOffsetExitReason = 0
    ConceptualOffsetIoDirection = 32 // Example, after fixed fields
    ConceptualOffsetIoSize      = 33
    ConceptualOffsetIoPort      = 34
    ConceptualOffsetIoCount     = 36
    ConceptualOffsetIoDataOffset= 40
    ConceptualOffsetInternalErrorSuberror = 32 // If it's the first field in that part of union
)
*/
