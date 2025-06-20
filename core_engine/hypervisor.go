package core_engine

import (
	// Assuming pb types are generated and accessible via this import path
	// This is needed if pb.VMConfig is part of the interface.
	// For now, to avoid import cycle if pb imports core_engine types indirectly,
	// we might need to adjust. However, for this conceptual step, let's assume
	// pb.VMConfig is a known type.
	pb "github.com/V-Architect/v-architect-core/core_engine/pb"
)

// HostCapabilityInfo stores information about the host's capabilities.
// This might be generated from a Protobuf definition in a real scenario,
// or be a Go struct that mirrors the pb.HostCapabilities message.
type HostCapabilityInfo struct {
	CPUType         string
	CPUFeatures     []string // e.g., "VT-x", "EPT", "AVX2"
	TotalMemoryGB   uint64
	KVMAPIVersion   int
	KVMCapabilities map[string]bool // Specific KVM capabilities checked
	// Other fields like GPUInfo, AIAcceleratorInfo from pb.HostCapabilities can be added here
	// For simplicity, keeping it aligned with what KVMHypervisor directly gathers for now.
}

// Hypervisor defines the interface for hypervisor operations.
type Hypervisor interface {
	// GetHostCapabilities retrieves information about the host system's and hypervisor's capabilities.
	// In a full implementation, this might return pb.HostCapabilities directly.
	GetHostCapabilities() (*HostCapabilityInfo, error)

	// CreateVM creates a new virtual machine instance context with the hypervisor.
	// It takes a VM ID (for logging/tracking) and the VM's configuration.
	// It returns a hypervisor-specific file descriptor (e.g., KVM VM fd) for the created VM,
	// or an error if creation fails.
	CreateVM(vmID string, config *pb.VMConfig) (vmFd int, err error)

	// CloseVMContext closes or releases the virtual machine instance context.
	// vmFd is the file descriptor obtained from CreateVM.
	CloseVMContext(vmFd int) error

	// Close releases any global resources held by the hypervisor instance (e.g., /dev/kvm fd).
	Close() error

	// CheckExtension checks if a specific hypervisor capability/extension is available.
	// The meaning of 'capEnum' is hypervisor-specific (e.g., KVM_CAP_USER_MEMORY for KVM).
	// This is more for specific, ad-hoc checks if needed beyond initial capability gathering.
	CheckExtension(capEnum int) (bool, error) // capEnum would be KVM_CAP_* value

	// GetAPIVersion returns the API version of the hypervisor (e.g., KVM_GET_API_VERSION).
	GetAPIVersion() (int, error)
}
