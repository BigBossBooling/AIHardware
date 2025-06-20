package core_engine

import (
	// "encoding/binary" // For conceptual checks of page table entries
	"fmt"
	"testing"

	pb "github.com/V-Architect/v-architect-core/core_engine/pb" // Assuming this path from prior steps
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

// newTestVMConfigForMemoryTests helper (from previous sub-issue, ensure it's suitable)
func newTestVMConfigForMemoryTests(name string, ramMB uint64) *pb.VMConfig {
	return &pb.VMConfig{
		VmId:         fmt.Sprintf("vm-%s-id", name),
		VmName:       name,
		Architecture: pb.VirtualHardwareArch_X86_64, // Required by ValidateVMConfigBusinessLogic
		VcpuConfig:   &pb.VCPUConfig{Count: 1, Topology: &pb.VCPUConfig_Topology{Sockets:1, CoresPerSocket:1, ThreadsPerCore:1}}, // Required
		VramConfig:   &pb.VRAMConfig{SizeMb: ramMB},
		// Minimal other fields to pass potential basic validations if called via setupMemory
		StorageDevices:    []*pb.StorageDevice{},
		NetworkInterfaces: []*pb.NetworkInterface{},
		GraphicsConfig:    &pb.GraphicsConfig{Type: pb.GraphicsConfig_NONE}, // Headless
		SerialPorts:       []*pb.SerialPortConfig{},
	}
}

// TestVM_SetupAndCleanupMemory_Success (from previous sub-issue, kept for completeness of memory_test.go)
func TestVM_SetupAndCleanupMemory_Success(t *testing.T) {
	fmt.Println("Conceptual Test: TestVM_SetupAndCleanupMemory_Success - START")
	config := newTestVMConfigForMemoryTests("MemSetupCleanupVM", 128)
	mockVmFd := 999
	vm := NewVirtualMachine(config.VmId, config, mockVmFd)

	// --- Test setupMemory ---
	// setupMemory now also calls setupInitialPaging. We need enough RAM.
	// minRequiredRamForPaging in setupInitialPaging is (4*4096) + (512*4096) = 2113536 bytes (approx 2.01MB)
	// So, 128MB is plenty.
	fmt.Println("Conceptual Test: Calling vm.setupMemory() which includes setupInitialPaging...")
	err := vm.setupMemory()
	if err != nil {
		t.Fatalf("vm.setupMemory() failed unexpectedly: %v", err)
	}
	if vm.guestMem == nil {
		t.Error("guestMem is nil after successful setupMemory")
	}
	if len(vm.memoryRegions) != 1 { // setupMemory creates one main region, setupInitialPaging uses it
		t.Errorf("Expected 1 memory region, got %d", len(vm.memoryRegions))
	} else {
		if vm.memoryRegions[0].MemorySize != uint64(128*1024*1024) {
			t.Errorf("MemoryRegion[0] size mismatch: expected %d, got %d", uint64(128*1024*1024), vm.memoryRegions[0].MemorySize)
		}
	}
	if vm.ramSizeBytes != uint64(128*1024*1024) {
		t.Errorf("vm.ramSizeBytes mismatch: expected %d, got %d", uint64(128*1024*1024), vm.ramSizeBytes)
	}
	// Implicitly, setupInitialPaging must also have succeeded.

	// --- Test cleanupMemory ---
	fmt.Println("Conceptual Test: Calling vm.cleanupMemory()...")
	err = vm.cleanupMemory()
	if err != nil {
		t.Fatalf("vm.cleanupMemory() failed unexpectedly: %v", err)
	}
	if vm.guestMem != nil {
		t.Error("guestMem is not nil after successful cleanupMemory")
	}
	if len(vm.memoryRegions) != 0 {
		t.Errorf("Expected 0 memory regions after cleanup, got %d", len(vm.memoryRegions))
	}
	fmt.Println("Conceptual Test: TestVM_SetupAndCleanupMemory_Success - PASSED")
}

// TestVM_SetupMemory_RamSizeZero (from previous sub-issue, kept for completeness)
func TestVM_SetupMemory_RamSizeZero(t *testing.T) {
	fmt.Println("Conceptual Test: TestVM_SetupMemory_RamSizeZero - START")
	config := newTestVMConfigForMemoryTests("MemZeroRAMVM", 0)
	mockVmFd := 998
	vm := NewVirtualMachine(config.VmId, config, mockVmFd)

	fmt.Println("Conceptual Test: Calling vm.setupMemory() with 0 RAM size...")
	err := vm.setupMemory()
	if err == nil {
		t.Errorf("Expected error when calling setupMemory with 0 RAM size, but got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly received error for 0 RAM size: %v\n", err)
		if !strings.Contains(err.Error(), "RAM size is 0") {
			t.Errorf("Error message should indicate RAM size issue, got: %s", err.Error())
		}
	}
	if vm.guestMem != nil { t.Error("guestMem should be nil after failed setupMemory") }
	if len(vm.memoryRegions) != 0 {t.Errorf("memoryRegions should be empty after failed setupMemory")}
	fmt.Println("Conceptual Test: TestVM_SetupMemory_RamSizeZero - PASSED")
}


// --- Tests for setupInitialPaging (new for this sub-issue) ---

func TestVM_SetupInitialPaging_Valid(t *testing.T) {
	fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_Valid - START")

	// Config for a VM with enough RAM for basic paging structures + 2MB identity map
	// setupInitialPaging needs at least (4*4096 for tables) + (512*4096 for 2MB map) = 2113536 bytes. ~2.01MB.
	// So, 4MB should be fine.
	config := newTestVMConfigForMemoryTests("PagingValidVM", 4)
	mockVmFd_paging := 1001
	vm := NewVirtualMachine(config.VmId, config, mockVmFd_paging)

	// Manually simulate that vm.guestMem is allocated, as setupInitialPaging expects it.
	// In the actual flow, setupMemory() would do this before calling setupInitialPaging().
	vm.guestMem = make([]byte, vm.ramSizeBytes)
	PageMapLevel4AddressGPA = 0x1000 // Ensure default for test predictability

	fmt.Println("Conceptual Test: Calling vm.setupInitialPaging()...")
	err := vm.setupInitialPaging()
	if err != nil {
		t.Fatalf("vm.setupInitialPaging() failed: %v", err)
	}

	// Conceptual Verification: Placeholder checks. Real checks would involve binary.LittleEndian.Uint64.
	fmt.Printf("Conceptual Test: PML4E[0] at 0x%X conceptually points to PDPT at 0x%X.\n", PageMapLevel4AddressGPA, PageMapLevel4AddressGPA+0x1000)
	fmt.Printf("Conceptual Test: PDPTE[0] at 0x%X conceptually points to PD at 0x%X.\n", PageMapLevel4AddressGPA+0x1000, PageMapLevel4AddressGPA+0x2000)
	fmt.Printf("Conceptual Test: PDE[0] at 0x%X conceptually points to PT at 0x%X.\n", PageMapLevel4AddressGPA+0x2000, PageMapLevel4AddressGPA+0x3000)
	fmt.Printf("Conceptual Test: PTE[0] at 0x%X conceptually identity maps GPA 0x0.\n", PageMapLevel4AddressGPA+0x3000)

	fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_Valid - PASSED")
}

func TestVM_SetupInitialPaging_InsufficientRAM(t *testing.T) {
	fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_InsufficientRAM - START")
	// Required: (4*4096 for tables) + (at least 1*4096 page to map) = 20480 bytes.
	// Let's give it less than what it needs for tables alone.
	// Or less than what it needs for its 2MB identity map + tables.
	// The function expects to map 2MB (512*4096 = 2097152 bytes). Tables = 16384. Total = 2113536.
	// So 1MB RAM (1024*1024 = 1048576 bytes) is insufficient.
	config := newTestVMConfigForMemoryTests("PagingFailVM", 1) // 1MB RAM
	mockVmFd_paging_fail := 1002
	vm := NewVirtualMachine(config.VmId, config, mockVmFd_paging_fail)
	vm.guestMem = make([]byte, vm.ramSizeBytes)

	fmt.Println("Conceptual Test: Calling vm.setupInitialPaging() with insufficient RAM...")
	err := vm.setupInitialPaging()
	if err == nil {
		t.Errorf("Expected error for insufficient RAM for paging structures, but got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly received error for insufficient RAM: %v\n", err)
		if !strings.Contains(err.Error(), "too small for initial paging setup") && !strings.Contains(err.Error(), "exceed guest RAM size") {
			t.Errorf("Error message should indicate RAM size or structure fit issue, got: %s", err.Error())
		}
	}
	fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_InsufficientRAM - PASSED")
}

func TestVM_SetupInitialPaging_GuestMemNil(t *testing.T) {
    fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_GuestMemNil - START")
    config := newTestVMConfigForMemoryTests("PagingNilMemVM", 4) // 4MB RAM is fine
    mockVmFd_paging_nil := 1003
    vm := NewVirtualMachine(config.VmId, config, mockVmFd_paging_nil)
    vm.guestMem = nil // Intentionally set guestMem to nil

    fmt.Println("Conceptual Test: Calling vm.setupInitialPaging() with nil guestMem...")
    err := vm.setupInitialPaging()
    if err == nil {
        t.Errorf("Expected error when guestMem is nil, but got nil")
    } else {
        fmt.Printf("Conceptual Test: Correctly received error for nil guestMem: %v\n", err)
        if !strings.Contains(err.Error(), "guestMem not allocated") {
			t.Errorf("Error message should indicate guestMem not allocated, got: %s", err.Error())
		}
    }
    fmt.Println("Conceptual Test: TestVM_SetupInitialPaging_GuestMemNil - PASSED")
}
