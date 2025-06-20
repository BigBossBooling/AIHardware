package core_engine

// Assuming pb types are generated and accessible via this import path
// The go_package option in the .proto file was "./;proto"
// and the files are generated into proto/ directory.
// So, the import path should be relative to the module root.
// If core_engine is a package within the module root, and proto is also at module root:
// import pb "github.com/V-Architect/v-architect-core/proto"
// If core_engine is the module root itself:
import pb "github.com/V-Architect/v-architect-core/proto"


// Hypervisor defines the interface for hypervisor operations.
type Hypervisor interface {
	// CreateVM creates a new virtual machine instance context with the hypervisor.
	// It takes a VMConfig (protobuf definition).
	// It returns a hypervisor-specific file descriptor (e.g., KVM VM fd) for the created VM,
	// the VCPU mmap size required by this hypervisor, and an error if creation fails.
	CreateVM(config *pb.VMConfig) (vmFd int, vcpuMmapSize int, err error)

	// CloseVMContext closes or releases the virtual machine instance context.
	// vmFd is the file descriptor obtained from CreateVM.
	CloseVMContext(vmFd int) error

	// GetHostCapabilities retrieves information about the host system's and hypervisor's capabilities.
	// Returns the protobuf HostCapabilities message.
	GetHostCapabilities() (*pb.HostCapabilities, error)

	// Close releases any global resources held by the hypervisor instance (e.g., /dev/kvm fd).
	Close() error

	// GetKVMRunSize returns the size of the KVM_RUN structure for VCPU mmap.
	// This is somewhat KVM specific but useful if abstracting over KVM-like hypervisors.
	GetKVMRunSize() (int, error)

	// GetAPIVersion returns the API version of the hypervisor (e.g., KVM_GET_API_VERSION).
	// This is useful for checking compatibility and available features.
	// (This was removed from the interface in a previous iteration, adding back for clarity as KVMHypervisor uses it)
	GetAPIVersion() (int, error)
}
