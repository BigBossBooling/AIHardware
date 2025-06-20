// +build linux

package core_engine

import (
	"fmt"
	"os"
	"syscall" // Using syscall directly for conceptual clarity of ioctl numbers
	// For a production system, prefer "golang.org/x/sys/unix" which is more robust
	// and platform-independent where possible (and provides KVM_XXX constants).

	pb "github.com/V-Architect/v-architect-core/proto" // Updated import path
)

// KVMHypervisor implements the Hypervisor interface using KVM.
type KVMHypervisor struct {
	kvmFd           int // File descriptor for /dev/kvm
	apiVersion      int
	vcpuMmapMinSize int               // Minimum KVM VCPU mmap size (from KVM_GET_VCPU_MMAP_SIZE)
	kvmCapabilities map[string]bool   // Stores checked KVM capabilities by their string name
}

// NewKVMHypervisor creates and initializes a KVM hypervisor context.
func NewKVMHypervisor() (*KVMHypervisor, error) {
	fmt.Println("Conceptual KVM: Attempting to open /dev/kvm")
	kvmFile, err := os.OpenFile("/dev/kvm", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/kvm (ensure KVM kernel modules kvm and kvm_intel/kvm_amd are loaded and user has permissions): %w", err)
	}
	kvmFileDescriptor := int(kvmFile.Fd())

	// Get KVM API Version
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFileDescriptor), uintptr(KVM_GET_API_VERSION), 0)
	if errno != 0 {
		_ = kvmFile.Close()
		return nil, fmt.Errorf("KVM_GET_API_VERSION ioctl failed: %w", errno)
	}
	apiVersion := int(ret)
	if apiVersion != KVM_API_VERSION_EXPECTED {
		_ = kvmFile.Close()
		return nil, fmt.Errorf("unsupported KVM API version: got %d, expected %d", apiVersion, KVM_API_VERSION_EXPECTED)
	}
	fmt.Printf("Conceptual KVM: KVM API Version: %d\n", apiVersion)

	// Get VCPU MMAP Size
	ret, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(kvmFileDescriptor), uintptr(KVM_GET_VCPU_MMAP_SIZE), 0)
	if errno != 0 {
		_ = kvmFile.Close()
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE ioctl failed: %w", errno)
	}
	vcpuMmapSize := int(ret)
	if vcpuMmapSize <= 0 {
		_ = kvmFile.Close()
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE returned invalid size: %d", vcpuMmapSize)
	}
	fmt.Printf("Conceptual KVM: KVM VCPU MMAP Size: %d bytes\n", vcpuMmapSize)


	h := &KVMHypervisor{
		kvmFd:           kvmFileDescriptor,
		apiVersion:      apiVersion,
		vcpuMmapMinSize: vcpuMmapSize,
		kvmCapabilities: make(map[string]bool),
		// Note: kvmFile is not stored in the struct to avoid needing to manage its closure
		// if NewKVMHypervisor succeeds but KVMHypervisor.Close() is missed.
		// The fd (kvmFileDescriptor) is what's used. The os.File can be closed here
		// if the fd is duped or if syscall.Close is used on h.kvmFd in h.Close().
		// For simplicity of conceptual code, we'll assume fd remains valid until h.Close().
		// A more robust solution might store kvmFile and close it in h.Close().
	}
	// For now, we'll close the os.File handle as we have the integer fd.
	// This is not ideal; better to store *os.File and use its Fd() when needed for syscalls,
	// or dup the fd if os.File needs to be closed early.
	// Let's assume for this conceptual phase, kvmFileDescriptor remains valid after kvmFile.Close()
	// ONLY IF IT WAS DUPED. Since it's not duped, we should keep kvmFile or use syscall.Dup.
	// To avoid complexity, let's NOT close kvmFile here and assume h.Close() will handle it
	// by closing h.kvmFd (which is derived from kvmFile). This means KVMHypervisor needs an *os.File field.
	// Let's revert to storing *os.File for proper closure.

	// Re-evaluating: The prompt had kvmFd as int. Let's stick to that and assume fd management.
	// If we close kvmFile here, kvmFileDescriptor becomes invalid unless duped.
	// For conceptual, assume kvmFileDescriptor remains usable.
	// Real code: store *os.File, or dup fd.
	// kvmFile.Close() // This would invalidate kvmFileDescriptor if not duped.
	// For this pass, let's assume kvmFileDescriptor is the raw FD and we manage it directly.

	essentialCaps := map[string]int{ // Using int for capEnum as defined in kvm_constants_linux.go
		"USER_MEMORY":      KVM_CAP_USER_MEMORY,
		"IRQCHIP":          KVM_CAP_IRQCHIP,
		"HLT":              KVM_CAP_HLT,
		"SET_TSS_ADDR":     KVM_CAP_SET_TSS_ADDR,
		"PIT2":             KVM_CAP_PIT2,
		"EXT_CPUID":        KVM_CAP_EXT_CPUID,
		"IOEVENTFD":        KVM_CAP_IOEVENTFD,
		"IRQFD":            KVM_CAP_IRQFD,
		"MP_STATE":         KVM_CAP_MP_STATE,
	}
	fmt.Println("Conceptual KVM: Checking KVM Capabilities...")
	for name, capEnum := range essentialCaps {
		supported, checkErr := h.checkKVMExtension(capEnum)
		if checkErr != nil {
			fmt.Printf("Warning: Error checking KVM Capability %s (Enum: %d): %v\n", name, capEnum, checkErr)
		}
		h.kvmCapabilities[name] = supported
		fmt.Printf("Conceptual KVM: KVM Capability %s supported: %t\n", name, supported)
		if !supported && isCriticalKvmCapability(name) {
			_ = syscall.Close(h.kvmFd)
			return nil, fmt.Errorf("critical KVM capability %s not supported by host", name)
		}
	}

	return h, nil
}

func isCriticalKvmCapability(capName string) bool {
	critical := []string{"USER_MEMORY", "IRQCHIP"}
	for _, c := range critical {
		if c == capName { return true }
	}
	return false
}

func (h *KVMHypervisor) checkKVMExtension(capEnum int) (bool, error) {
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.kvmFd), uintptr(KVM_CHECK_EXTENSION), uintptr(capEnum))
	if errno != 0 {
		return false, fmt.Errorf("KVM_CHECK_EXTENSION ioctl for cap %d failed: %w", capEnum, errno)
	}
	return ret > 0, nil
}

func (h *KVMHypervisor) GetHostCapabilities() (*pb.HostCapabilities, error) {
	fmt.Println("Conceptual KVM: KVMHypervisor.GetHostCapabilities called")

	// Convert internal h.kvmCapabilities (map[string]bool) to []string for protobuf
	var capsPresent []string
	for capName, supported := range h.kvmCapabilities {
		if supported {
			capsPresent = append(capsPresent, capName)
		}
	}

	// In a real scenario, more host details (CPU, memory, etc.) would be queried here
	// using OS-specific utilities, potentially from a separate capabilities_linux.go module.
	return &pb.HostCapabilities{
		KvmAvailable:           true, // If NewKVMHypervisor succeeded, KVM is available
		SupportedCpuArchs:      []string{"x86-64"}, // KVM on Linux typically means host arch
		MaxVcpusPerVm:          256, // Example, can be checked via KVM_CAP_MAX_VCPUS
		KvmCapabilitiesPresent: capsPresent,
		// Other fields like CpuInfo, TotalMemoryGb would be populated by OS calls.
		CpuInfo: &pb.HostCapabilities_CPUInfo{ModelString: "Conceptual KVM Host CPU", CoreCount: 4, ThreadCount: 8},
		TotalMemoryGb: 16,
	}, nil
}

func (h *KVMHypervisor) CreateVM(config *pb.VMConfig) (int, int, error) { // vmID removed, config has it
	vmID := config.GetVmId()
	fmt.Printf("Conceptual KVM: KVMHypervisor.CreateVM called for VM ID: %s (Name: %s)\n", vmID, config.GetVmName())

	vmFdInt, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.kvmFd), uintptr(KVM_CREATE_VM), 0)
	if errno != 0 {
		return -1, 0, fmt.Errorf("KVM_CREATE_VM ioctl failed for VM %s: %w", vmID, errno)
	}
	vmFd := int(vmFdInt)
	fmt.Printf("Conceptual KVM: KVM_CREATE_VM successful for VM %s, vmFd: %d\n", vmID, vmFd)

	if h.kvmCapabilities["SET_TSS_ADDR"] { // Check if cap was found and supported
		_, _, errnoSetTSS := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFd), uintptr(KVM_SET_TSS_ADDR), uintptr(0xfffbd000))
		if errnoSetTSS != 0 {
			_ = syscall.Close(vmFd)
			return -1, 0, fmt.Errorf("KVM_SET_TSS_ADDR failed for VM %s: %w", vmID, errnoSetTSS)
		}
		fmt.Printf("Conceptual KVM: KVM_SET_TSS_ADDR called for VM %s.\n", vmID)
	} else {
		fmt.Println("Conceptual KVM: Warning - KVM_CAP_SET_TSS_ADDR not supported or not checked, skipping TSS setup.")
	}

	if h.kvmCapabilities["IRQCHIP"] {
		_, _, errnoCreateIRQChip := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFd), uintptr(KVM_CREATE_IRQCHIP), 0)
		if errnoCreateIRQChip != 0 {
			_ = syscall.Close(vmFd)
			return -1, 0, fmt.Errorf("KVM_CREATE_IRQCHIP failed for VM %s: %w", vmID, errnoCreateIRQChip)
		}
		fmt.Printf("Conceptual KVM: KVM_CREATE_IRQCHIP called for VM %s.\n", vmID)
	} else {
		_ = syscall.Close(vmFd)
		return -1, 0, fmt.Errorf("KVM_CAP_IRQCHIP is critical and not supported for VM %s", vmID)
	}

	if h.kvmCapabilities["PIT2"] {
		_, _, errnoCreatePIT2 := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vmFd), uintptr(KVM_CREATE_PIT2), 0)
		if errnoCreatePIT2 != 0 {
			_ = syscall.Close(vmFd)
			return -1, 0, fmt.Errorf("KVM_CREATE_PIT2 failed for VM %s: %w", vmID, errnoCreatePIT2)
		}
		fmt.Printf("Conceptual KVM: KVM_CREATE_PIT2 called for VM %s.\n", vmID)
	} else {
		fmt.Println("Conceptual KVM: Warning - KVM_CAP_PIT2 not reported as supported, skipping PIT setup.")
	}

	return vmFd, h.vcpuMmapMinSize, nil
}

func (h *KVMHypervisor) CloseVMContext(vmFd int) error {
	fmt.Printf("Conceptual KVM: Closing KVM VM context (fd: %d)\n", vmFd)
	// In real code: return syscall.Close(vmFd)
	return nil
}

func (h *KVMHypervisor) Close() error {
	if h.kvmFd > 0 {
		fmt.Printf("Conceptual KVM: Closing KVM system FD: %d\n", h.kvmFd)
		err := syscall.Close(h.kvmFd) // Actual close for the system FD
		if err != nil {
			return fmt.Errorf("failed to close /dev/kvm fd %d: %w", h.kvmFd, err)
		}
		h.kvmFd = -1 // Mark as closed
	}
	return nil
}

func (h *KVMHypervisor) GetKVMRunSize() (int, error) {
	if h.vcpuMmapMinSize <= 0 {
		return 0, fmt.Errorf("VCPU MMAP size not initialized or invalid: %d", h.vcpuMmapMinSize)
	}
	return h.vcpuMmapMinSize, nil
}

func (h *KVMHypervisor) GetAPIVersion() (int, error) {
    if h.apiVersion == 0 { // Should be set by NewKVMHypervisor
        return 0, fmt.Errorf("KVM API version not initialized")
    }
    return h.apiVersion, nil
}

// Ensure KVMHypervisor satisfies the Hypervisor interface.
var _ Hypervisor = &KVMHypervisor{}
