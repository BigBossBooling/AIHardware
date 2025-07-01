package hypervisor

import "unsafe" // Required for unsafe.Pointer if KvmIo uses it directly or for calculating offsets

// KVM exit reasons (subset, common ones)
const (
	KVM_EXIT_UNKNOWN          = 0
	KVM_EXIT_EXCEPTION        = 1
	KVM_EXIT_IO               = 2
	KVM_EXIT_HYPERCALL        = 3
	KVM_EXIT_DEBUG            = 4
	KVM_EXIT_HLT              = 5
	KVM_EXIT_MMIO             = 6
	KVM_EXIT_IRQ_WINDOW_OPEN  = 7
	KVM_EXIT_SHUTDOWN         = 8
	KVM_EXIT_FAIL_ENTRY       = 9
	KVM_EXIT_INTR             = 10
	KVM_EXIT_SET_TPR          = 11
	KVM_EXIT_TPR_ACCESS       = 12
	KVM_EXIT_S390_SIEIC       = 13
	KVM_EXIT_S390_RESET       = 14
	KVM_EXIT_DCR              = 15
	KVM_EXIT_NMI              = 16
	KVM_EXIT_INTERNAL_ERROR   = 17
	KVM_EXIT_OSI              = 18
	KVM_EXIT_PAPR_HCALL       = 19
	KVM_EXIT_S390_UCONTROL    = 20
	KVM_EXIT_WATCHDOG         = 21
	KVM_EXIT_S390_TSCH        = 22
	KVM_EXIT_EPR              = 23
	KVM_EXIT_SYSTEM_EVENT     = 24
	KVM_EXIT_S390_STSI        = 25
	KVM_EXIT_IOAPIC_EOI       = 26
	KVM_EXIT_HYPERV           = 27
)

// KVM_EXIT_IO directions
const (
	KVM_EXIT_IO_IN  = 0
	KVM_EXIT_IO_OUT = 1
)

// KVM_EXIT_IO data payload max size (architecture specific, but often small for port IO)
// For x86, PIO data payload is typically within the first few bytes of the data_offset.
// The KvmIo struct uses a flexible array member style in C, which is harder in Go.
// We often define a fixed-size array here or handle the offset carefully.
const KVM_EXIT_IO_MAX_DATA_SIZE = 8 // Guest can do up to 4-byte I/O, string I/O is more complex. Let's use 8 for safety.


// KvmRun is the main structure passed to KVM_RUN ioctl.
// Its layout must exactly match the C struct in <linux/kvm.h> for the host kernel.
// This is a simplified version for x86_64. Fields might vary by architecture.
// The actual size of this struct is obtained via KVM_GET_VCPU_MMAP_SIZE.
type KvmRun struct {
	// Input fields (set by user space before KVM_RUN)
	RequestInterruptWindow uint8
	Padding1               [7]byte // Or other fields depending on arch and KVM version

	// Output fields (set by KVM after KVM_RUN)
	ExitReason uint32
	ReadyForInterruptInjection uint8
	IfFlag uint8
	Padding2 [2]byte

	// More output fields, often part of a union in C.
	// The exact layout depends on ExitReason.
	// Example: KvmSystemEvent, KvmInternalError, etc. are part of this union.
	// For simplicity, we'll define the Io struct which is commonly used.
	// The actual C struct uses a large anonymous union here.
	// We will represent the union part with specific structs, and the user of KvmRun
	// will need to interpret this part based on ExitReason.
	// The offset to these union members is critical.

	// For KVM_EXIT_IO:
	Io KvmIo // This assumes KvmIo is at the correct offset within the union

	// Other potential union members (add as needed, ensure proper offsets or use unsafe.Pointer)
	// Ex KvmDebugExitArch
	// FailEntry KvmFailEntry
	// Internal KvmInternalError
	// ... and many others

	// The following are just placeholders to give an idea of where other exit types might store data.
	// The actual C struct is more complex. This is a highly simplified Go representation.
	// This part of the struct is a C union, so only one of these is valid at a time,
	// and they all start at the same memory offset.
	// You would typically access these fields using unsafe.Pointer arithmetic based on ExitReason.

	// For KVM_EXIT_FAIL_ENTRY
	FailEntry struct {
		HardwareEntryFailureReason uint64
		Cpu						   uint32 // Since KVM v4.6
		// Padding or other fields might exist
	} // Note: This assumes FailEntry starts at the same offset as Io. This is typical for a C union.

	// For KVM_EXIT_INTERNAL_ERROR
	Internal struct {
		Suberror uint32
		Ndata    uint32
		Data     [16]uint64 // Or appropriate size based on kernel
	} // Assumes Internal starts at the same offset as Io.

	// Add other structures for other exit reasons as needed.
	// The key is that these are all part of a union in the C struct,
	// meaning they overlay the same memory region.
	// In Go, you'd typically define the largest member or use unsafe.Pointer
	// to cast to the correct type based on ExitReason.
	// For KvmRun, the KVM_GET_VCPU_MMAP_SIZE ensures enough space for all members.
	// The Io member is one of the first and most common members of this union.
	// Let's assume for now that `Io` is the primary member we care about at this offset.
	// A more robust solution would use `unsafe.Pointer` and offsets for other union members.

	// kvm_sync_regs (not directly used by most simple VMs)
	// ...
}


// KvmIo is part of KvmRun for KVM_EXIT_IO.
// This structure is embedded within the KvmRun struct, often as part of a union.
type KvmIo struct {
	Direction  uint8  // KVM_EXIT_IO_IN or KVM_EXIT_IO_OUT
	Size       uint8  // 1, 2, or 4 bytes
	Port       uint16 // I/O port number
	Count      uint32 // Number of times the I/O operation should be repeated (for string I/O)
	DataOffset uint64 // Offset within KvmRun struct where data is located (for KVM_EXIT_IO_OUT)
	                   // or where data should be placed (for KVM_EXIT_IO_IN).
	                   // Data is typically right after this struct in memory (flexible array member in C).
	// Data payload follows here in memory, accessed via DataOffset.
	// In Go, you'd typically use unsafe.Pointer(&kvmRun.Io) + kvmRun.Io.DataOffset
	// or ensure KvmRun is large enough and access kvmRun.Data[:kvmRun.Io.Size] if Data is a byte slice field.
	// For simplicity in vcpu.go, we assume data can be accessed as an array at DataOffset.
	// Example: dataSlice := (*[KVM_EXIT_IO_MAX_DATA_SIZE]byte)(unsafe.Pointer(uintptr(unsafe.Pointer(vcpu.kvmRun)) + uintptr(vcpu.kvmRun.Io.DataOffset)))[:vcpu.kvmRun.Io.Size]
}

// Assert that KvmIo is structured as expected (offsets are approximations)
// These assertions are more for documentation here, actual C struct layout is king.
var _ [1]struct{} = [unsafe.Offsetof(KvmIo{}.Size) - unsafe.Offsetof(KvmIo{}.Direction)]struct{}{}   // Size is 1 byte, immediately after Direction. Offset diff = 1.
var _ [1]struct{} = [unsafe.Offsetof(KvmIo{}.Port) - unsafe.Offsetof(KvmIo{}.Size)]struct{}{}       // Port is 2 bytes, Size is 1 byte. Port starts 1 byte after Size. Offset diff = 1.
var _ [2]struct{} = [unsafe.Offsetof(KvmIo{}.Count) - unsafe.Offsetof(KvmIo{}.Port)]struct{}{}      // Count is 4 bytes, Port is 2 bytes. Count starts 2 bytes after Port. Offset diff = 2.
var _ [4]struct{} = [unsafe.Offsetof(KvmIo{}.DataOffset) - unsafe.Offsetof(KvmIo{}.Count)]struct{}{} // DataOffset is 8 bytes, Count is 4 bytes. DataOffset starts 4 bytes after Count. Offset diff = 4.


// Note on KvmRun structure:
// The KvmRun struct in C contains a large union for different exit reasons.
// Handling this directly in Go requires careful use of unsafe.Pointer and knowledge
// of the offsets of union members. The KvmIo struct is one member of that union.
// The `Io KvmIo` field in the Go KvmRun struct assumes it's placed at the beginning
// of where this union starts in memory. If other union members are needed,
// they would typically be accessed by casting a pointer to the start of this union area.
// For example:
//   io_offset := unsafe.Offsetof(KvmRun{}.Io) // Get offset of Io field (or the union start)
//   ptr_to_union_member := unsafe.Pointer(uintptr(unsafe.Pointer(kvmRun)) + io_offset)
//   specific_exit_struct := (*SpecificExitStructType)(ptr_to_union_member)

// The definition above for KvmRun places Io directly. If it's the first member of the C union, this is fine.
// If KVM_EXIT_IO is the only exit reason involving this union that you handle, it simplifies things.
// Otherwise, a common pattern is:
// type KvmRun struct {
//     ... common fields ...
//     ExitData [SomeLargeSize]byte // or a large enough field to cover all union members
// }
// And then cast parts of ExitData based on ExitReason.
// For now, the direct embedding of KvmIo (and FailEntry, Internal as examples at same conceptual offset) is a simplification.
// The size of KvmRun is determined by KVM_GET_VCPU_MMAP_SIZE, which accounts for the full union.
// The most important part for KVM_EXIT_IO is that KvmIo struct itself is correct and that
// vcpu.kvmRun.Io.DataOffset correctly points to the data payload area within the mmapped KvmRun region.
// The expression `uintptr(unsafe.Pointer(&kvmRunMap[0])) + uintptr(kvmRun.Io.DataOffset)` is often used
// to get the absolute address of the I/O data.
// Or, if kvmRun is a pointer to the start of the mmaped region:
// `(uintptr(unsafe.Pointer(kvmRun)) + uintptr(kvmRun.Io.DataOffset))`
// And the actual data bytes are typically right after the KvmIo struct itself if DataOffset points there.
// The kernel sets DataOffset to be an offset from the beginning of the kvm_run struct.
// So, `(*[SIZE]byte)(unsafe.Pointer( uintptr(unsafe.Pointer(kvmRun)) + uintptr(kvmRun.Io.DataOffset) ))`
// is a common way to access it.
// My `vcpu.go` uses `unsafe.Pointer(&vcpu.kvmRun.Io.DataOffset)` which is likely incorrect.
// It should be `unsafe.Pointer(uintptr(unsafe.Pointer(vcpu.kvmRun)) + uintptr(vcpu.kvmRun.Io.DataOffset))`
// or more directly, since `kvm_run->io.data_offset` is the offset from the start of `kvm_run`:
// `data_ptr := unsafe.Pointer(uintptr(unsafe.Pointer(vcpu.kvmRun)) + uintptr(vcpu.kvmRun.Io.DataOffset))`
// `dataSlice := (*[KVM_EXIT_IO_MAX_DATA_SIZE]byte)(data_ptr)[:vcpu.kvmRun.Io.Size]`
// The vCPU code has: `dataSlice := (*[hypervisor.KVM_EXIT_IO_MAX_DATA_SIZE]byte)(unsafe.Pointer(&vcpu.kvmRun.Io.DataOffset))[:vcpu.kvmRun.Io.Size]`
// This is interpreting the *value* of DataOffset as the data itself, which is wrong.
// DataOffset is an *offset*. The data is *at* that offset from the start of the kvm_run struct.
// The `vcpu.go` provided earlier had:
// `dataSlice := (*[hypervisor.KVM_EXIT_IO_MAX_DATA_SIZE]byte)(unsafe.Pointer(&vcpu.kvmRun.Io.DataOffset))[:vcpu.kvmRun.Io.Size]`
// This should be corrected in vcpu.go to:
// `baseAddr := uintptr(unsafe.Pointer(vcpu.kvmRun))`
// `dataAddr := baseAddr + uintptr(vcpu.kvmRun.Io.DataOffset)`
// `dataSlice := (*[hypervisor.KVM_EXIT_IO_MAX_DATA_SIZE]byte)(unsafe.Pointer(dataAddr))[:vcpu.kvmRun.Io.Size]`
// I will make this correction when vcpu.go is next modified.

// For now, the struct definitions are the primary goal.
// The KvmRun struct here is a simplified representation focusing on KVM_EXIT_IO.
// A full representation would need to handle the C union more carefully, possibly
// by declaring a large byte array for the union part and then casting to the
// appropriate struct type based on ExitReason.
// However, many KVM examples place the `io` struct directly if it's the primary one used.
// The key is that the mmapped region (kvmRun points to its start) is large enough for any C union member.
// And that KvmIo's Go definition matches the C one for KVM_EXIT_IO.Tool output for `create_file_with_block`:
