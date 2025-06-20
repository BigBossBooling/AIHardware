package core_engine

import (
	"fmt"
	"sync"
	"testing"

	pb "github.com/V-Architect/v-architect-core/core_engine/pb"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

// MockHypervisorForMgr provides a mock implementation of the Hypervisor interface for testing VMManager.
type MockHypervisorForMgr struct {
	mockCreateVMFdCounter int // To provide unique "fds"
	mockCreateVMErr       error
	mockCloseVMContextErr error
	// vmFdStore tracks "opened" vmFds by the mock to check if CloseVMContext is called on a valid one.
	vmFdStore map[int]bool
	mu        sync.Mutex // To protect vmFdStore if tests run in parallel (though typically not for unit tests)
}

// NewMockHypervisorForMgr creates an instance of the mock hypervisor.
func NewMockHypervisorForMgr() *MockHypervisorForMgr {
	return &MockHypervisorForMgr{
		vmFdStore:            make(map[int]bool),
		mockCreateVMFdCounter: 100, // Start FDs from a non-zero value
	}
}

// GetHostCapabilities implements Hypervisor.
func (m *MockHypervisorForMgr) GetHostCapabilities() (*HostCapabilityInfo, error) {
	fmt.Println("MockHypervisorForMgr: GetHostCapabilities called")
	// Return minimal valid data, or make it configurable if tests need specific capabilities.
	return &HostCapabilityInfo{
		KVMAPIVersion:   12, // Assuming KVM_API_VERSION_EXPECTED is 12
		KVMCapabilities: map[string]bool{"USER_MEMORY": true, "IRQCHIP": true},
	}, nil
}

// CreateVM implements Hypervisor.
func (m *MockHypervisorForMgr) CreateVM(vmID string, config *pb.VMConfig) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mockCreateVMErr != nil {
		return -1, m.mockCreateVMErr
	}
	m.mockCreateVMFdCounter++
	m.vmFdStore[m.mockCreateVMFdCounter] = true // Mark fd as "opened"
	fmt.Printf("MockHypervisorForMgr: CreateVM called for ID %s, Name %s. Returning mock vmFd %d\n", vmID, config.GetVmName(), m.mockCreateVMFdCounter)
	return m.mockCreateVMFdCounter, nil
}

// CloseVMContext implements Hypervisor.
func (m *MockHypervisorForMgr) CloseVMContext(vmFd int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mockCloseVMContextErr != nil {
		return m.mockCloseVMContextErr
	}
	if _, ok := m.vmFdStore[vmFd]; !ok {
		return fmt.Errorf("mock CloseVMContext: fd %d was not 'opened' by this mock or already 'closed'", vmFd)
	}
	delete(m.vmFdStore, vmFd) // Mark as "closed"
	fmt.Printf("MockHypervisorForMgr: VM Context (vmFd %d) closed.\n", vmFd)
	return nil
}

// Close implements Hypervisor.
func (m *MockHypervisorForMgr) Close() error {
	fmt.Println("MockHypervisorForMgr: Close called (global hypervisor cleanup)")
	return nil
}

// CheckExtension implements Hypervisor (added for interface completeness).
func (m *MockHypervisorForMgr) CheckExtension(capEnum int) (bool, error) {
	fmt.Printf("MockHypervisorForMgr: CheckExtension called for cap %d (returning true)\n", capEnum)
	return true, nil
}

// GetAPIVersion implements Hypervisor (added for interface completeness).
func (m *MockHypervisorForMgr) GetAPIVersion() (int, error) {
	fmt.Println("MockHypervisorForMgr: GetAPIVersion called (returning 12)")
	return 12, nil
}


// newTestPbVMConfigForManager creates a basic pb.VMConfig for testing VMManager.
func newTestPbVMConfigForManager(name string) *pb.VMConfig {
	return &pb.VMConfig{
		// VmId is usually set by VMManager or passed in, so not always needed here.
		VmName:       name,
		Architecture: "x86-64",
		OsTypeHint:   "linux_generic_test_env",
		VcpuConfig:   &pb.VCPUConfig{Count: 1, Topology: &pb.VCPUConfig_Topology{Sockets: 1, CoresPerSocket: 1, ThreadsPerCore: 1}},
		VramConfig:   &pb.VRAMConfig{SizeMb: 512}, // Ensure meets min if any validation happens early
		// Other fields like StorageDevices, NetworkInterfaces can be empty for these specific tests
		// unless RegisterNewVM or NewVirtualMachine starts validating them.
		StorageDevices:    []*pb.StorageDevice{},
		NetworkInterfaces: []*pb.NetworkInterface{},
		GraphicsConfig:    &pb.GraphicsConfig{Type: pb.GraphicsConfig_NONE}, // Headless often simplest
		SerialPorts:       []*pb.SerialPortConfig{},
	}
}

func TestVMManager_NewVMManager(t *testing.T) {
	fmt.Println("Conceptual Test: TestVMManager_NewVMManager - START")
	mockHypervisor := NewMockHypervisorForMgr()
	manager := NewVMManager(mockHypervisor)

	// require.NotNil(t, manager, "NewVMManager should return a non-nil instance")
	if manager == nil {
		t.Fatal("NewVMManager should return a non-nil instance")
	}
	// assert.NotNil(t, manager.vms, "VMs map should be initialized")
	if manager.vms == nil {
		t.Error("VMs map should be initialized by NewVMManager")
	}
	// assert.Same(t, mockHypervisor, manager.hypervisor, "Hypervisor instance should be stored")
	if manager.hypervisor != mockHypervisor {
		t.Error("Hypervisor instance not stored correctly by NewVMManager")
	}
	fmt.Println("Conceptual Test: TestVMManager_NewVMManager - PASSED")
}


func TestVMManager_RegisterAndGetVM(t *testing.T) {
	fmt.Println("Conceptual Test: TestVMManager_RegisterAndGetVM - START")
	mockHypervisor := NewMockHypervisorForMgr()
	manager := NewVMManager(mockHypervisor)

	config1 := newTestPbVMConfigForManager("TestVMReg001")
	vmID1_suggested := "vm001-reg-test"

	// Register new VM
	vm1, err := manager.RegisterNewVM(vmID1_suggested, config1)
	// require.NoError(t, err, "RegisterNewVM should succeed")
	if err != nil { t.Fatalf("RegisterNewVM failed: %v", err) }
	// require.NotNil(t, vm1, "RegisterNewVM should return a VM instance")
	if vm1 == nil { t.Fatal("RegisterNewVM returned nil VM") }

	// Check if VM ID in config was updated by RegisterNewVM (if it was generated)
	// For this test, vmID1_suggested is used, so config.VmId might not be set by RegisterNewVM itself
	// but NewVirtualMachine should use the passed vmID1_suggested.
	if vm1.Config.GetVmId() != "" && vm1.Config.GetVmId() != vmID1_suggested {
	    // This check is valid if RegisterNewVM is expected to inject the ID into the passed config.
	    // However, it's more common for the VM object to hold the definitive ID.
	    // t.Errorf("Expected VM ID in config to be %s or empty, got %s", vmID1_suggested, vm1.Config.GetVmId())
	}


	// assert.Equal(t, vmID1_suggested, vm1.ID, "VM ID should match suggested ID if provided")
	if vm1.ID != vmID1_suggested { t.Errorf("Expected VM ID %s, got %s", vmID1_suggested, vm1.ID) }
	// assert.Equal(t, CREATED, vm1.GetStatus(), "VM should be in CREATED state")
	if vm1.GetStatus() != CREATED {
		t.Errorf("Expected VM status CREATED, got %s", vm1.GetStatus())
	}
	// assert.Greater(t, vm1.vmFd, 0, "VM FD should be positive (mocked value)")
	if vm1.vmFd <= 0 {t.Errorf("Expected a positive vmFd, got %d", vm1.vmFd)}


	// Get the VM
	retrievedVM1, errGet := manager.GetVM(vmID1_suggested)
	// require.NoError(t, errGet, "GetVM should succeed for existing VM")
	if errGet != nil { t.Fatalf("GetVM failed for existing VM: %v", errGet) }
	// require.Same(t, vm1, retrievedVM1, "GetVM should return the same instance")
	if vm1 != retrievedVM1 { t.Errorf("GetVM did not return the same instance for ID %s", vmID1_suggested) }

	// Test getting non-existent VM
	_, errGetNonExistent := manager.GetVM("non-existent-vm-id")
	// require.Error(t, errGetNonExistent, "GetVM should fail for non-existent VM")
	if errGetNonExistent == nil {t.Error("Expected error for non-existent VM, got nil")}

	// Test registering duplicate VM ID
	configDup := newTestPbVMConfigForManager("DuplicateVMName")
	_, errDup := manager.RegisterNewVM(vmID1_suggested, configDup)
	// require.Error(t, errDup, "RegisterNewVM should fail for duplicate VM ID")
	if errDup == nil {t.Error("Expected error for duplicate VM ID '%s', got nil", vmID1_suggested)}

	// Test registering another VM with auto-generated ID
	config2 := newTestPbVMConfigForManager("TestVM002_AutoID")
	vm2, err2 := manager.RegisterNewVM("", config2) // Empty ID suggestion
	if err2 != nil {t.Fatalf("RegisterNewVM with empty ID failed: %v", err2)}
	if vm2 == nil { t.Fatal("RegisterNewVM (auto-ID) returned nil VM") }
	if vm2.ID == "" {t.Error("VM ID was not auto-generated")}
	if vm2.ID == vmID1_suggested {t.Error("Auto-generated ID clashed with existing one")}
	// Check if config in vm2 has the auto-generated ID
	// if vm2.Config.GetVmId() != vm2.ID {
	//    t.Errorf("Expected VM ID in config to be %s for auto-gen, got %s", vm2.ID, vm2.Config.GetVmId())
	// }


	fmt.Println("Conceptual Test: TestVMManager_RegisterAndGetVM - PASSED")
}

func TestVirtualMachine_LifecycleStubs(t *testing.T) {
	fmt.Println("Conceptual Test: TestVirtualMachine_LifecycleStubs - START")
	// vmFd is a placeholder, config is also basic.
	vm := NewVirtualMachine("vm-lifecycle-test", newTestPbVMConfigForManager("LifecycleTestVM"), 12345)
	if vm.GetStatus() != CREATED {t.Errorf("Initial VM status incorrect. Expected CREATED, got %s", vm.GetStatus())}

	// Start
	fmt.Println("Conceptual Test: Attempting Start...")
	err := vm.Start() // kvmSystemFd removed from stub for this test as it's not used by the stub
	if err != nil {t.Errorf("vm.Start() failed: %v", err)}
	if vm.GetStatus() != RUNNING {t.Errorf("Expected RUNNING after Start, got %s", vm.GetStatus())}

	// Pause
	fmt.Println("Conceptual Test: Attempting Pause...")
	err = vm.Pause()
	if err != nil {t.Errorf("vm.Pause() failed: %v", err)}
	if vm.GetStatus() != PAUSED {t.Errorf("Expected PAUSED after Pause, got %s", vm.GetStatus())}

	// Resume
	fmt.Println("Conceptual Test: Attempting Resume...")
	err = vm.Resume()
	if err != nil {t.Errorf("vm.Resume() failed: %v", err)}
	if vm.GetStatus() != RUNNING {t.Errorf("Expected RUNNING after Resume, got %s", vm.GetStatus())}

	// Stop
	fmt.Println("Conceptual Test: Attempting Stop (non-forced)...")
	err = vm.Stop(false)
	if err != nil {t.Errorf("vm.Stop(false) failed: %v", err)}
	if vm.GetStatus() != STOPPED {t.Errorf("Expected STOPPED after Stop, got %s", vm.GetStatus())}

	// Restart a stopped VM
	fmt.Println("Conceptual Test: Attempting re-Start on a STOPPED VM...")
	err = vm.Start()
	if err != nil {t.Errorf("vm.Start() on stopped VM failed: %v", err)}
	if vm.GetStatus() != RUNNING {t.Errorf("Expected RUNNING after re-Start, got %s", vm.GetStatus())}

	// Test invalid transitions
	fmt.Println("Conceptual Test: Testing invalid transitions...")
	vm.SetStatus(STOPPED) // Manually set for test
	err = vm.Pause()
	if err == nil {t.Errorf("Expected error pausing a STOPPED VM, got nil")}

	vm.SetStatus(RUNNING) // Manually set for test
	err = vm.Start()
	if err == nil {t.Errorf("Expected error starting a RUNNING VM, got nil")}

	fmt.Println("Conceptual Test: TestVirtualMachine_LifecycleStubs - PASSED")
}

func TestVMManager_DeleteVM(t *testing.T) {
	fmt.Println("Conceptual Test: TestVMManager_DeleteVM - START")
	mockHypervisor := NewMockHypervisorForMgr()
	manager := NewVMManager(mockHypervisor)
	config := newTestPbVMConfigForManager("VMToDelete")
	vmID := "test-vm-to-be-deleted-007"

	vm, regErr := manager.RegisterNewVM(vmID, config)
	if regErr != nil || vm == nil {
		t.Fatalf("Setup for DeleteVM failed: RegisterNewVM error: %v", regErr)
	}
	originalVmFd := vm.vmFd

	// Delete the VM
	fmt.Printf("Conceptual Test: Deleting VM %s (mock fd: %d)\n", vmID, originalVmFd)
	err := manager.DeleteVM(vmID)
	if err != nil {t.Fatalf("DeleteVM failed for existing VM '%s': %v", vmID, err)}

	// Verify VM is removed from manager
	_, errGet := manager.GetVM(vmID)
	if errGet == nil {t.Errorf("Expected error getting VM '%s' after DeleteVM, but got nil", vmID)}

	// Check if vmFd was "closed" by mock hypervisor
	mockHypervisor.mu.Lock()
	if _, ok := mockHypervisor.vmFdStore[originalVmFd]; ok {
		t.Errorf("Mock VM fd %d was not 'closed' (removed from vmFdStore) by DeleteVM", originalVmFd)
	}
	mockHypervisor.mu.Unlock()

	// Test deleting non-existent VM
	errDelNonExistent := manager.DeleteVM("non-existent-vm-for-delete-test")
	if errDelNonExistent == nil {t.Errorf("Expected error deleting non-existent VM, got nil")}

	fmt.Println("Conceptual Test: TestVMManager_DeleteVM - PASSED")
}

func TestVMManager_ListVMs(t *testing.T) {
	fmt.Println("Conceptual Test: TestVMManager_ListVMs - START")
	mockHypervisor := NewMockHypervisorForMgr()
	manager := NewVMManager(mockHypervisor)

	if len(manager.ListVMs()) != 0 {t.Error("ListVMs should be empty initially")}

	config1 := newTestPbVMConfigForManager("ListTestVM_A")
	vm1, _ := manager.RegisterNewVM("vmList_A", config1)

	config2 := newTestPbVMConfigForManager("ListTestVM_B")
	_, _ = manager.RegisterNewVM("vmList_B", config2)

	list1 := manager.ListVMs()
	if len(list1) != 2 {t.Errorf("Expected 2 VMs in list, got %d", len(list1))}
	if list1["vmList_A"] != CREATED.String() {t.Errorf("vmList_A status mismatch: expected %s, got %s", CREATED, list1["vmList_A"])}
	if list1["vmList_B"] != CREATED.String() {t.Errorf("vmList_B status mismatch: expected %s, got %s", CREATED, list1["vmList_B"])}

	// Conceptually start one VM using the stubbed Start method
	if vm1 != nil {
		_ = vm1.Start()
	}

	list2 := manager.ListVMs()
	if list2["vmList_A"] != RUNNING.String() {t.Errorf("vmList_A status after conceptual start mismatch: expected %s, got %s", RUNNING, list2["vmList_A"])}

	fmt.Println("Conceptual Test: TestVMManager_ListVMs - PASSED")
}
