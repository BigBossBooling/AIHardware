package hypervisor

import (
	"fmt"
	"log" // Added for basic logging
	"os"
	"syscall" // Direct syscalls for ioctl, consider x/sys/unix for more safety
	// "unsafe" // No longer needed directly here with x/sys/unix for current ioctls

	"golang.org/x/sys/unix" // Preferred for syscalls where possible
)

// KVMHypervisor represents the KVM hypervisor context.
// It holds the file descriptor for the /dev/kvm device.
type KVMHypervisor struct {
	kvmFd *os.File // File descriptor for /dev/kvm
	// We can add more fields here later, e.g., supported extensions.
}

// VirtualMachine represents a KVM virtual machine.
// It holds the file descriptor for the created VM.
type VirtualMachine struct {
	vmFd int // File descriptor for the VM
	// We can add VM configuration, vCPUs, memory regions etc. here later.
	// For now, it's just a placeholder for the VM FD.
}

// Note on syscalls:
// The standard `syscall` package is powerful but less safe.
// `golang.org/x/sys/unix` provides more Go-idiomatic and safer wrappers.
// We will prefer `golang.org/x/sys/unix` for ioctls.

// ioctl performs an ioctl syscall.
// This is a helper, but for KVM, we'll often use unix.IoctlInt & unix.IoctlRetInt directly.
// For ioctls that pass a pointer (e.g., KVM_SET_USER_MEMORY_REGION),
// uintptr(unsafe.Pointer(data)) would be used for the arg parameter.
func ioctl(fd uintptr, req uintptr, arg uintptr) (err error) {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

// NewKVMHypervisor creates a new KVMHypervisor instance.
// It opens /dev/kvm and verifies the KVM API version.
func NewKVMHypervisor() (*KVMHypervisor, error) {
	log.Printf("Attempting to open KVM device at %s", kvmDevicePath)
	kvmFile, err := os.OpenFile(kvmDevicePath, os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		log.Printf("Error opening %s: %v", kvmDevicePath, err)
		return nil, fmt.Errorf("failed to open %s: %w", kvmDevicePath, err)
	}
	log.Printf("Successfully opened %s, fd: %d", kvmDevicePath, kvmFile.Fd())

	// Get KVM API version
	// For KVM_GET_API_VERSION, the third argument to ioctl is ignored (can be 0).
	// It returns the version number directly.
	version, err := unix.IoctlRetInt(int(kvmFile.Fd()), ioctl_KVM_GET_API_VERSION)
	if err != nil {
		log.Printf("KVM_GET_API_VERSION ioctl failed for fd %d: %v", kvmFile.Fd(), err)
		kvmFile.Close() // Clean up on error
		return nil, fmt.Errorf("KVM_GET_API_VERSION ioctl failed: %w", err)
	}
	log.Printf("KVM_GET_API_VERSION returned: %d", version)

	if version != KVM_API_VERSION {
		log.Printf("Unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
		kvmFile.Close() // Clean up
		return nil, fmt.Errorf("unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
	}

	// Optionally, check for required KVM extensions here in the future if needed.
	// For example, KVM_CHECK_EXTENSION for KVM_CAP_USER_MEMORY, KVM_CAP_IRQCHIP, etc.
	log.Println("KVMHypervisor initialized successfully.")
	return &KVMHypervisor{
		kvmFd: kvmFile,
	}, nil
}

// Close closes the KVM file descriptor.
func (h *KVMHypervisor) Close() error {
	if h.kvmFd != nil {
		log.Printf("Closing KVMHypervisor (fd: %d)", h.kvmFd.Fd())
		err := h.kvmFd.Close()
		if err != nil {
			log.Printf("Error closing KVMHypervisor fd %d: %v", h.kvmFd.Fd(), err)
			return fmt.Errorf("error closing KVM fd %d: %w", h.kvmFd.Fd(), err)
		}
		h.kvmFd = nil // Mark as closed
		log.Printf("KVMHypervisor fd closed successfully.")
		return nil
	}
	log.Println("KVMHypervisor Close called on already closed or uninitialized instance.")
	return nil
}

// CreateVM creates a new KVM virtual machine.
// It uses the KVM_CREATE_VM ioctl on the KVM file descriptor.
// For now, it returns a simple VirtualMachine struct containing the VM's file descriptor.
// The 'machineType' argument is currently unused but could be used for future extensions (e.g. s390 specific).
func (h *KVMHypervisor) CreateVM(machineType ...uint64) (*VirtualMachine, error) {
	log.Println("KVMHypervisor CreateVM called.")
	if h.kvmFd == nil {
		log.Println("Error: CreateVM called on nil or closed KVMHypervisor.")
		return nil, fmt.Errorf("KVMHypervisor is not initialized or has been closed")
	}

	// The KVM_CREATE_VM ioctl can take an optional machine type argument for some architectures (e.g. s390).
	// For x86_64, this argument is typically 0 or ignored.
	// We use IoctlRetInt as KVM_CREATE_VM returns the new VM FD.
	// var mType uintptr // mType is currently unused for x86, will be used if we support other architectures
	// if len(machineType) > 0 {
	// 	mType = uintptr(machineType[0])
	// }

	// According to kvm/api.txt, KVM_CREATE_VM is an _IO ioctl, however, many implementations
	// and examples show it being called with IoctlRetInt or similar, as it returns the VM FD.
	// The man page for KVM_CREATE_VM also states "Returns a file descriptor for the new VM".
	// Let's use unix.IoctlRetInt. If a machine type is passed, it's the third argument.
	// If no machine type (or 0) is needed (common for x86), the third arg for _IO is usually 0.
	// Let's assume x86 for now where the argument is effectively ignored or 0.
	// If unix.IoctlRetInt expects a non-zero arg for non-pointer _IO, we pass 0.
	// The documentation for x/sys/unix.IoctlRetInt says:
	// "IoctlRetInt performs an ioctl operation that returns an int."
	// This matches KVM_CREATE_VM.

	vmFd, err := unix.IoctlRetInt(int(h.kvmFd.Fd()), ioctl_KVM_CREATE_VM)
	// If KVM_CREATE_VM required a parameter (like machineType for s390), it would be:
	// vmFd, err := unix.IoctlRetInt(int(h.kvmFd.Fd()), ioctl_KVM_CREATE_VM, int(mType))
	// But for x86, the third argument isn't typically used with IoctlRetInt for this _IO ioctl.
	// Let's stick to the simpler call first, assuming it implicitly passes 0 as the third arg or it's not needed.

	if err != nil {
		log.Printf("KVM_CREATE_VM ioctl failed for KVM fd %d: %v", h.kvmFd.Fd(), err)
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl failed: %w", err)
	}

	if vmFd < 0 { // Should be caught by err != nil from IoctlRetInt, but as a safeguard.
		log.Printf("KVM_CREATE_VM ioctl returned invalid fd: %d for KVM fd %d", vmFd, h.kvmFd.Fd())
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl returned invalid fd: %d", vmFd)
	}
	log.Printf("Successfully created VM with fd: %d from KVM fd: %d", vmFd, h.kvmFd.Fd())
	return &VirtualMachine{
		vmFd: vmFd,
	}, nil
}

// Close closes the VM file descriptor.
func (vm *VirtualMachine) Close() error {
	if vm.vmFd != 0 {
		log.Printf("Closing VirtualMachine (fd: %d)", vm.vmFd)
		err := unix.Close(vm.vmFd)
		if err != nil {
			log.Printf("Error closing VirtualMachine fd %d: %v", vm.vmFd, err)
			return fmt.Errorf("failed to close VM fd %d: %w", vm.vmFd, err)
		}
		log.Printf("VirtualMachine fd %d closed successfully.", vm.vmFd)
		vm.vmFd = 0 // Mark as closed
		return nil
	}
	log.Println("VirtualMachine Close called on already closed or uninitialized instance.")
	return nil
}

// Placeholder for VirtualMachine methods if we decide to make it richer soon
// func (vm *VirtualMachine) Close() error {
//  if vm.vmFd != 0 {
//    return unix.Close(vm.vmFd)
//  }
//  return nil
// }
