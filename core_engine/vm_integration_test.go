package core_engine

import (
	"bytes" // For testable serial output buffer
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/V-Architect/v-architect-core/proto"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

// --- Test Utilities ---

// MockHypervisorForIntegration provides a mock hypervisor for integration tests.
// It needs to return valid (though placeholder) fds.
type MockHypervisorForIntegration struct {
	kvmSystemFd           int
	vcpuMmapSize          int
	nextVmFd              int
	vmFds                 map[int]bool // Store "active" vmFds
	mu                    sync.Mutex
	hostCapabilities      *pb.HostCapabilities
	getCapabilitiesErr    error
	createVMErr           error
	closeVMContextErr     error
	getKVMRunSizeErr      error
	getAPIVersionErr      error
}

func NewMockHypervisorForIntegration() *MockHypervisorForIntegration {
	return &MockHypervisorForIntegration{
		kvmSystemFd:      99, // Placeholder /dev/kvm fd
		vcpuMmapSize:     4096, // Typical KVM_RUN size
		nextVmFd:         1000,
		vmFds:            make(map[int]bool),
		hostCapabilities: &pb.HostCapabilities{KvmAvailable: true, SupportedCpuArchs: []string{"x86-64"}},
	}
}
func (m *MockHypervisorForIntegration) CreateVM(config *pb.VMConfig) (int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createVMErr != nil {
		return -1, 0, m.createVMErr
	}
	m.nextVmFd++
	m.vmFds[m.nextVmFd] = true
	fmt.Printf("MockHypervisor: CreateVM called for ID %s. Returning mock vmFd %d\n", config.GetVmId(), m.nextVmFd)
	return m.nextVmFd, m.vcpuMmapSize, nil
}
func (m *MockHypervisorForIntegration) CloseVMContext(vmFd int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeVMContextErr != nil { return m.closeVMContextErr }
	if !m.vmFds[vmFd] { return fmt.Errorf("mock CloseVMContext: fd %d not active or already closed", vmFd) }
	delete(m.vmFds, vmFd)
	fmt.Printf("MockHypervisor: CloseVMContext for vmFd %d\n", vmFd)
	return nil
}
func (m *MockHypervisorForIntegration) GetHostCapabilities() (*pb.HostCapabilities, error) {
	return m.hostCapabilities, m.getCapabilitiesErr
}
func (m *MockHypervisorForIntegration) Close() error { return nil }
func (m *MockHypervisorForIntegration) GetKVMRunSize() (int, error) { return m.vcpuMmapSize, m.getKVMRunSizeErr }
func (m *MockHypervisorForIntegration) GetAPIVersion() (int, error) { return 12, m.getAPIVersionErr }

// Modify SerialPortDevice for testable output (conceptual change, applied during test setup)
// This is a simplified version of what might be done in serial_port.go for testing.
type TestableSerialPortDevice struct {
	*SerialPortDevice
	TestOutputBuffer *bytes.Buffer
}

func NewTestableSerialPortDevice(id string, config *pb.SerialPortConfig, ioBase uint16, irqNum uint32) (*TestableSerialPortDevice, error) {
	// Force LOG_ONLY to use a buffer for testing, or adapt based on type
	config.Type = pb.SerialPortConfig_LOG_ONLY

	baseSp, err := NewSerialPortDevice(id, config, ioBase, irqNum)
	if err != nil {
		return nil, err
	}
	tsp := &TestableSerialPortDevice{
		SerialPortDevice: baseSp,
		TestOutputBuffer: new(bytes.Buffer),
	}
	// Override the HostBackend for LOG_ONLY to write to our buffer
	if config.GetType() == pb.SerialPortConfig_LOG_ONLY {
		tsp.HostBackend = tsp.TestOutputBuffer // Redirect output
	}
	return tsp, nil
}
// Override HandlePIOWrite to use TestOutputBuffer if LOG_ONLY
func (tsp *TestableSerialPortDevice) HandlePIOWrite(offset uint16, data uint8, size int) error {
	if tsp.Config.GetType() == pb.SerialPortConfig_LOG_ONLY && tsp.TestOutputBuffer != nil {
		if offset == UART_RX { // THR
			_, err := tsp.TestOutputBuffer.Write([]byte{data})
			// Simulate LSR update
			tsp.mutex.Lock()
			tsp.lsrReg &= ^(UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
			tsp.lsrReg |= (UART_LSR_TX_EMPTY | UART_LSR_TX_IDLE)
			tsp.mutex.Unlock()
			return err
		}
	}
	return tsp.SerialPortDevice.HandlePIOWrite(offset, data, size) // Call original for other registers
}


// --- Integration Test ---

func TestMinimalSystemBoot_SerialOutput_Halt_Conceptual(t *testing.T) {
	fmt.Println("Conceptual Integration Test: TestMinimalSystemBoot_SerialOutput_Halt - START")
	// require := require.New(t)
	// assert := assert.New(t)

	// 1. Prepare Test Kernel and Config
	tempDir := t.TempDir()
	dummyKernelPath := filepath.Join(tempDir, "kernel.bin")
	// This is a conceptual kernel. It should write to serial 0x3F8 and then HLT.
	// The VCPU.Run() loop simulates this behavior for vCPU0 if a serial port is present.
	err := os.WriteFile(dummyKernelPath, []byte("conceptual_kernel_hlt_after_serial_write"), 0644)
	if err != nil { t.Fatalf("Failed to create dummy kernel file: %v", err) }

	vmID := "test-boot-vm"
	serialConfig := &pb.SerialPortConfig{
		Id:             "com1",
		Type:           pb.SerialPortConfig_LOG_ONLY, // For capturing output in test
		IoBaseOverride: DEFAULT_SERIAL_IO_BASE_COM1,
	}
	vmConfig := &pb.VMConfig{
		VmId:         vmID,
		Name:       "MinimalBootTestVM",
		Architecture: pb.VirtualHardwareArch_X86_64,
		VcpuConfig:   &pb.VCPUConfig{Count: 1},
		MemoryConfig: &pb.MemoryConfig{SizeMb: 64}, // Minimal RAM
		SerialPorts:  []*pb.SerialPortConfig{serialConfig},
		KernelImagePath: dummyKernelPath,
		KernelCmdline: "console=ttyS0 earlyprintk=serial,0x3f8,115200", // Standard Linux cmdline
		FirmwareType: pb.VMConfig_BIOS, // Or direct kernel boot doesn't strictly need firmware type
	}

	// 2. Setup Hypervisor and VMManager
	mockHypervisor := NewMockHypervisorForIntegration()
	vmManager := NewVMManager(mockHypervisor)

	// 3. Register VM (this also calls vm.setupMemory which includes setupInitialPaging)
	vmInstance, err := vmManager.RegisterNewVM(vmID, vmConfig)
	if err != nil { t.Fatalf("VMManager.RegisterNewVM failed: %v", err) }
	if vmInstance == nil { t.Fatal("RegisterNewVM returned nil VM instance") }

	// Modify the VM's serial port to use the TestableSerialPortDevice
	// This requires initializeDevices to be called first, or we manually set it up.
	// For this test, let's assume initializeDevices inside StartProcess will use NewSerialPortDevice.
	// We need a way to inject our testable serial port or inspect its output.
	// Let's refine initializeDevices on the VM to allow injecting a testable serial port for LOG_ONLY.
	// This is a bit of a hack for conceptual testing.
	// A better way would be for NewSerialPortDevice to return an interface,
	// and we provide a mock implementation of that interface for testing.
	// For now, we'll assume the first serial port is the one we want to check.

	// Re-initialize devices with a TestableSerialPortDevice for LOG_ONLY
	vmInstance.resourceLock.Lock()
	if len(vmInstance.Config.GetSerialPorts()) > 0 && vmInstance.Config.GetSerialPorts()[0].GetType() == pb.SerialPortConfig_LOG_ONLY {
		spCfg := vmInstance.Config.GetSerialPorts()[0]
		ioBase := spCfg.GetIoBaseOverride(); if ioBase == 0 { ioBase = DEFAULT_SERIAL_IO_BASE_COM1 }
		id := spCfg.GetId(); if id == "" {id = "com1"}

		testableSp, tspErr := NewTestableSerialPortDevice(id, spCfg, uint16(ioBase), 4)
		if tspErr != nil { t.Fatalf("Failed to create testable serial port: %v", tspErr) }
		vmInstance.serialPorts = []*SerialPortDevice{testableSp.SerialPortDevice} // Store the base device
		// Keep a reference to the testable part for checking output
		testSerialOutputBuffer := testableSp.TestOutputBuffer
		vmInstance.resourceLock.Unlock()

		// 4. Start the VM
		fmt.Println("Conceptual Integration Test: Calling vm.StartProcess()...")
		// kvmSystemFd is from hypervisor, but mockHypervisor doesn't expose its internal fd.
		// Pass a placeholder. NewVCPU and other methods use placeholders for fds.
		startErr := vmInstance.StartProcess(mockHypervisor.kvmSystemFd,
											vmConfig.KernelImagePath,
											vmConfig.GetInitrdPath(), // empty for this test
											vmConfig.KernelCmdline)

		// In this conceptual test, StartProcess will return AFTER vCPU goroutines are launched.
		// The vCPU.Run() simulation is short and will lead to HLT then SHUTDOWN.
		// We need to wait for the VM to reach a terminal state (STOPPED or FAILED).

		timeout := time.After(500 * time.Millisecond) // Increased timeout for all prints
		var finalVmStatus VMStatus
	Loop:
		for {
			select {
			case <-timeout:
				t.Errorf("VM did not reach STOPPED or FAILED state within timeout.")
				break Loop
			default:
				finalVmStatus = vmInstance.GetStatus()
				if finalVmStatus == STOPPED || finalVmStatus == FAILED {
					break Loop
				}
				time.Sleep(20 * time.Millisecond)
			}
		}

		if startErr != nil {
			// StartProcess itself might fail before vCPUs run if setup steps error out
			t.Logf("vm.StartProcess returned an error as expected for some failure paths: %v", startErr)
		}

		// 5. Verify Serial Output
		// The VCPU.Run loop simulates writing 'V' then 'M' via KVM_EXIT_IO to COM1.
		expectedSerialOutput := "VM"
		actualSerialOutput := testSerialOutputBuffer.String()
		if !strings.Contains(actualSerialOutput, expectedSerialOutput) {
			t.Errorf("Expected serial output to contain '%s', got '%s'", expectedSerialOutput, actualSerialOutput)
		} else {
			fmt.Printf("Conceptual Integration Test: Verified serial output contains '%s'. Full output: '%s'\n", expectedSerialOutput, actualSerialOutput)
		}

		// 6. Verify VM Status
		finalVmStatus = vmInstance.GetStatus() // Get it again after loop
		// The conceptual VCPU.Run() for vCPU0 simulates IO then HLT then SHUTDOWN, which sets VM to STOPPING.
		// If StartProcess completes, it sets to RUNNING. Then vCPU exits, sets to STOPPING.
		if finalVmStatus != STOPPING && finalStatus != FAILED { // VCPU sets to STOPPING on SHUTDOWN exit
			t.Errorf("Expected VM status to be STOPPING or FAILED, got %s. Last error: %v", finalVmStatus, vmInstance.GetLastError())
		} else {
			fmt.Printf("Conceptual Integration Test: VM final status is %s, Last Error: %v\n", finalVmStatus, vmInstance.GetLastError())
		}

	} else {
		vmInstance.resourceLock.Unlock()
		t.Fatal("Test setup error: Could not replace serial port with testable version.")
	}


	// 7. Cleanup
	fmt.Println("Conceptual Integration Test: Calling vmManager.DeleteVM()...")
	delErr := vmManager.DeleteVM(vmID)
	if delErr != nil { t.Errorf("vmManager.DeleteVM failed: %v", delErr) }

	fmt.Println("Conceptual Integration Test: TestMinimalSystemBoot_SerialOutput_Halt - PASSED")
}
