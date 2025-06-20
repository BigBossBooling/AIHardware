package core_engine

import (
	"fmt"
	"sync"
	"unsafe" // Required for conceptual HostUserAddr, even if operations are stubbed

	// Assuming pb types are generated and accessible via this import path
	pb "github.com/V-Architect/v-architect-core/core_engine/pb"
	// "golang.org/x/sys/unix" // For actual syscalls and mmap for KVM_SET_USER_MEMORY_REGION
)

// VMStatus enum and String() method
type VMStatus int

const (
	CREATED VMStatus = iota
	STARTING
	RUNNING
	PAUSING
	PAUSED
	RESUMING
	STOPPING
	STOPPED
	FAILED
)

func (s VMStatus) String() string {
	return [...]string{"CREATED", "STARTING", "RUNNING", "PAUSING", "PAUSED", "RESUMING", "STOPPING", "STOPPED", "FAILED"}[s]
}


// MemoryRegion describes a KVM memory slot.
type MemoryRegion struct {
	Slot          uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	HostUserAddr  uintptr
	Flags         uint32
}

// VirtualMachine struct
type VirtualMachine struct {
	ID             string
	Config         *pb.VMConfig
	Status         VMStatus
	vmFd           int

	vcpus          []*VCPU
	memoryRegions  []MemoryRegion
	guestMem       []byte
	ramSizeBytes   uint64
	serialPorts    []*SerialPortDevice

	lastError    error
	statusLock   sync.RWMutex
	resourceLock sync.Mutex
}

// NewVirtualMachine
func NewVirtualMachine(id string, config *pb.VMConfig, vmFd int) *VirtualMachine {
	fmt.Printf("Conceptual VM: NewVirtualMachine created: ID=%s, Name=%s, VMFd=%d\n", id, config.GetVmName(), vmFd)

	var ramSize uint64
	if config.GetVramConfig() != nil {
		ramSize = config.GetVramConfig().GetSizeMb() * 1024 * 1024
	}

	return &VirtualMachine{
		ID:            id,
		Config:        config,
		Status:        CREATED,
		vmFd:          vmFd,
		vcpus:         make([]*VCPU, 0, config.GetVcpuConfig().GetCount()),
		memoryRegions: make([]MemoryRegion, 0, 1),
		serialPorts:   make([]*SerialPortDevice, 0, len(config.GetSerialPorts())),
		ramSizeBytes:  ramSize,
	}
}

// setupMemory
func (vm *VirtualMachine) setupMemory() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	if vm.guestMem != nil || len(vm.memoryRegions) > 0 {
		fmt.Printf("Warning: VM %s memory might already be configured or guestMem not nil. Re-checking.\n", vm.ID)
		if vm.guestMem != nil && vm.ramSizeBytes == uint64(len(vm.guestMem)) && len(vm.memoryRegions) > 0 {
			fmt.Printf("Conceptual Memory: VM %s memory appears to be already set up with correct size. Skipping re-setup.\n", vm.ID)
			return nil
		}
		return fmt.Errorf("VM %s memory setup state inconsistent (guestMem: %p, regions: %d)", vm.ID, vm.guestMem, len(vm.memoryRegions))
	}

	if vm.ramSizeBytes == 0 {
		return fmt.Errorf("VM %s RAM size is 0 (VramConfig.SizeMb: %d), cannot setup memory", vm.ID, vm.Config.GetVramConfig().GetSizeMb())
	}

	fmt.Printf("Conceptual Memory: setupMemory for VM ID: %s, Size: %d bytes\n", vm.ID, vm.ramSizeBytes)

	vm.guestMem = make([]byte, vm.ramSizeBytes)
	hostUserAddr := uintptr(0)
	if len(vm.guestMem) > 0 {
		hostUserAddr = uintptr(unsafe.Pointer(&vm.guestMem[0]))
	}
	fmt.Printf("Conceptual Memory: Mmap'd %d bytes for VM %s guest RAM at host addr (conceptual) 0x%x\n", vm.ramSizeBytes, vm.ID, hostUserAddr)

	mainRamRegion := MemoryRegion{
		Slot:          0,
		GuestPhysAddr: 0x0,
		MemorySize:    vm.ramSizeBytes,
		HostUserAddr:  hostUserAddr,
		Flags:         0,
	}

	fmt.Printf("Conceptual Memory: KVM_SET_USER_MEMORY_REGION ioctl called for VM %s, Slot %d, GPA 0x%x, Size %d bytes, HVA 0x%x\n",
		vm.ID, mainRamRegion.Slot, mainRamRegion.GuestPhysAddr, mainRamRegion.MemorySize, mainRamRegion.HostUserAddr)

	vm.memoryRegions = append(vm.memoryRegions, mainRamRegion)

	if err := vm.setupInitialPaging(); err != nil {
		vm.guestMem = nil
		vm.memoryRegions = nil
		return fmt.Errorf("failed to setup initial guest paging for VM %s: %w", vm.ID, err)
	}

	fmt.Printf("Conceptual Memory: Guest RAM and initial page tables setup complete for VM ID: %s.\n", vm.ID)
	return nil
}

// cleanupMemory
func (vm *VirtualMachine) cleanupMemory() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	fmt.Printf("Conceptual Memory: cleanupMemory for VM ID: %s\n", vm.ID)
	if vm.guestMem != nil {
		vm.guestMem = nil
		fmt.Printf("Conceptual Memory: Guest RAM (mmap'd host memory) unmapped for VM ID: %s\n", vm.ID)
	} else {
		fmt.Printf("Conceptual Memory: No guest RAM (vm.guestMem was nil) to unmap for VM ID: %s\n", vm.ID)
	}

	if len(vm.memoryRegions) > 0 {
		fmt.Printf("Conceptual Memory: Clearing %d tracked memory regions for VM ID: %s.\n", len(vm.memoryRegions), vm.ID)
		vm.memoryRegions = make([]MemoryRegion, 0, 1)
	}
	return nil
}

// Constants for conceptual kernel/initrd/boot_params loading addresses
const (
	DefaultKernelLoadGPA  uint64 = 0x100000  // 1MB (typical for bzImage)
	DefaultInitrdLoadGPA  uint64 = 0x2000000 // 32MB (example, must be after kernel and page tables)
	DefaultBootParamsGPA  uint64 = 0x010000  // 64KB (standard for x86 zero page/boot_params)
	// PageMapLevel4AddressGPA is assumed to be available from memory.go (e.g., 0x1000)
)

// StartProcess orchestrates the full VM startup sequence.
func (vm *VirtualMachine) StartProcess(kvmSystemFd int, kernelPath string, initrdPath string, cmdline string) error {
	vm.statusLock.Lock()
	if vm.Status != CREATED && vm.Status != STOPPED {
		vm.statusLock.Unlock()
		return fmt.Errorf("VM %s is in state %s, cannot start", vm.ID, vm.Status)
	}
	vm.Status = STARTING
	vm.lastError = nil
	vm.statusLock.Unlock()

	fmt.Printf("Conceptual: VM %s - StartProcess initiated. Kernel: '%s', Initrd: '%s', Cmdline: '%s'\n",
		vm.ID, kernelPath, initrdPath, cmdline)

	var err error
	defer func() {
		if err != nil {
			fmt.Printf("Conceptual: VM %s - Error during StartProcess: %v. Attempting cleanup.\n", vm.ID, err)
			vm.SetStatus(FAILED)
			vm.SetError(err)

			if errCleanup := vm.cleanupDevices(); errCleanup != nil {
				fmt.Printf("Conceptual: VM %s - Error during device cleanup after startup failure: %v\n", vm.ID, errCleanup)
			}
			if errCleanup := vm.cleanupMemory(); errCleanup != nil {
				fmt.Printf("Conceptual: VM %s - Error during memory cleanup after startup failure: %v\n", vm.ID, errCleanup)
			}
			vm.resourceLock.Lock()
			for _, v_cpu := range vm.vcpus { // Renamed v to v_cpu to avoid conflict
				if v_cpu != nil { v_cpu.Close() }
			}
			vm.vcpus = nil
			vm.resourceLock.Unlock()
			fmt.Printf("Conceptual: VM %s - Cleanup attempt finished after startup failure.\n", vm.ID)
		}
	}()

	fmt.Printf("Conceptual: VM %s - Calling setupMemory()...\n", vm.ID)
	if err = vm.setupMemory(); err != nil {
		return fmt.Errorf("failed to setup memory for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual: VM %s - Memory and initial paging setup complete.\n", vm.ID)

	pml4GPA := PageMapLevel4AddressGPA

	fmt.Printf("Conceptual: VM %s - Calling loadBootImage()...\n", vm.ID)
	if err = vm.loadBootImage(kernelPath, initrdPath, cmdline,
							   DefaultKernelLoadGPA, DefaultInitrdLoadGPA, DefaultBootParamsGPA); err != nil {
		return fmt.Errorf("failed to load boot image for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual: VM %s - Boot image loading complete.\n", vm.ID)

	fmt.Printf("Conceptual: VM %s - Calling initializeDevices()...\n", vm.ID)
	if err = vm.initializeDevices(); err != nil {
		 return fmt.Errorf("failed to initialize devices for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual: VM %s - Device initialization complete.\n", vm.ID)

	fmt.Printf("Conceptual: VM %s - Creating and initializing %d vCPUs...\n", vm.ID, vm.Config.GetVcpuConfig().GetCount())
	vm.resourceLock.Lock()
	vm.vcpus = make([]*VCPU, 0, vm.Config.GetVcpuConfig().GetCount())
	for i := 0; i < int(vm.Config.GetVcpuConfig().GetCount()); i++ {
		var vcpu *VCPU
		vcpu, err = NewVCPU(vm, i, kvmSystemFd)
		if err != nil {
			err = fmt.Errorf("failed to create vCPU %d for VM %s: %w", i, vm.ID, err)
			vm.resourceLock.Unlock()
			return err
		}

		if err = vcpu.setupInitialArchState(DefaultKernelLoadGPA, DefaultBootParamsGPA, pml4GPA); err != nil {
			err = fmt.Errorf("failed to setup initial arch state for vCPU %d on VM %s: %w", i, vm.ID, err)
			vm.resourceLock.Unlock()
			return err
		}
		vm.vcpus = append(vm.vcpus, vcpu)
		fmt.Printf("Conceptual: VM %s - vCPU %d created and state initialized.\n", vm.ID, i)
	}
	vm.resourceLock.Unlock()
	fmt.Printf("Conceptual: VM %s - All vCPUs created and initialized.\n", vm.ID)

	fmt.Printf("Conceptual: VM %s - Launching vCPU run loops...\n", vm.ID)
	for _, vcpuInstance := range vm.vcpus {
		go func(v_cpu_instance *VCPU) { // Changed v to v_cpu_instance
			fmt.Printf("Conceptual: Starting run loop for vCPU %d on VM %s\n", v_cpu_instance.id, v_cpu_instance.vm.ID)
			runErr := v_cpu_instance.Run()
			if runErr != nil {
				fmt.Printf("Error running vCPU %d on VM %s: %v\n", v_cpu_instance.id, v_cpu_instance.vm.ID, runErr)
				v_cpu_instance.vm.SetError(runErr)
				v_cpu_instance.vm.SetStatus(FAILED)
			}
			fmt.Printf("Conceptual: Run loop for vCPU %d on VM %s finished.\n", v_cpu_instance.id, v_cpu_instance.vm.ID)
		}(vcpuInstance)
	}

	vm.SetStatus(RUNNING)
	fmt.Printf("Conceptual: VM %s - All vCPU run loops launched. VM status is RUNNING.\n", vm.ID)
	return nil
}

// SetStatus updates the VM's status in a thread-safe manner.
func (vm *VirtualMachine) SetStatus(status VMStatus) {
	vm.statusLock.Lock()
	defer vm.statusLock.Unlock()
	if vm.Status == FAILED && status != FAILED { // Don't override FAILED unless explicitly resetting
		fmt.Printf("Conceptual VM %s: Attempt to change status from FAILED to %s ignored.\n", vm.ID, status)
		return
	}
	fmt.Printf("Conceptual VM %s: Status changing from %s to %s.\n", vm.ID, vm.Status, status)
	vm.Status = status
}

// SetError records an error for the VM in a thread-safe manner.
func (vm *VirtualMachine) SetError(err error) {
	vm.statusLock.Lock()
	defer vm.statusLock.Unlock()
	fmt.Printf("Conceptual VM %s: Recording error: %v\n", vm.ID, err)
	vm.lastError = err
}


// Stop, Pause, Resume, GetStatus, GetLastError, initializeDevices, cleanupDevices methods as previously defined
func (vm *VirtualMachine) Stop(force bool) error {
	vm.statusLock.Lock()
	if vm.Status == STOPPED || vm.Status == CREATED {
		fmt.Printf("Conceptual VM: VM %s already stopped or not started (Status: %s).\n", vm.ID, vm.Status)
		if vm.Status == FAILED && force {
			fmt.Printf("Conceptual VM: VM %s is FAILED, attempting forced stop/cleanup.\n", vm.ID)
		} else if vm.Status != FAILED {
			vm.statusLock.Unlock()
			return nil
		}
	}
	if vm.Status != RUNNING && vm.Status != PAUSED && vm.Status != STARTING && vm.Status != FAILED && vm.Status != RESUMING && vm.Status != PAUSING {
		vm.statusLock.Unlock()
		return fmt.Errorf("VM %s is in state %s, cannot stop", vm.ID, vm.Status)
	}
	previousStatus := vm.Status
	fmt.Printf("Conceptual VM: VM %s changing status from %s to STOPPING. Forced: %t\n", vm.ID, vm.Status, force)
	vm.Status = STOPPING
	vm.statusLock.Unlock()
	fmt.Println("Conceptual VM: --- BEGIN Full VM Stop Sequence (Conceptual Stubs) ---")
	fmt.Printf("Conceptual VM: Signaling %d vCPU run loops to exit for VM %s.\n", len(vm.vcpus), vm.ID)
	fmt.Printf("Conceptual VM: Conceptually waiting for vCPU goroutines to finish for VM %s.\n", vm.ID)
	vm.resourceLock.Lock()
	if len(vm.vcpus) > 0 {
		fmt.Printf("Conceptual VM: vCPUs for VM %s would be closed and cleaned up.\n", vm.ID)
		for _, v_cpu := range vm.vcpus { if v_cpu != nil {v_cpu.Close()} }
		vm.vcpus = nil
	}
	vm.resourceLock.Unlock()
	if err := vm.cleanupDevices(); err != nil {
		fmt.Printf("Warning: Error during device cleanup in Stop for VM %s: %v\n", vm.ID, err)
	}
	if err := vm.cleanupMemory(); err != nil {
		 fmt.Printf("Warning: Error during memory cleanup in Stop for VM %s: %v\n", vm.ID, err)
	}
	fmt.Println("Conceptual VM: --- END Full VM Stop Sequence (Conceptual Stubs) ---")
	vm.statusLock.Lock()
	vm.Status = STOPPED
	vm.statusLock.Unlock()
	fmt.Printf("Conceptual VM: VM %s status changed to STOPPED. Previous status: %s.\n", vm.ID, previousStatus)
	return nil
}

func (vm *VirtualMachine) Pause() error {
	vm.statusLock.Lock()
	defer vm.statusLock.Unlock()
	if vm.Status != RUNNING {
		return fmt.Errorf("VM %s is not RUNNING (current state: %s), cannot pause", vm.ID, vm.Status)
	}
	fmt.Printf("Conceptual VM: VM %s changing status from RUNNING to PAUSING.\n", vm.ID)
	vm.Status = PAUSING
	fmt.Println("Conceptual VM: Signaling vCPU run loops to pause (stop calling KVM_RUN) for VM", vm.ID)
	vm.Status = PAUSED
	fmt.Printf("Conceptual VM: VM %s status changed to PAUSED.\n", vm.ID)
	return nil
}

func (vm *VirtualMachine) Resume() error {
	vm.statusLock.Lock()
	defer vm.statusLock.Unlock()
	if vm.Status != PAUSED {
		return fmt.Errorf("VM %s is not PAUSED (current state: %s), cannot resume", vm.ID, vm.Status)
	}
	fmt.Printf("Conceptual VM: VM %s changing status from PAUSED to RESUMING.\n", vm.ID)
	vm.Status = RESUMING
	fmt.Println("Conceptual VM: Signaling vCPU run loops to resume (re-enter KVM_RUN) for VM", vm.ID)
	vm.Status = RUNNING
	fmt.Printf("Conceptual VM: VM %s status changed to RUNNING.\n", vm.ID)
	return nil
}

func (vm *VirtualMachine) GetStatus() VMStatus {
	vm.statusLock.RLock()
	defer vm.statusLock.RUnlock()
	return vm.Status
}

func (vm *VirtualMachine) GetLastError() error {
	vm.statusLock.RLock()
	defer vm.statusLock.RUnlock()
	return vm.lastError
}

type VMManager struct {
	vms        map[string]*VirtualMachine
	vmsLock    sync.RWMutex
	hypervisor Hypervisor
}

func NewVMManager(h Hypervisor) *VMManager {
	fmt.Println("Conceptual VMManager: NewVMManager created.")
	return &VMManager{
		vms:        make(map[string]*VirtualMachine),
		hypervisor: h,
	}
}

func (m *VMManager) RegisterNewVM(idSuggestion string, config *pb.VMConfig) (*VirtualMachine, error) {
	m.vmsLock.Lock()
	defer m.vmsLock.Unlock()
	vmID := idSuggestion
	if vmID == "" {
		vmID = fmt.Sprintf("vm-%d", len(m.vms)+1001)
		fmt.Printf("Conceptual VMManager: No vm_id_suggestion provided, generated ID: %s\n", vmID)
	}
	if config.GetVmId() == "" && idSuggestion == "" {
		// This is complex if pb.VMConfig is not a pointer or is immutable.
		// Conceptually, the vm.Config should reflect the final vmID.
		// For now, assume vm.Config is correctly updated or vm.ID is the source of truth.
	}
	if _, exists := m.vms[vmID]; exists {
		return nil, fmt.Errorf("VM with ID %s already exists", vmID)
	}
	fmt.Printf("Conceptual VMManager: RegisterNewVM for ID %s (Name: %s)\n", vmID, config.GetVmName())
	vmFd, err := m.hypervisor.CreateVM(vmID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVM VM context for %s: %w", vmID, err)
	}
	fmt.Printf("Conceptual VMManager: KVM VM context created (fd: %d) for ID %s.\n", vmFd, vmID)
	vm := NewVirtualMachine(vmID, config, vmFd)
	if err := vm.setupMemory(); err != nil { // setupMemory includes setupInitialPaging
		m.hypervisor.CloseVMContext(vmFd)
		return nil, fmt.Errorf("failed to setup memory for VM %s during registration: %w", vmID, err)
	}
	fmt.Printf("Conceptual VMManager: Initial memory setup complete for VM %s during registration.\n", vmID)
	m.vms[vmID] = vm
	fmt.Printf("Conceptual VMManager: VM %s (Name: %s) registered successfully.\n", vmID, config.GetVmName())
	return vm, nil
}

func (m *VMManager) GetVM(vmID string) (*VirtualMachine, error) {
	m.vmsLock.RLock()
	defer m.vmsLock.RUnlock()
	vm, ok := m.vms[vmID]
	if !ok {
		return nil, fmt.Errorf("VM with ID '%s' not found", vmID)
	}
	return vm, nil
}

func (m *VMManager) DeleteVM(vmID string) error {
	m.vmsLock.Lock()
	defer m.vmsLock.Unlock()
	vm, ok := m.vms[vmID]
	if !ok {
		return fmt.Errorf("VM with ID '%s' not found for deletion", vmID)
	}
	fmt.Printf("Conceptual VMManager: DeleteVM called for %s (Current Status: %s)\n", vmID, vm.GetStatus())
	currentStatus := vm.GetStatus()
	if currentStatus != STOPPED && currentStatus != CREATED && currentStatus != FAILED {
		fmt.Printf("Conceptual VMManager: Attempting to stop VM %s before deletion.\n", vmID)
		if err := vm.Stop(true); err != nil {
			fmt.Printf("Error stopping VM %s during delete: %v. Proceeding with resource cleanup.\n", vmID, err)
			vm.SetStatus(STOPPED)
			vm.SetError(fmt.Errorf("forced stop during deletion: %w", err))
		}
	}
	vm.resourceLock.Lock()
	if len(vm.vcpus) > 0 {
		fmt.Printf("Conceptual VMManager: Ensuring VCPUs are cleaned for VM %s post-stop.\n", vmID)
		for _, v_cpu := range vm.vcpus { if v_cpu != nil {v_cpu.Close()} }
		vm.vcpus = nil
	}
	vm.resourceLock.Unlock()
	if err := vm.cleanupDevices(); err != nil {
		fmt.Printf("Error during final device cleanup for VM %s during delete: %v. Proceeding.\n", vmID, err)
	}
	if err := vm.cleanupMemory(); err != nil {
		fmt.Printf("Error cleaning up memory for VM %s during delete: %v. Proceeding.\n", vmID, err)
	}
	fmt.Printf("Conceptual VMManager: Closing KVM VM context (fd: %d) for VM %s.\n", vm.vmFd, vmID)
	if err := m.hypervisor.CloseVMContext(vm.vmFd); err != nil {
		fmt.Printf("Error closing KVM VM context for %s (fd: %d): %w. Proceeding with map deletion.\n", vmID, vm.vmFd, err)
	}
	delete(m.vms, vmID)
	fmt.Printf("Conceptual VMManager: VM %s fully deleted and removed from VMManager.\n", vmID)
	return nil
}

func (m *VMManager) ListVMs() map[string]string {
	m.vmsLock.RLock()
	defer m.vmsLock.RUnlock()
	statuses := make(map[string]string)
	for id, vm := range m.vms {
		statuses[id] = vm.GetStatus().String()
	}
	return statuses
}

func (vm *VirtualMachine) initializeDevices() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual VM %s: Initializing I/O devices...\n", vm.ID)
	if vm.Config.GetSerialPorts() != nil && len(vm.Config.GetSerialPorts()) > 0 {
		vm.serialPorts = make([]*SerialPortDevice, 0, len(vm.Config.GetSerialPorts()))
		fmt.Printf("Conceptual VM %s: Found %d serial port configs.\n", vm.ID, len(vm.Config.GetSerialPorts()))
		for i, spConfig := range vm.Config.GetSerialPorts() {
			var ioBase uint16
			if spConfig.GetIoBaseAddressOverride() != 0 {
				ioBase = uint16(spConfig.GetIoBaseAddressOverride())
			} else {
				switch i {
				case 0: ioBase = DEFAULT_SERIAL_IO_BASE_COM1
				case 1: ioBase = DEFAULT_SERIAL_IO_BASE_COM2
				default:
					fmt.Printf("Warning: Too many serial ports defined for default I/O base assignment (port index %d for VM %s). Skipping this serial port.\n", i, vm.ID)
					continue
				}
			}
			deviceID := spConfig.GetId()
			if deviceID == "" { deviceID = fmt.Sprintf("com%d", i+1) }
			outputToStdOut := true
			if spConfig.GetType() == pb.SerialPortConfig_PTY { outputToStdOut = false }
			serialDev, err := NewSerialPortDevice(deviceID, ioBase, outputToStdOut)
			if err != nil {
				return fmt.Errorf("failed to create serial port %s for VM %s: %w", deviceID, vm.ID, err)
			}
			vm.serialPorts = append(vm.serialPorts, serialDev)
			fmt.Printf("Conceptual VM %s: Serial port '%s' (index %d) initialized at I/O base 0x%X.\n", vm.ID, deviceID, i, ioBase)
		}
	} else {
		fmt.Printf("Conceptual VM %s: No serial port configurations found.\n", vm.ID)
	}
	fmt.Printf("Conceptual VM %s: All I/O devices conceptually initialized.\n", vm.ID)
	return nil
}

func (vm *VirtualMachine) cleanupDevices() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual VM %s: Cleaning up I/O devices...\n", vm.ID)
	var lastErr error
	if len(vm.serialPorts) > 0 {
		fmt.Printf("Conceptual VM %s: Cleaning up %d serial port(s).\n", vm.ID, len(vm.serialPorts))
		for i, spDev := range vm.serialPorts {
			if spDev == nil {
				fmt.Printf("Conceptual VM %s: Serial port at index %d is nil, skipping close.\n", vm.ID, i)
				continue
			}
			fmt.Printf("Conceptual VM %s: Closing serial port '%s'.\n", vm.ID, spDev.id)
			if err := spDev.Close(); err != nil {
				fmt.Printf("Error closing serial port %s for VM %s: %v\n", spDev.id, vm.ID, err)
				lastErr = err
			}
		}
		vm.serialPorts = nil
	} else {
		fmt.Printf("Conceptual VM %s: No serial ports to clean up.\n", vm.ID)
	}
	if lastErr != nil {
		return fmt.Errorf("encountered error(s) during device cleanup for VM %s: %w", vm.ID, lastErr)
	}
	fmt.Printf("Conceptual VM %s: All I/O devices conceptually cleaned up.\n", vm.ID)
	return nil
}
