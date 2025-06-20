package main

// HostCapabilityInfo holds information about the host's hardware capabilities.
// This is a conceptual representation. A real implementation might involve
// more detailed structs, possibly mirroring protobuf definitions if used.
type HostCapabilityInfo struct {
	CPUType         string
	CPUFeatures     []string
	TotalMemoryMB   uint64
	SupportedVMExt  bool // e.g., VT-x/AMD-V
	KVMAPIVersion   int
	// Add other relevant capabilities like IOMMU, SR-IOV, specific device info etc.
}

// Hypervisor defines the interface that different hypervisor implementations
// (like KVM, and potentially others in the future) must satisfy.
type Hypervisor interface {
	// Initialize performs any necessary setup for the hypervisor.
	Initialize() error

	// GetCapabilities returns information about the host and hypervisor capabilities.
	GetCapabilities() (*HostCapabilityInfo, error)

	// CreateVM creates a new virtual machine instance context.
	// It returns a VM file descriptor (or equivalent handle) and an error.
	// For KVM, this would be the vm_fd.
	CreateVMContext() (int, error) // Returning int for fd, could be interface{} for more generic handle

	// CloseVMContext closes or releases the virtual machine instance context.
	CloseVMContext(vm_fd int) error

	// CheckExtension checks if a specific hypervisor capability/extension is available.
	// The meaning of 'cap' is hypervisor-specific (e.g., KVM_CAP_USER_MEMORY for KVM).
	CheckExtension(cap int) (bool, error)

	// GetAPIVersion returns the API version of the hypervisor (e.g., KVM_GET_API_VERSION).
	GetAPIVersion() (int, error)

	// Add other common hypervisor operations here if needed,
	// though many operations are VM-specific (like vCPU creation, memory mapping)
	// and might be methods on a VM object that uses the hypervisor context.
}
