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

// Note: Direct syscalls in Go are complex. We'll use the `golang.org/x/sys/unix` package
// for ioctl operations, which abstracts some of this. However, the request codes themselves
// are still needed.
// The constants defined here are the "request" arguments for the ioctl syscall.
// For _IO style ioctls (like GET_API_VERSION, CREATE_VM), the third argument to an ioctl syscall (uintptr(0)) is often ignored.
// For _IOW, _IOR, _IOWR, it would be a pointer to the data structure being passed.
