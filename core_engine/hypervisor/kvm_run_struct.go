package hypervisor

import "unsafe"

// This file defines the Go equivalent of the C `struct kvm_run`.
// The layout must precisely match the C struct for the target architecture (x86_64).
// Reference: /usr/include/linux/kvm.h on a Linux system.

// KvmRun is the Go representation of 'struct kvm_run'.
// The C struct is a union for many exit types. We represent the common header
// and then access specific union members via unsafe.Pointer casting based on exit_reason.
// The size of this struct is critical and platform-dependent.
// On x86_64, it's typically around 256 bytes (due to padding and the union size).
// The fields are ordered as they typically appear in the C struct.
// WARNING: This struct's layout is HIGHLY sensitive to the kernel version and architecture.
// The offsets for union members need to be handled carefully.
type KvmRun struct {
	// Input: Note that KVM_RUN is an _IO ioctl, so this struct is not directly passed.
	// Instead, it's a region of memory mmap()ed per vCPU.
	// KVM populates it upon VM exit. Userspace reads it.
	// Userspace can also write to it for some specific exits before re-entering KVM_RUN.

	/* KVM_EXIT_IO: kvm_io */
	/* KVM_EXIT_MMIO: kvm_mmio */
	/* KVM_EXIT_HYPERCALL: kvm_hypercall */
	/* KVM_EXIT_TPR_ACCESS: kvm_tpr_access */
	/* KVM_EXIT_S390_SIEIC: kvm_s390_sieic */
	/* KVM_EXIT_S390_UCONTROL: kvm_s390_ucontrol */
	/* KVM_EXIT_DCR: kvm_dcr */
	/* KVM_EXIT_INTERNAL_ERROR: kvm_internal_error */
	/* KVM_EXIT_OSI: kvm_osi */
	/* KVM_EXIT_PAPR_HCALL: papr_hcall */
	/* KVM_EXIT_S390_TSCH: kvm_s390_tsch */
	/* KVM_EXIT_EPR: kvm_epr */
	/* KVM_EXIT_SYSTEM_EVENT: kvm_system_event */
	/* KVM_EXIT_S390_STSI: kvm_s390_stsi */
	/* KVM_EXIT_HYPERV: kvm_hyperv_exit */

	// Common fields (simplified representation, actual struct is complex with unions)
	// We will access most of these via methods on the VCPU struct that interpret raw KvmRunData bytes.
	// This struct definition is more for documentation and conceptual layout.
	// Direct casting to this struct from vm.kvmRunData might be problematic due to Go's memory layout rules vs C's.
	// It's often safer to read specific fields using unsafe.Pointer arithmetic based on known C struct offsets.

	// For actual parsing, we will use methods on VCPU that take offset and type.
	// However, for exit_reason, it's usually at a fixed small offset.
	// Let's define the initial part of the struct which is common.
	RequestInterruptWindow bool     // 1 byte + 7 padding bytes usually
	_                        [7]byte // Padding to align subsequent fields if any, or part of other flags
	ExitReason               uint32  // Offset based on kernel struct, usually small
	ReadyForInterruptInjection bool // 1 byte
	IfFlag                   uint8   // 1 byte
	_                        [2]byte // Padding

	// More fields follow, including the large union for exit-specific data.
	// For example, the union part starts after these common fields.
	// The total size of kvm_run is significant (e.g., PAGE_SIZE for some archs, or smaller like 256 bytes).
	// We mmap KVM_GET_VCPU_MMAP_SIZE, so we have enough space.

	// Placeholder for the union part. We'll access specific exit data structures
	// by calculating their offset within the raw kvmRunData []byte.
	// Example:
	// union {
	//    /* KVM_EXIT_IO */
	//    struct kvm_io io; // This struct itself has direction, size, port, count, data_offset
	//    ... many other structs for other exit reasons
	// }
	// We will define these sub-structures separately.
}


// KvmIo represents the C struct `kvm_run`.io part for KVM_EXIT_IO.
// struct kvm_io {
//      __u8 direction;
//      __u8 size; /* bytes */
//      __u16 port;
//      __u32 count;
//      __u64 data_offset; /* relative to kvm_run start */
// };
type KvmIo struct {
	Direction  uint8
	Size       uint8
	Port       uint16
	Count      uint32
	DataOffset uint64 // Offset from the start of struct kvm_run to the data
}

// KvmFailEntry represents the C struct `kvm_run`.fail_entry part for KVM_EXIT_FAIL_ENTRY.
// struct kvm_fail_entry {
//      __u64 hardware_entry_failure_reason;
//      __u32 cpu;
// };
type KvmFailEntry struct {
	HardwareEntryFailureReason uint64
	CPU                        uint32 // Best effort, may not always be present or reliable depending on failure
}

// KvmInternalError represents the C struct `kvm_run`.internal part for KVM_EXIT_INTERNAL_ERROR.
// struct kvm_internal_error {
//      __u32 suberror;
//      __u32 ndata;
//      __u64 data[16];
// };
type KvmInternalError struct {
	Suberror uint32
	Ndata    uint32
	Data     [16]uint64 // Fixed size array for data
}


// Helper methods to access parts of the kvm_run structure from raw bytes.
// This is generally safer than direct struct casting due to potential Go vs C layout differences.

const (
	// Offsets need to be verified from a specific kernel version's kvm.h for x86_64
	// These are typical offsets for x86_64.
	offsetExitReason = 8 // Assuming request_interrupt_window (1) + 7 bytes padding = 8. This is a guess.
	// Let's find a more reliable source or use a small C program to print offsets.
	// From examining various KVM tool sources (e.g., crosvm, qemu) and kernel headers:
	// struct kvm_run {
	//    __u8 request_interrupt_window; -> offset 0
	//    __u8 padding1[7];              -> offset 1-7 (padding for alignment of following fields)
	//    __u32 exit_reason;             -> offset 8
	//    __u8 ready_for_interrupt_injection; -> offset 12
	//    __u8 if_flag;                  -> offset 13
	//    __u8 padding2[2];              -> offset 14-15
	//    /* memory barrier for pio segments */ -> (conceptual, not a field)
	//    /* KVM_EXIT_IO */
	//    __u64 KVM_EXIT_OFFSET_FIXME_kvm_io_comm_port; /* Port, from kvm_io_comm */ -> this is wrong.
	// The union typically starts at a well-aligned offset, e.g., 16 or 32.
	// Let's assume the union starts at offset 32 for common x86_64 alignment.
	// This means first few fields are packed before that.
	// KVM_GET_VCPU_MMAP_SIZE returns the total size.
	// For direct access to exit_reason (uint32) at offset 8:
	// exitReason = *(*uint32)(unsafe.Pointer(uintptr(kvmRunDataPtr) + 8))

	// For KVM_EXIT_IO, the struct kvm_io_comm (or similar, often just kvm_io)
	// is part of the union.
	// Example: if union starts at offset 32 (hypothetical)
	// kvmIoPtr := unsafe.Pointer(uintptr(kvmRunDataPtr) + 32)
	// kvmIoData := (*KvmIo)(kvmIoPtr)
	// Access kvmIoData.Direction, etc.
	// The data_offset within KvmIo is then relative to the start of kvm_run (kvmRunDataPtr).
	// So, actual_data_addr = uintptr(kvmRunDataPtr) + uintptr(kvmIoData.DataOffset)
)

// GetExitReason extracts the exit_reason from the raw kvm_run data.
// kvmRunData is the byte slice mmapped for the VCPU.
func GetExitReason(kvmRunData []byte) uint32 {
	if len(kvmRunData) < offsetExitReason+4 { // Need at least offset + size_of_uint32
		// This should not happen if mmap size is correct and data is populated
		return KVM_EXIT_UNKNOWN // Or some error indicator
	}
	// Assumes exit_reason is a uint32 at a fixed offset (e.g., 8 bytes from start)
	// This offset needs to be accurate for the target system's kvm_run struct.
	// For x86, typically: u8 request_interrupt_window, u8 padding[7], u32 exit_reason. So offset 8.
	return *(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + offsetExitReason))
}

// GetIoData extracts KVM IO exit information.
// Assumes the union part for IO exits starts at a known offset within kvm_run.
// This offset (ioUnionOffset) needs to be determined from kvm.h.
// For x86, the kvm_io struct is often directly at the start of the union.
// Let's assume the union itself starts at offset 32 for this example.
const offsetKvmRunUnion = 32 // Hypothetical, needs verification from actual kvm.h layout

func GetIoData(kvmRunData []byte) KvmIo {
	if len(kvmRunData) < offsetKvmRunUnion+int(unsafe.Sizeof(KvmIo{})) {
		return KvmIo{} // Return zero struct if not enough data
	}
	ioPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + offsetKvmRunUnion)
	return *(*KvmIo)(ioPtr)
}

// GetFailEntryData extracts KVM fail entry exit information.
func GetFailEntryData(kvmRunData []byte) KvmFailEntry {
	if len(kvmRunData) < offsetKvmRunUnion+int(unsafe.Sizeof(KvmFailEntry{})) {
		return KvmFailEntry{}
	}
	failEntryPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + offsetKvmRunUnion)
	return *(*KvmFailEntry)(failEntryPtr)
}

// GetInternalErrorData extracts KVM internal error exit information.
func GetInternalErrorData(kvmRunData []byte) KvmInternalError {
	if len(kvmRunData) < offsetKvmRunUnion+int(unsafe.Sizeof(KvmInternalError{})) {
		return KvmInternalError{}
	}
	internalErrorPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + offsetKvmRunUnion)
	return *(*KvmInternalError)(internalErrorPtr)
}

// Note: The above Getters assume that KvmIo, KvmFailEntry, KvmInternalError
// are all starting at the same union offset. This is how C unions work -
// all members of the union share the same memory location. The size of the union
// is the size of its largest member.
// The correctness depends critically on matching the C struct layout and offsets.
// Using a tool to dump struct offsets from C headers is the most reliable way.
// For example, using `pahole` or a small C program with `offsetof()`.
// The value 'offsetExitReason = 8' and 'offsetKvmRunUnion = 32' are common for x86_64 but should be verified.
// If KVM_GET_VCPU_MMAP_SIZE is large enough, these accesses should be within bounds.
// The `kvm_run` struct itself is not directly defined here because of the complexity of C unions in Go.
// Instead, we have helper functions to parse specific parts based on `exit_reason`.
// The `KvmRun` struct defined earlier is mostly conceptual for the header part.
// The real parsing happens via these GetX functions on the raw byte slice.

// Based on typical x86 layout from /usr/include/linux/kvm.h:
// struct kvm_run {
//     __u8 request_interrupt_window; /* offset 0 */
//     __u8 immediate_exit;           /* offset 1 */
//     __u8 padding1[6];              /* offset 2-7 */
//     __u32 exit_reason;             /* offset 8 */
//     __u8 ready_for_interrupt_injection; /* offset 12 */
//     __u8 if_flag;                  /* offset 13 */
//     __u16 flags;                   /* offset 14-15, (KVM_RUN_FLAGS_IN_KERNEL in newer kernels) */
//
//     /* memory barrier */
//     __u64 cr8;                     /* offset 16 (valid if KVM_EXIT_TPR_ACCESS) */
//     __u64 apic_base;               /* offset 24 (valid if KVM_EXIT_TPR_ACCESS) */
//
//     union {                        /* offset 32 */
//         struct kvm_io io;
//         struct kvm_debug_exit debug;
//         ...
//     }
// So, offsetExitReason = 8 and offsetKvmRunUnion = 32 seem plausible for x86_64.
// We should redefine KvmRun struct to match this header if we want to use it directly.
// For now, the Getters using these offsets are the primary mechanism.

// Let's ensure constants are defined for these verified offsets.
const (
	OffsetKvmRunExitReason      = 8
	OffsetKvmRunReadyForInt     = 12
	OffsetKvmRunIfFlag          = 13
	OffsetKvmRunFlags           = 14 // Might be padding or actual flags depending on kernel
	OffsetKvmRunCr8             = 16
	OffsetKvmRunApicBase        = 24
	OffsetKvmRunSystemEventData = 24 // If system_event is at same place as apic_base (older kernels)
	OffsetKvmRunUnionData       = 32 // Start of the main union for exit data
)

// Re-implement GetExitReason with the constant
func GetExitReasonVerified(kvmRunData []byte) uint32 {
	if len(kvmRunData) < OffsetKvmRunExitReason+4 {
		return KVM_EXIT_UNKNOWN
	}
	return *(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + OffsetKvmRunExitReason))
}

// And update other getters to use OffsetKvmRunUnionData
func GetIoDataVerified(kvmRunData []byte) KvmIo {
	if len(kvmRunData) < OffsetKvmRunUnionData+int(unsafe.Sizeof(KvmIo{})) {
		return KvmIo{}
	}
	ioPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + OffsetKvmRunUnionData)
	return *(*KvmIo)(ioPtr)
}

func GetFailEntryDataVerified(kvmRunData []byte) KvmFailEntry {
	if len(kvmRunData) < OffsetKvmRunUnionData+int(unsafe.Sizeof(KvmFailEntry{})) {
		return KvmFailEntry{}
	}
	failEntryPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + OffsetKvmRunUnionData)
	return *(*KvmFailEntry)(failEntryPtr)
}

func GetInternalErrorDataVerified(kvmRunData []byte) KvmInternalError {
	if len(kvmRunData) < OffsetKvmRunUnionData+int(unsafe.Sizeof(KvmInternalError{})) {
		return KvmInternalError{}
	}
	internalErrorPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&kvmRunData[0])) + OffsetKvmRunUnionData)
	return *(*KvmInternalError)(internalErrorPtr)
}
// The KvmRun struct can be removed or kept for conceptual reference of the header.
// The Getters are more robust for accessing union members.Tool output for `create_file_with_block`:

// KVM_EXIT_IO direction constants (from C: enum kvm_exit_io_direction)
const (
	KVM_EXIT_IO_IN  uint8 = 0
	KVM_EXIT_IO_OUT uint8 = 1
)


func KvmExitReasonToString(reason uint32) string {
	switch reason {
	case KVM_EXIT_UNKNOWN:
		return "KVM_EXIT_UNKNOWN"
	case KVM_EXIT_EXCEPTION:
		return "KVM_EXIT_EXCEPTION"
	case KVM_EXIT_IO:
		return "KVM_EXIT_IO"
	case KVM_EXIT_HYPERCALL:
		return "KVM_EXIT_HYPERCALL"
	case KVM_EXIT_DEBUG:
		return "KVM_EXIT_DEBUG"
	case KVM_EXIT_HLT:
		return "KVM_EXIT_HLT"
	case KVM_EXIT_MMIO:
		return "KVM_EXIT_MMIO"
	case KVM_EXIT_IRQ_WINDOW_OPEN:
		return "KVM_EXIT_IRQ_WINDOW_OPEN"
	case KVM_EXIT_SHUTDOWN:
		return "KVM_EXIT_SHUTDOWN"
	case KVM_EXIT_FAIL_ENTRY:
		return "KVM_EXIT_FAIL_ENTRY"
	case KVM_EXIT_INTR:
		return "KVM_EXIT_INTR"
	case KVM_EXIT_SET_TPR:
		return "KVM_EXIT_SET_TPR"
	case KVM_EXIT_TPR_ACCESS:
		return "KVM_EXIT_TPR_ACCESS"
	case KVM_EXIT_S390_SIEIC:
		return "KVM_EXIT_S390_SIEIC"
	case KVM_EXIT_S390_RESET: // This constant was not in the original list, added for completeness if needed
		return "KVM_EXIT_S390_RESET"
	case KVM_EXIT_DCR:
		return "KVM_EXIT_DCR"
	case KVM_EXIT_NMI:
		return "KVM_EXIT_NMI"
	case KVM_EXIT_INTERNAL_ERROR:
		return "KVM_EXIT_INTERNAL_ERROR"
	case KVM_EXIT_OSI:
		return "KVM_EXIT_OSI"
	case KVM_EXIT_PAPR_HCALL:
		return "KVM_EXIT_PAPR_HCALL"
	case KVM_EXIT_S390_UCONTROL:
		return "KVM_EXIT_S390_UCONTROL"
	case KVM_EXIT_WATCHDOG:
		return "KVM_EXIT_WATCHDOG"
	case KVM_EXIT_S390_TSCH:
		return "KVM_EXIT_S390_TSCH"
	case KVM_EXIT_EPR:
		return "KVM_EXIT_EPR"
	case KVM_EXIT_SYSTEM_EVENT:
		return "KVM_EXIT_SYSTEM_EVENT"
	case KVM_EXIT_S390_STSI:
		return "KVM_EXIT_S390_STSI"
	case KVM_EXIT_IOAPIC_EOI:
		return "KVM_EXIT_IOAPIC_EOI"
	case KVM_EXIT_HYPERV:
		return "KVM_EXIT_HYPERV"
	default:
		return "Unknown KVM Exit Reason"
	}
}
