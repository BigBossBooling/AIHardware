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
