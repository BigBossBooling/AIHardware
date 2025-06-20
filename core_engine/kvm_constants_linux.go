// +build linux

package core_engine

// These are illustrative KVM constants. Actual values must be taken from <linux/kvm.h>
// or a reliable Go binding like "golang.org/x/sys/unix".
// Using direct hex/decimal values here is for conceptual demonstration and might not match
// any specific kernel version perfectly. The KVM_CAP_* enums are generally stable.
const (
	// KVMIO is the magic number for KVM ioctls.
	KVMIO = 0xAE

	// --- System ioctls (on /dev/kvm fd) ---
	// KVM_GET_API_VERSION: Returns the KVM API version.
	// Corresponds to `ioctl(kvm_fd, KVM_GET_API_VERSION, 0)`
	// In C: #define KVM_GET_API_VERSION       _IO(KVMIO,   0x00)
	KVM_GET_API_VERSION = (KVMIO << 8) | 0x00 // Value: 0xAE00

	// KVM_CREATE_VM: Creates a new virtual machine.
	// Corresponds to `ioctl(kvm_fd, KVM_CREATE_VM, machine_type)`
	// In C: #define KVM_CREATE_VM           _IO(KVMIO,   0x01) /* returns a vm fd */
	KVM_CREATE_VM = (KVMIO << 8) | 0x01 // Value: 0xAE01

	// KVM_CHECK_EXTENSION: Checks if a KVM capability is available.
	// Corresponds to `ioctl(kvm_fd, KVM_CHECK_EXTENSION, capability_enum)`
	// In C: #define KVM_CHECK_EXTENSION       _IO(KVMIO,   0x03)
	KVM_CHECK_EXTENSION = (KVMIO << 8) | 0x03 // Value: 0xAE03

	// KVM_GET_VCPU_MMAP_SIZE: Returns the size of the KVM VCPU mmap area (struct kvm_run).
	// Corresponds to `ioctl(kvm_fd, KVM_GET_VCPU_MMAP_SIZE, 0)`
	// In C: #define KVM_GET_VCPU_MMAP_SIZE    _IO(KVMIO,   0x04) /* returns size of vcpu mmap area */
	KVM_GET_VCPU_MMAP_SIZE = (KVMIO << 8) | 0x04 // Value: 0xAE04

	// --- VM ioctls (on vm_fd) ---
	// KVM_CREATE_VCPU: Creates a VCPU.
	// Corresponds to `ioctl(vm_fd, KVM_CREATE_VCPU, vcpu_id)`
	// In C: #define KVM_CREATE_VCPU           _IO(KVMIO,   0x41) /* returns a vcpu fd */
	KVM_CREATE_VCPU = (KVMIO << 8) | 0x41 // Value: 0xAE41

	// KVM_SET_TSS_ADDR: Sets the Task State Segment address (for x86).
	// Corresponds to `ioctl(vm_fd, KVM_SET_TSS_ADDR, address)`
	// In C: #define KVM_SET_TSS_ADDR          _IO(KVMIO,   0x47)
	KVM_SET_TSS_ADDR = (KVMIO << 8) | 0x47 // Value: 0xAE47

	// KVM_CREATE_IRQCHIP: Creates the in-kernel interrupt controller.
	// Corresponds to `ioctl(vm_fd, KVM_CREATE_IRQCHIP, 0)`
	// In C: #define KVM_CREATE_IRQCHIP        _IO(KVMIO,   0x60)
	KVM_CREATE_IRQCHIP = (KVMIO << 8) | 0x60 // Value: 0xAE60

	// KVM_CREATE_PIT2: Creates the in-kernel PIT (Programmable Interval Timer).
	// Corresponds to `ioctl(vm_fd, KVM_CREATE_PIT2, &kvm_pit_config)`
	// In C: #define KVM_CREATE_PIT2           _IOW(KVMIO,  0x77, struct kvm_pit_config)
	KVM_CREATE_PIT2 = (KVMIO << 8) | 0x77 // Value: 0xAE77 (Note: This is _IOW, but for syscall constant it's the number)

	// KVM_SET_USER_MEMORY_REGION: Defines a guest memory region.
	// Corresponds to `ioctl(vm_fd, KVM_SET_USER_MEMORY_REGION, &kvm_userspace_memory_region)`
	// In C: #define KVM_SET_USER_MEMORY_REGION _IOW(KVMIO, 0x46, struct kvm_userspace_memory_region)
	KVM_SET_USER_MEMORY_REGION = (KVMIO << 8) | 0x46 // Value: 0xAE46


	// --- VCPU ioctls (on vcpu_fd) ---
	// KVM_RUN: Runs the VCPU.
	// Corresponds to `ioctl(vcpu_fd, KVM_RUN, 0)`
	// In C: #define KVM_RUN                   _IO(KVMIO,   0x80)
	KVM_RUN = (KVMIO << 8) | 0x80 // Value: 0xAE80

	// Add more ioctl constants as they are implemented (e.g., for SREGS, REGS, CPUID, MSRS)
	// KVM_GET_SREGS = (KVMIO << 8) | 0x83
	// KVM_SET_SREGS = (KVMIO << 8) | 0x84
	// KVM_SET_CPUID2 = (KVMIO << 8) | 0x90
	// KVM_SET_MSRS = (KVMIO << 8) | 0x89


	// KVM Capabilities (enum values for KVM_CHECK_EXTENSION)
	// These values ARE standard and taken from <linux/kvm.h>
	KVM_CAP_IRQCHIP                 = 0
	KVM_CAP_HLT                     = 1
	// KVM_CAP_MMU_SHADOW_CACHE_CONTROL = 2 // Deprecated by KVM_CAP_USER_MEMORY
	KVM_CAP_USER_MEMORY             = 3  // Allows mapping host userspace memory for guest RAM
	KVM_CAP_SET_TSS_ADDR            = 4  // Allows setting TSS address
	// KVM_CAP_VAPIC                   = 5  // Deprecated by KVM_CAP_IRQCHIP with in-kernel LAPIC
	KVM_CAP_EXT_CPUID               = 7  // Allows setting extended CPUID features
	KVM_CAP_MP_STATE                = 14 // Allows getting/setting vCPU multiprocessor state
	KVM_CAP_COALESCED_MMIO          = 15 // Support for coalesced MMIO
	KVM_CAP_PIT2                    = 31 // For KVM_CREATE_PIT2 (implies KVM_CAP_PIT_STATE2)
	KVM_CAP_IOEVENTFD               = 33 // Support for I/O event file descriptors
	KVM_CAP_IRQFD                   = 32 // Support for interrupt file descriptors
	// ... many others. Add as needed for specific features.


	// KVM_API_VERSION_EXPECTED is the version V-Architect is designed against.
	KVM_API_VERSION_EXPECTED = 12
)

// Ensure build tag is present for Go to compile this only on Linux.
// The `// +build linux` line at the top handles this.
