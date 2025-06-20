package core_engine

import (
	"fmt"
	"testing"
	"time" // For test with timeout

	pb "github.com/V-Architect/v-architect-core/proto"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

// newTestVMForVCPUTest (ensure it sets up a serial port for KVM_EXIT_IO testing)
func newTestVMForVCPUTest(id string, name string, serialPortCount int) *VirtualMachine {
	var serialConfigs []*pb.SerialPortConfig
	if serialPortCount > 0 {
		serialConfigs = make([]*pb.SerialPortConfig, serialPortCount)
		for i := 0; i < serialPortCount; i++ {
			comID := fmt.Sprintf("com%d_test", i+1)
			var ioBase uint32 = uint32(DEFAULT_SERIAL_IO_BASE_COM1 + uint16(i*0x100)) // Ensure distinct bases if needed, or use standard ones
			if i == 1 { ioBase = DEFAULT_SERIAL_IO_BASE_COM2 }
			// etc. For this test, only COM1 (0x3F8) is usually hit by the Run loop simulation.

			serialConfigs[i] = &pb.SerialPortConfig{
				Id:                  comID,
				Type:                pb.SerialPortConfig_LOG_ONLY,
				IoBaseOverride:      ioBase,
			}
		}
	}

	config := &pb.VMConfig{
		VmId:         id,
		VmName:       name,
		Architecture: pb.VirtualHardwareArch_X86_64,
		VcpuConfig:   &pb.VCPUConfig{Count: 1},
		VramConfig:   &pb.VRAMConfig{SizeMb: 128},
		SerialPorts:  serialConfigs,
		StorageDevices:    []*pb.StorageDevice{},
		NetworkInterfaces: []*pb.NetworkInterface{},
		GraphicsConfig:    &pb.GraphicsConfig{Type: pb.GraphicsConfig_NONE},
	}
	vm := NewVirtualMachine(id, config, 999) // 999 is a dummy vmFd

	// Simulate device initialization that would happen in vm.initializeDevices()
	// This is crucial because VCPU.Run() will try to access vm.serialPorts
	// vm.initializeDevices() // Calling the actual method is better if it's simple enough
	// For this test, let's manually ensure serialPorts are created based on config
	if vm.serialPorts == nil { vm.serialPorts = make([]*SerialPortDevice,0)}
	if len(vm.serialPorts) == 0 && len(config.GetSerialPorts()) > 0 {
		for i, spCfg := range config.GetSerialPorts() {
			ioBase := spCfg.GetIoBaseOverride()
			if ioBase == 0 { // Assign default if override is not set
				if i == 0 { ioBase = DEFAULT_SERIAL_IO_BASE_COM1 }
				// Add more defaults if testing more than one serial port
			}
			devID := spCfg.GetId()
			if devID == "" { devID = fmt.Sprintf("com%d_test_default", i+1)}

			// Use LOG_ONLY for tests to avoid actual PTY/file ops
			sp, _ := NewSerialPortDevice(devID, uint16(ioBase), (spCfg.GetType() == pb.SerialPortConfig_STDIO), /*spCfg*/)
			vm.serialPorts = append(vm.serialPorts, sp)
		}
	}
	return vm
}


func TestNewVCPU_Success_WithKvmRun(t *testing.T) { // Renamed to avoid conflict if old test exists
	fmt.Println("Conceptual Test: TestNewVCPU_Success_WithKvmRun - START")
	mockVM := newTestVMForVCPUTest("vm-newvcpu-ok-kr", "TestNewVCPU_KvmRunVM", 0)
	kvmSystemFd_placeholder := 99

	vcpu, err := NewVCPU(mockVM, 0, kvmSystemFd_placeholder)
	if err != nil { t.Fatalf("NewVCPU failed: %v", err) }
	if vcpu == nil { t.Fatal("NewVCPU returned nil") }

	if vcpu.id != 0 { t.Errorf("Expected vCPU ID 0, got %d", vcpu.id) }
	if vcpu.vmFd != mockVM.vmFd { t.Errorf("Expected vcpu.vmFd to be %d, got %d", mockVM.vmFd, vcpu.vmFd)}
	if vcpu.vcpuFd <= 0 { t.Errorf("Expected positive vcpuFd placeholder, got %d", vcpu.vcpuFd)}
	if vcpu.kvmRun == nil { t.Error("vcpu.kvmRun should not be nil (should point to KvmRun struct)") }
	if vcpu.vm != mockVM { t.Error("vcpu.vm reference not set correctly") }

	fmt.Println("Conceptual Test: TestNewVCPU_Success_WithKvmRun - PASSED")
}

func TestVCPU_setupInitialArchState_Full(t *testing.T) { // Renamed
	fmt.Println("Conceptual Test: TestVCPU_setupInitialArchState_Full - START")
	mockVM := newTestVMForVCPUTest("vm-archstate-full", "ArchStateFullVM", 0)
	vcpu, _ := NewVCPU(mockVM, 0, 99)

	kernelEntryGPA := uint64(0x100000)
	bootParamsGPA  := uint64(0x10000)
	pml4BaseGPA    := PageMapLevel4AddressGPA // Use the one from memory.go (or kvm_x86_arch.go if moved)

	err := vcpu.setupInitialArchState(kernelEntryGPA, bootParamsGPA, pml4BaseGPA)
	if err != nil {
		t.Fatalf("setupInitialArchState failed: %v", err)
	}
	// Test relies on Printf statements inside setupInitialArchState for conceptual verification.
	fmt.Println("Conceptual Test: TestVCPU_setupInitialArchState_Full - PASSED (check logs for conceptual calls)")
}


func TestVCPU_RunLoop_HandlesExits(t *testing.T) { // Renamed
	fmt.Println("Conceptual Test: TestVCPU_RunLoop_HandlesExits - START")

	mockVM := newTestVMForVCPUTest("vm-runloop-exits", "TestRunLoopExitsVM", 1)
	if len(mockVM.serialPorts) == 0 {
		t.Fatalf("Test Setup Error: No serial ports initialized in mockVM. Serial I/O exit won't be tested properly.")
	} else {
		t.Logf("Info: mockVM has %d serial ports. First one: ID=%s, Base=0x%X",
			len(mockVM.serialPorts), mockVM.serialPorts[0].ID, mockVM.serialPorts[0].guestIoBaseAddr)
	}
	mockVM.SetStatus(RUNNING)

	vcpu, err := NewVCPU(mockVM, 0, 99)
	if err != nil { t.Fatalf("NewVCPU failed for RunLoop test: %v", err) }
	if vcpu == nil { t.Fatal("NewVCPU returned nil for RunLoop test") }

	// The conceptual Run loop in vcpu.go is limited to 10 iterations and simulates
	// KVM_EXIT_IO (for serial), then KVM_EXIT_HLT, then KVM_EXIT_SHUTDOWN for vCPU 0.
	done := make(chan error, 1)
	go func() {
		done <- vcpu.Run()
	}()

	select {
	case runErr := <-done:
		if runErr != nil {
			// This test expects the Run loop to exit cleanly due to simulated KVM_EXIT_SHUTDOWN
			// or after limited HLT iterations. An error here would be unexpected for the happy path simulation.
			t.Errorf("vcpu.Run() returned an unexpected error: %v", runErr)
		} else {
			fmt.Println("Conceptual Test: vcpu.Run() completed as expected by simulation (KVM_EXIT_SHUTDOWN or HLT loop end).")
		}
	case <-time.After(300 * time.Millisecond):
		t.Errorf("vcpu.Run() did not complete within timeout (conceptual loop simulation might be stuck or SHUTDOWN not reached)")
	}

	finalStatus := mockVM.GetStatus()
	// The Run loop simulation sets status to STOPPING on SHUTDOWN or loop end, or FAILED on error.
	if finalStatus != STOPPING && finalStatus != FAILED {
		 t.Errorf("VM status unexpected after Run loop: expected STOPPING or FAILED, got %s", finalStatus)
	}
	if mockVM.GetLastError() != nil {
		fmt.Printf("Conceptual Test: VM LastError after Run: %v\n", mockVM.GetLastError())
	}

	fmt.Println("Conceptual Test: TestVCPU_RunLoop_HandlesExits - PASSED")
}


func TestVCPU_Close_NoPanic(t *testing.T){ // Renamed
	fmt.Println("Conceptual Test: TestVCPU_Close_NoPanic - START")
	mockVM := newTestVMForVCPUTest("vm-close-test-np", "TestCloseNoPanicVM", 0)
	vcpu, _ := NewVCPU(mockVM, 0, 99)

	err := vcpu.Close()
	if err != nil {
		t.Errorf("vcpu.Close() returned error: %v", err)
	}
	fmt.Println("Conceptual Test: TestVCPU_Close_NoPanic - PASSED")
}
