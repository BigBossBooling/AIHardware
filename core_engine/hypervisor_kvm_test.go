// +build linux

package core_engine

import (
	"fmt"
	"os" // For os.Getuid() and os.Stat() if used to skip tests
	"testing"

	pb "github.com/V-Architect/v-architect-core/proto" // Updated import path
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

// TestNewKVMHypervisor_Integration attempts to initialize KVM.
// This test WILL FAIL if KVM is not available or user lacks permissions.
// It should ideally be tagged as an integration test or run in an environment where KVM is guaranteed.
func TestNewKVMHypervisor_Integration(t *testing.T) {
	fmt.Println("Conceptual Integration Test: TestNewKVMHypervisor_Integration - START")

	// Skip if not running in an environment expected to have KVM available for actual tests
	if os.Getenv("CI_SKIP_KVM_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping KVM integration test due to CI_SKIP_KVM_INTEGRATION_TESTS=true")
		return
	}
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		t.Skipf("Skipping KVM integration test: /dev/kvm not found. KVM module likely not loaded or KVM not supported.")
		return
	}
	// A check for permissions (e.g., os.Getuid() == 0 or member of 'kvm' group) might also be warranted.

	h, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("NewKVMHypervisor() failed: %v. Ensure KVM is enabled, kernel modules (kvm, kvm_intel/kvm_amd) are loaded, and user has permissions for /dev/kvm.", err)
	}
	if h == nil {
		t.Fatal("NewKVMHypervisor() returned nil hypervisor instance despite no error.")
	}

	if h.apiVersion != KVM_API_VERSION_EXPECTED {
		t.Errorf("Expected KVM API version %d, got %d", KVM_API_VERSION_EXPECTED, h.apiVersion)
	}
	if h.vcpuMmapMinSize <= 0 {
		t.Errorf("Expected positive vcpuMmapMinSize, got %d", h.vcpuMmapMinSize)
	}

	// Check for a critical capability that should exist.
	if supported, exists := h.kvmCapabilities["USER_MEMORY"]; !exists || !supported {
		t.Errorf("KVM_CAP_USER_MEMORY not reported as supported or check missing. Capabilities: %v", h.kvmCapabilities)
	}

	errClose := h.Close()
	if errClose != nil {
		t.Errorf("h.Close() failed: %v", errClose)
	}

	fmt.Println("Conceptual Integration Test: TestNewKVMHypervisor_Integration - PASSED (conceptually, relies on host KVM setup)")
}

func TestKVMHypervisor_CreateVM_Integration(t *testing.T) {
	fmt.Println("Conceptual Integration Test: TestKVMHypervisor_CreateVM_Integration - START")

	if _, errOs := os.Stat("/dev/kvm"); os.IsNotExist(errOs) {
		t.Skipf("Skipping KVM integration test: /dev/kvm not found.")
		return
	}

	h, err := NewKVMHypervisor()
	if err != nil { t.Fatalf("NewKVMHypervisor() failed for CreateVM test: %v", err)}
	if h == nil { t.Fatal("NewKVMHypervisor() returned nil for CreateVM test") }
	defer h.Close()

	// Create a minimal dummy VMConfig for testing CreateVM.
	dummyConfig := &pb.VMConfig{
		VmId:   "test-vm-id-integration-002", // Ensure VmId is present
		VmName: "MyTestVM_Integration_Create",
		// Other fields can be default/nil as CreateVM in KVMHypervisor primarily uses vmID for logging.
		// For a more robust test, ensure config is valid enough if CreateVM starts using it.
		VcpuConfig: &pb.VCPUConfig{Count: 1},
		VramConfig: &pb.VRAMConfig{SizeMb: 128},
		Architecture: pb.VirtualHardwareArch_X86_64,
	}

	vmFd, vcpuMmapSize, errCreate := h.CreateVM(dummyConfig) // Pass config
	if errCreate != nil {
		t.Fatalf("h.CreateVM() failed: %v", errCreate)
	}
	if vmFd <= 0 {
		t.Errorf("Expected valid VM FD (>0), got %d", vmFd)
	}
	if vcpuMmapSize <= 0 {
		t.Errorf("Expected valid vcpuMmapSize (>0), got %d", vcpuMmapSize)
	}
	fmt.Printf("Conceptual Integration Test: CreateVM returned conceptual vmFd: %d, vcpuMmapSize: %d\n", vmFd, vcpuMmapSize)

	errCloseVM := h.CloseVMContext(vmFd)
	if errCloseVM != nil {
		t.Errorf("h.CloseVMContext() for vmFd %d returned error: %v", vmFd, errCloseVM)
	}

	fmt.Println("Conceptual Integration Test: TestKVMHypervisor_CreateVM_Integration - PASSED (conceptually, relies on host KVM setup)")
}

func TestKVMHypervisor_GetHostCapabilities_Integration(t *testing.T) {
    fmt.Println("Conceptual Integration Test: TestKVMHypervisor_GetHostCapabilities_Integration - START")
    if _, errOs := os.Stat("/dev/kvm"); os.IsNotExist(errOs) {
		t.Skipf("Skipping KVM integration test: /dev/kvm not found.")
		return
	}

    h, err := NewKVMHypervisor()
    if err != nil { t.Fatalf("NewKVMHypervisor() failed: %v", err) }
    if h == nil { t.Fatal("NewKVMHypervisor() returned nil") }
    defer h.Close()

    caps, errCaps := h.GetHostCapabilities()
    if errCaps != nil { t.Fatalf("GetHostCapabilities() failed: %v", errCaps) }
    if caps == nil { t.Fatal("GetHostCapabilities() returned nil") }

    if !caps.GetKvmAvailable() { // Using getter for protobuf bool field
        t.Errorf("Expected KVM to be available, but GetKvmAvailable() is false")
    }
    // Check one of the KVM capabilities from the string list
    foundUserMemCap := false
    for _, capName := range caps.GetKvmCapabilitiesPresent() {
        if capName == "USER_MEMORY" {
            foundUserMemCap = true
            break
        }
    }
    if !foundUserMemCap {
        t.Errorf("Expected 'USER_MEMORY' in KvmCapabilitiesPresent, got %v", caps.GetKvmCapabilitiesPresent())
    }
    // assert.NotEmpty(t, caps.GetCpuInfo().GetModelString(), "CPUInfo.ModelString should not be empty")
    if caps.GetCpuInfo().GetModelString() == "" {
		t.Error("CPUInfo.ModelString is empty in capabilities")
	}


    fmt.Println("Conceptual Integration Test: TestKVMHypervisor_GetHostCapabilities_Integration - PASSED (basic checks on conceptual data)")
}

func TestKVMHypervisor_GetKVMRunSize_Integration(t *testing.T) {
    fmt.Println("Conceptual Integration Test: TestKVMHypervisor_GetKVMRunSize_Integration - START")
    if _, errOs := os.Stat("/dev/kvm"); os.IsNotExist(errOs) {
		t.Skipf("Skipping KVM integration test: /dev/kvm not found.")
		return
	}
    h, err := NewKVMHypervisor()
    if err != nil { t.Fatalf("NewKVMHypervisor() failed: %v", err) }
    if h == nil { t.Fatal("NewKVMHypervisor() returned nil") }
    defer h.Close()

    size, errSize := h.GetKVMRunSize()
    if errSize != nil {t.Fatalf("GetKVMRunSize() failed: %v", errSize)}
    if size <= 0 {t.Errorf("Expected positive KVM run size, got %d", size)}

    fmt.Printf("Conceptual Integration Test: GetKVMRunSize returned: %d\n", size)
    fmt.Println("Conceptual Integration Test: TestKVMHypervisor_GetKVMRunSize_Integration - PASSED")
}
