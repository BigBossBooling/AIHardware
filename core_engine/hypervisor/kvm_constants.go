package hypervisor

// KVM ioctl constants.
// These values are typically found in <linux/kvm.h>.
// It's crucial to use the correct values for the target kernel.
// For portability and to avoid cgo, we define them directly.
// Reference: https://www.kernel.org/doc/Documentation/virtual/kvm/api.txt
// Or by inspecting /usr/include/linux/kvm.h on a Linux system.

const (
	// ioctl_KVM_GET_API_VERSION is the KVM API version.
	// It should return KVM_API_VERSION (which is 12).
	// #define KVM_GET_API_VERSION       _IO(KVMIO,   0x00)
	ioctl_KVM_GET_API_VERSION = 0xAE00 // IO (KVMIO, 0x00) - KVMIO is 0xAE

	// ioctl_KVM_CREATE_VM creates a new virtual machine.
	// Returns a VM file descriptor.
	// #define KVM_CREATE_VM           _IO(KVMIO,   0x01)
	ioctl_KVM_CREATE_VM = 0xAE01 // IO (KVMIO, 0x01)

	// KVM_API_VERSION is the expected API version.
	KVM_API_VERSION = 12
)

const (
	// kvmDevicePath is the path to the KVM device file.
	kvmDevicePath = "/dev/kvm"
)

// Add other KVM constants here as they become necessary, for example:
// KVM_SET_USER_MEMORY_REGION, KVM_CREATE_VCPU, KVM_RUN etc.
//
// For ioctl number construction (for reference, not directly used in Go below without unsafe):
// _IO(type, nr)    ((type) << 8 | (nr))
// _IOW(type, nr, size)  ((type) << 8 | (nr) | (sizeof(size) << 16)) | (1 << 30) // DIR_WRITE
// _IOR(type, nr, size)  ((type) << 8 | (nr) | (sizeof(size) << 16)) | (1 << 31) // DIR_READ
// _IOWR(type, nr, size) ((type) << 8 | (nr) | (sizeof(size) << 16)) | (3 << 30) // DIR_WRITE | DIR_READ
//
// KVMIO is 0xAE.
// Example: KVM_GET_API_VERSION is _IO(KVMIO, 0x00)
// So, (0xAE << 8) | 0x00 = 0xAE00

// --- KVM System ioctls (on /dev/kvm fd) ---
// (KVM_GET_API_VERSION, KVM_CREATE_VM already here)

// --- VM ioctls (on VM fd) ---
// (KVM_CREATE_VCPU already here)

// ioctl_KVM_SET_USER_MEMORY_REGION is used to create, modify, or delete a guest physical memory region.
// Takes a pointer to struct kvm_userspace_memory_region.
// #define KVM_SET_USER_MEMORY_REGION _IOW(KVMIO, 0x46, struct kvm_userspace_memory_region)
// Note: _IOW means data is written from userspace to kernel.
// Size of struct kvm_userspace_memory_region is part of the ioctl number encoding for _IOW/_IOR.
// However, for Go's x/sys/unix, we pass the correctly sized structure as the third argument,
// and the ioctl number constant doesn't usually encode the size directly in its value if defined simply.
// Let's verify the actual value from headers or reliable source.
// From /usr/include/linux/kvm.h: #define KVM_SET_USER_MEMORY_REGION      _IOW(KVMIO, 0x46, struct kvm_userspace_memory_region)
// KVMIO = 0xAE. For _IOW, it's ((WRITE) << _IOC_DIRSHIFT) | ((TYPE) << _IOC_TYPESHIFT) | ((NR) << _IOC_NRSHIFT) | ((SIZE) << _IOC_SIZESHIFT)
// WRITE = 1. The actual calculation is complex. Often these are defined as hex constants.
// A common value found for KVM_SET_USER_MEMORY_REGION is 0x4020AE46 for a 32-byte struct (common on 64-bit).
// Let's use the simpler base number if x/sys/unix handles the _IOW nature correctly by pointer type.
// If not, specific uintptr conversion with unsafe.Pointer will be needed.
// For now, using the base number and relying on x/sys/unix.Ioctl uintptr arg.
const ioctl_KVM_SET_USER_MEMORY_REGION = 0xAE46 // Placeholder, may need adjustment based on _IOW encoding if not using raw syscalls with correctly calculated value.
                                              // More robust: Calculate or find the fully encoded value.
                                              // Let's use a more common value found in other projects if direct calculation is tricky:
                                              // Example: 0x4020AE46 (assuming 32-byte struct kvm_userspace_memory_region)
                                              // For x/sys/unix, often the unadorned number (like _IO) is used and the pointer type handles direction.
                                              // Let's try with the direct _IO style number first and see if unix.Ioctl... works.
                                              // No, _IOW is not like _IO. The number must be correct.
                                              // The struct size is sizeof(struct kvm_userspace_memory_region) which is 40 bytes on 64-bit.
                                              // Let's use the value that corresponds to that.
                                              // #define KVM_SET_USER_MEMORY_REGION      _IOW(KVMIO, 0x46, struct kvm_userspace_memory_region)
                                              // For a struct of 40 bytes (5 * uint64_t):
                                              // IOC_WRITE = 1U
                                              // From Go's perspective, we'll use unix.IoctlPtr সভাবে.
                                              // The constant itself should be the "request" part, which is usually TYPE << 8 | NR
                                              // So, 0xAE46 is likely correct for the 'request' part, and IoctlPtr handles direction.
                                              // Let's stick with the direct interpretation first: KVMIO (0xAE) << 8 | 0x46
                                              // This seems to be the case for many other KVM ioctls in x/sys/unix.
                                              // Rechecking: KVM_SET_USER_MEMORY_REGION is indeed complex.
                                              // A common definition is `_IOW(KVMIO, 0x46, struct kvm_userspace_memory_region)`.
                                              // The actual value for x86_64 (where kvm_userspace_memory_region is 40 bytes) is often 0x4028AE46.
                                              // Let's define it as such if we were to use raw syscalls.
                                              // For unix.IoctlWritePtr, the `req` should be this full value.
const KVM_NR_SET_USER_MEMORY_REGION = 0x46
// Let's use the fully encoded value for clarity if available, or construct if necessary.
// For now, assuming unix.IoctlWritePointer will work with a simpler req + typed pointer.
// This might be a point of failure / refinement.
// Let's assume for now we will use unix.IoctlWritePointer with the base request number.
// However, most sources indicate the fully encoded number is needed.
// For x86_64, struct kvm_userspace_memory_region is 40 bytes.
// #define _IOC_WRITE          1U
// #define _IOC_TYPEBITS       8
// #define _IOC_NRBITS         8
// #define _IOC_SIZEBITS       14
// #define _IOC_NRSHIFT        0
// #define _IOC_TYPESHIFT      (_IOC_NRSHIFT + _IOC_NRBITS)
// #define _IOC_SIZESHIFT      (_IOC_TYPESHIFT + _IOC_TYPEBITS)
// #define _IOC_DIRSHIFT       (_IOC_SIZESHIFT + _IOC_SIZEBITS)
// #define _IOW(type,nr,size)  ((_IOC_WRITE<<_IOC_DIRSHIFT)|((type)<<_IOC_TYPESHIFT)|((nr)<<_IOC_NRSHIFT)|((__IOC_ тело_SIZEOF(size))<<_IOC_SIZESHIFT))
// Where __IOC_ тело_SIZEOF(size) is effectively sizeof(struct kvm_userspace_memory_region)
// If struct is 32 bytes (0x20):
// _IOW(0xAE, 0x46, size=32) = (1<<30) | (0xAE<<8) | (0x46<<0) | (0x20<<16)
// = 0x40000000 | 0x0000AE00 | 0x00000046 | 0x00200000 = 0x4020AE46
const ioctl_KVM_SET_USER_MEMORY_REGION_FULL = 0x4020AE46 // Corrected for 32-byte struct


// Flags for kvm_userspace_memory_region.flags
const (
	KVM_MEM_LOG_DIRTY_PAGES uint32 = 1 << 0
	KVM_MEM_READONLY        uint32 = 1 << 1
)


// --- VCPU ioctls (on VCPU fd) ---
// (KVM_GET_VCPU_MMAP_SIZE, KVM_RUN already here)
//
// Example: KVM_CREATE_VM is _IO(KVMIO, 0x01)
// So, (0xAE << 8) | 0x01 = 0xAE01

const (
	// ioctl_KVM_CREATE_VCPU creates a VCPU for a VM.
	// Argument is VCPU id. Returns VCPU fd.
	// #define KVM_CREATE_VCPU           _IO(KVMIO,   0x41)
	ioctl_KVM_CREATE_VCPU = 0xAE41 // IO (KVMIO, 0x41)

	// ioctl_KVM_GET_VCPU_MMAP_SIZE returns the size of the kvm_run structure.
	// #define KVM_GET_VCPU_MMAP_SIZE    _IO(KVMIO,   0x04)
	ioctl_KVM_GET_VCPU_MMAP_SIZE = 0xAE04 // IO (KVMIO, 0x04)

	// ioctl_KVM_RUN runs a VCPU.
	// Argument is VCPU fd (passed to ioctl on the VCPU fd itself, not KVM fd).
	// #define KVM_RUN                   _IO(KVMIO,   0x80)
	ioctl_KVM_RUN = 0xAE80 // IO (KVMIO, 0x80)
)

// KVM Exit Reasons from kvm_run.exit_reason
const (
	KVM_EXIT_UNKNOWN      = 0
	KVM_EXIT_EXCEPTION    = 1
	KVM_EXIT_IO           = 2
	KVM_EXIT_HYPERCALL    = 3
	KVM_EXIT_DEBUG        = 4
	KVM_EXIT_HLT          = 5
	KVM_EXIT_MMIO         = 6
	KVM_EXIT_IRQ_WINDOW_OPEN = 7
	KVM_EXIT_SHUTDOWN     = 8
	KVM_EXIT_FAIL_ENTRY   = 9
	KVM_EXIT_INTR         = 10
	KVM_EXIT_SET_TPR      = 11
	KVM_EXIT_TPR_ACCESS   = 12
	KVM_EXIT_S390_SIEIC   = 13
	KVM_EXIT_S390_RESET   = 14
	KVM_EXIT_DCR          = 15 // Deprecated
	KVM_EXIT_NMI          = 16
	KVM_EXIT_INTERNAL_ERROR = 17
	KVM_EXIT_OSI          = 18
	KVM_EXIT_PAPR_HCALL   = 19
	KVM_EXIT_S390_UCONTROL = 20
	KVM_EXIT_WATCHDOG     = 21
	KVM_EXIT_S390_TSCH    = 22
	KVM_EXIT_EPR          = 23
	KVM_EXIT_SYSTEM_EVENT = 24
	KVM_EXIT_S390_STSI    = 25
	KVM_EXIT_IOAPIC_EOI   = 26
	KVM_EXIT_HYPERV       = 27
)

// Note: Direct syscalls in Go are complex. We'll use the `golang.org/x/sys/unix` package
// for ioctl operations, which abstracts some of this. However, the request codes themselves
// are still needed.
// The constants defined here are the "request" arguments for the ioctl syscall.
// For _IO style ioctls (like GET_API_VERSION, CREATE_VM), the third argument to an ioctl syscall (uintptr(0)) is often ignored.
// For _IOW, _IOR, _IOWR, it would be a pointer to the data structure being passed.
