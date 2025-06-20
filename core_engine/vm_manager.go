package core_engine

import (
	"fmt"
	"sync"
	"unsafe" // Required for conceptual HostUserAddr, even if operations are stubbed

	pb "github.com/V-Architect/v-architect-core/proto"
)

// VMStatus enum and String() method (as previously defined)
type VMStatus int
const ( /* ... CREATED to UNKNOWN ... */
	CREATED VMStatus = iota; STARTING; RUNNING; PAUSING; PAUSED; RESUMING; STOPPING; STOPPED; FAILED; UNKNOWN
)
func (s VMStatus) String() string { /* ... */
	switch s {
		case CREATED: return "CREATED"; case STARTING: return "STARTING"; case RUNNING: return "RUNNING";
		case PAUSING: return "PAUSING"; case PAUSED: return "PAUSED"; case RESUMING: return "RESUMING";
		case STOPPING: return "STOPPING"; case STOPPED: return "STOPPED"; case FAILED: return "FAILED";
		default: return "UNKNOWN";
	}
}

// MemoryRegion struct (as previously defined)
type MemoryRegion struct {
	Slot          uint32; GuestPhysAddr uint64; MemorySize    uint64; HostUserAddr  uintptr; Flags         uint32;
}

// VirtualMachine struct (ensure serialPorts is present)
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
	// virtioBlkDevices []*VirtIOBlkDevice // Future
	// virtioNetDevices []*VirtIONetDevice // Future
	lastError    error
	statusLock   sync.RWMutex
	resourceLock sync.Mutex
}

// NewVirtualMachine (as previously defined)
func NewVirtualMachine(id string, config *pb.VMConfig, vmFd int) *VirtualMachine {
	var ramSize uint64
	if config.GetMemoryConfig() != nil { // Updated to use MemoryConfig from finalized proto
		ramSize = config.GetMemoryConfig().GetSizeMb() * 1024 * 1024
	} else if config.GetVramConfig() != nil { // Fallback for older VMConfig structure if still in use
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

// setupMemory, cleanupMemory, setupInitialPaging (from memory.go, but methods on VM)
// initializeDevices, cleanupDevices (as previously defined, handling serial ports)
// These methods are assumed to be correctly defined from previous sub-issues.
// For brevity, their full code is not repeated here but they are called by StartProcess.
func (vm *VirtualMachine) setupMemory() error { // Includes call to setupInitialPaging
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	if vm.guestMem != nil || len(vm.memoryRegions) > 0 { /* ... check if already setup ... */ return nil }
	if vm.ramSizeBytes == 0 { return fmt.Errorf("VM %s RAM size is 0", vm.ID) }
	vm.guestMem = make([]byte, vm.ramSizeBytes)
	hostUserAddr := uintptr(0); if len(vm.guestMem) > 0 { hostUserAddr = uintptr(unsafe.Pointer(&vm.guestMem[0])) }
	mainRamRegion := MemoryRegion{Slot: 0, GuestPhysAddr: 0x0, MemorySize: vm.ramSizeBytes, HostUserAddr: hostUserAddr, Flags: 0}
	fmt.Printf("Conceptual Memory: KVM_SET_USER_MEMORY_REGION for VM %s, Slot %d\n", vm.ID, mainRamRegion.Slot)
	vm.memoryRegions = append(vm.memoryRegions, mainRamRegion)
	if err := vm.setupInitialPaging(); err != nil {
		vm.guestMem = nil; vm.memoryRegions = nil
		return fmt.Errorf("failed to setup initial paging for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual Memory: Guest RAM and initial page tables setup complete for VM ID: %s.\n", vm.ID)
	return nil
}
func (vm *VirtualMachine) cleanupMemory() error { /* ... conceptual munmap and clear regions ... */
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	vm.guestMem = nil; vm.memoryRegions = nil
	fmt.Printf("Conceptual Memory: cleanupMemory for VM ID: %s\n", vm.ID); return nil
}
func (vm *VirtualMachine) initializeDevices() error {
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual Devices: VM %s - Initializing devices...\n", vm.ID)
	if vm.Config.GetSerialPorts() != nil && len(vm.Config.GetSerialPorts()) > 0 {
		vm.serialPorts = make([]*SerialPortDevice, 0, len(vm.Config.GetSerialPorts()))
		for i, spConfig := range vm.Config.GetSerialPorts() {
			var ioBase uint16
			if spConfig.GetIoBaseOverride() != 0 { ioBase = uint16(spConfig.GetIoBaseOverride()) } else {
				if i == 0 { ioBase = DEFAULT_SERIAL_IO_BASE_COM1 } else if i == 1 { ioBase = DEFAULT_SERIAL_IO_BASE_COM2 } else { continue }
			}
			devID := spConfig.GetId(); if devID == "" { devID = fmt.Sprintf("com%d", i+1) }
			outputToStdOut := (spConfig.GetType() == pb.SerialPortConfig_STDIO)
			if spConfig.GetType() == pb.SerialPortConfig_LOG_ONLY { outputToStdOut = true } // For test purposes, make LOG_ONLY print like STDIO

			serialDev, err := NewSerialPortDevice(devID, spConfig, ioBase, 4 ) // Pass full config
			if err != nil { return fmt.Errorf("failed to create serial port %s for VM %s: %w", deviceID, vm.ID, err) }
			vm.serialPorts = append(vm.serialPorts, serialDev)
		}
		fmt.Printf("Conceptual Devices: VM %s - Serial ports initialized.\n", vm.ID)
	}
	// ... other devices ...
	return nil
}
func (vm *VirtualMachine) cleanupDevices() error {
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual Devices: VM %s - Cleaning up devices...\n", vm.ID)
	for _, sp := range vm.serialPorts { if sp != nil { sp.Close() } }
	vm.serialPorts = nil
	// ... other devices ...
	return nil
}


// Constants for kernel/initrd/boot_params loading addresses
const (
	DefaultKernelLoadGPA  uint64 = 0x100000  // 1MB (Linux bzImage conventional load address)
	DefaultInitrdLoadGPA  uint64 = 0x2000000 // 32MB (example, ensure it's after kernel & page tables)
	DefaultBootParamsGPA  uint64 = 0x010000  // 64KB (standard for x86 zero page/boot_params)
)

// StartProcess orchestrates the full VM startup sequence.
// This method assumes vm.Config, vm.ID, and vm.vmFd are already set.
// kvmSystemFd is the FD for /dev/kvm, needed for some KVM ioctls like KVM_GET_VCPU_MMAP_SIZE.
func (vm *VirtualMachine) StartProcess(kvmSystemFd int, kernelPath string, initrdPath string, cmdline string) (err error) {
	vm.statusLock.Lock()
	if vm.Status != CREATED && vm.Status != STOPPED {
		vm.statusLock.Unlock()
		return fmt.Errorf("VM %s is in state %s, cannot start", vm.ID, vm.Status)
	}
	vm.Status = STARTING
	vm.lastError = nil
	vm.statusLock.Unlock()

	fmt.Printf("Conceptual StartProcess: VM %s initiated. Kernel: '%s', Initrd: '%s', Cmdline: '%s'\n",
		vm.ID, kernelPath, initrdPath, cmdline)

	// Defer function to handle cleanup and status update on error during startup sequence
	defer func() {
		if err != nil {
			fmt.Printf("Conceptual StartProcess: Error during VM %s startup: %v. Attempting cleanup.\n", vm.ID, err)
			vm.SetStatus(FAILED)
			vm.SetError(err)

			if cleanupErr := vm.cleanupDevices(); cleanupErr != nil {
				fmt.Printf("Conceptual StartProcess: Error during device cleanup for VM %s after startup failure: %v\n", vm.ID, cleanupErr)
			}
			if cleanupErr := vm.cleanupMemory(); cleanupErr != nil {
				fmt.Printf("Conceptual StartProcess: Error during memory cleanup for VM %s after startup failure: %v\n", vm.ID, cleanupErr)
			}
			vm.resourceLock.Lock()
			for _, v_cpu := range vm.vcpus {
				if v_cpu != nil { v_cpu.Close() }
			}
			vm.vcpus = nil
			vm.resourceLock.Unlock()
			fmt.Printf("Conceptual StartProcess: Cleanup attempt for VM %s finished after startup failure.\n", vm.ID)
		}
	}()

	// 1. Setup Memory (includes initial paging setup)
	fmt.Printf("Conceptual StartProcess: VM %s - Calling setupMemory()...\n", vm.ID)
	if err = vm.setupMemory(); err != nil { // setupMemory calls setupInitialPaging internally
		return fmt.Errorf("failed to setup memory for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual StartProcess: VM %s - Memory and initial paging setup complete.\n", vm.ID)

	pml4GPA := PageMapLevel4AddressGPA // From memory.go

	// 2. Load Boot Image (Kernel, Optional Initrd, Boot Params)
	fmt.Printf("Conceptual StartProcess: VM %s - Calling loadBootImage()...\n", vm.ID)
	var kernelEntryActualGPA, bootParamsActualGPA uint64
	kernelEntryActualGPA, bootParamsActualGPA, err = vm.loadBootImage(kernelPath, initrdPath, cmdline,
							   DefaultKernelLoadGPA, DefaultInitrdLoadGPA, DefaultBootParamsGPA)
	if err != nil {
		return fmt.Errorf("failed to load boot image for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual StartProcess: VM %s - Boot image loading complete. Kernel Entry: 0x%X, Boot Params: 0x%X\n",
		vm.ID, kernelEntryActualGPA, bootParamsActualGPA)

	// 3. Initialize Devices (e.g., Serial Port, VirtIO devices)
	fmt.Printf("Conceptual StartProcess: VM %s - Calling initializeDevices()...\n", vm.ID)
	if err = vm.initializeDevices(); err != nil {
		 return fmt.Errorf("failed to initialize devices for VM %s: %w", vm.ID, err)
	}
	fmt.Printf("Conceptual StartProcess: VM %s - Device initialization complete.\n", vm.ID)

	// 4. Create and Initialize vCPUs
	vcpuCount := vm.Config.GetVcpuConfig().GetCount()
	fmt.Printf("Conceptual StartProcess: VM %s - Creating and initializing %d vCPUs...\n", vm.ID, vcpuCount)
	vm.resourceLock.Lock()
	vm.vcpus = make([]*VCPU, 0, vcpuCount)
	for i := 0; i < int(vcpuCount); i++ {
		var vcpu *VCPU
		vcpu, err = NewVCPU(vm, i, kvmSystemFd)
		if err != nil {
			err = fmt.Errorf("failed to create vCPU %d for VM %s: %w", i, vm.ID, err)
			vm.resourceLock.Unlock()
			return err
		}

		if err = vcpu.setupInitialArchState(kernelEntryActualGPA, bootParamsActualGPA, pml4GPA); err != nil {
			err = fmt.Errorf("failed to setup initial arch state for vCPU %d on VM %s: %w", i, vm.ID, err)
			vm.resourceLock.Unlock()
			return err
		}
		vm.vcpus = append(vm.vcpus, vcpu)
		fmt.Printf("Conceptual StartProcess: VM %s - vCPU %d created and state initialized.\n", vm.ID, i)
	}
	vm.resourceLock.Unlock()
	fmt.Printf("Conceptual StartProcess: VM %s - All vCPUs created and initialized.\n", vm.ID)

	// 5. Launch vCPU Run Loops (each in a goroutine)
	fmt.Printf("Conceptual StartProcess: VM %s - Launching %d vCPU run loops...\n", vm.ID, len(vm.vcpus))
	for _, vcpuInstance := range vm.vcpus {
		go func(v_cpu_instance *VCPU) { // Ensure correct capture of loop variable
			fmt.Printf("Conceptual StartProcess: Starting run loop for vCPU %d on VM %s\n", v_cpu_instance.id, v_cpu_instance.vm.ID)
			runErr := v_cpu_instance.Run()
			if runErr != nil {
				fmt.Printf("Error running vCPU %d on VM %s: %v\n", v_cpu_instance.id, v_cpu_instance.vm.ID, runErr)
				v_cpu_instance.vm.SetError(runErr)
				v_cpu_instance.vm.SetStatus(FAILED)
			}
			fmt.Printf("Conceptual StartProcess: Run loop for vCPU %d on VM %s finished.\n", v_cpu_instance.id, v_cpu_instance.vm.ID)
		}(vcpuInstance)
	}

	vm.SetStatus(RUNNING)
	fmt.Printf("Conceptual StartProcess: VM %s - All vCPU run loops launched. VM status is RUNNING.\n", vm.ID)
	return nil
}

// Stop, Pause, Resume, GetStatus, GetLastError, SetError, SetStatus methods as previously defined
func (vm *VirtualMachine) Stop(force bool) error { /* ... */
	vm.statusLock.Lock()
	if vm.Status == STOPPED || vm.Status == CREATED {
		if vm.Status == FAILED && force { } else if vm.Status != FAILED { vm.statusLock.Unlock(); return nil }
	}
	if vm.Status != RUNNING && vm.Status != PAUSED && vm.Status != STARTING && vm.Status != FAILED && vm.Status != RESUMING && vm.Status != PAUSING {
		vm.statusLock.Unlock(); return fmt.Errorf("VM %s is in state %s, cannot stop", vm.ID, vm.Status)
	}
	previousStatus := vm.Status; vm.Status = STOPPING; vm.statusLock.Unlock()
	fmt.Printf("Conceptual VM: VM %s status changing from %s to STOPPING. Forced: %t\n", vm.ID, previousStatus, force)
	fmt.Println("Conceptual VM: --- BEGIN Full VM Stop Sequence (Conceptual Stubs) ---")
	vm.resourceLock.Lock()
	if len(vm.vcpus) > 0 {
		fmt.Printf("Conceptual VM: vCPUs for VM %s would be closed and cleaned up.\n", vm.ID)
		for _, v_cpu := range vm.vcpus { if v_cpu != nil {v_cpu.Close()} }
		vm.vcpus = nil
	}
	vm.resourceLock.Unlock()
	if err := vm.cleanupDevices(); err != nil { fmt.Printf("Warning: Error during device cleanup in Stop for VM %s: %v\n", vm.ID, err) }
	if err := vm.cleanupMemory(); err != nil { fmt.Printf("Warning: Error during memory cleanup in Stop for VM %s: %v\n", vm.ID, err) }
	fmt.Println("Conceptual VM: --- END Full VM Stop Sequence (Conceptual Stubs) ---")
	vm.SetStatus(STOPPED)
	fmt.Printf("Conceptual VM: VM %s status changed to STOPPED from %s.\n", vm.ID, previousStatus)
	return nil
}
func (vm *VirtualMachine) Pause() error { /* ... */
	vm.statusLock.Lock(); defer vm.statusLock.Unlock()
	if vm.Status != RUNNING { return fmt.Errorf("VM %s is not RUNNING (current: %s), cannot pause", vm.ID, vm.Status) }
	vm.Status = PAUSING; fmt.Printf("VM %s: Status changing from %s to PAUSING.\n", vm.ID, RUNNING)
	vm.Status = PAUSED; fmt.Printf("VM %s: Status changed to PAUSED.\n", vm.ID); return nil
}
func (vm *VirtualMachine) Resume() error { /* ... */
	vm.statusLock.Lock(); defer vm.statusLock.Unlock()
	if vm.Status != PAUSED { return fmt.Errorf("VM %s is not PAUSED (current: %s), cannot resume", vm.ID, vm.Status) }
	vm.Status = RESUMING; fmt.Printf("VM %s: Status changing from %s to RESUMING.\n", vm.ID, PAUSED)
	vm.Status = RUNNING; fmt.Printf("VM %s: Status changed to RUNNING.\n", vm.ID); return nil
}
func (vm *VirtualMachine) GetStatus() VMStatus { vm.statusLock.RLock(); defer vm.statusLock.RUnlock(); return vm.Status }
func (vm *VirtualMachine) GetLastError() error { vm.statusLock.RLock(); defer vm.statusLock.RUnlock(); return vm.lastError }
func (vm *VirtualMachine) SetError(err error) {
	vm.statusLock.Lock(); defer vm.statusLock.Unlock()
	fmt.Printf("Conceptual VM %s: Recording error: %v\n", vm.ID, err); vm.lastError = err
}
func (vm *VirtualMachine) SetStatus(status VMStatus) {
	vm.statusLock.Lock(); defer vm.statusLock.Unlock()
	if vm.Status == FAILED && status != FAILED { return }
	fmt.Printf("Conceptual VM %s: Status changing from %s to %s (via SetStatus).\n", vm.ID, vm.Status, status); vm.Status = status
}


// VMManager struct and methods (NewVMManager, RegisterNewVM, GetVM, DeleteVM, ListVMs)
// RegisterNewVM calls vm.setupMemory()
// DeleteVM calls vm.Stop(true), vm.cleanupDevices(), vm.cleanupMemory(), hypervisor.CloseVMContext()
type VMManager struct { /* ... */
	vms        map[string]*VirtualMachine; vmsLock    sync.RWMutex; hypervisor Hypervisor
}
func NewVMManager(h Hypervisor) *VMManager { /* ... */
	fmt.Println("Conceptual VMManager: NewVMManager created."); return &VMManager{ vms: make(map[string]*VirtualMachine), hypervisor: h }
}
func (m *VMManager) RegisterNewVM(idSuggestion string, config *pb.VMConfig) (*VirtualMachine, error) {
	m.vmsLock.Lock(); defer m.vmsLock.Unlock()
	vmID := idSuggestion; if vmID == "" { vmID = fmt.Sprintf("vm-%d-%s", len(m.vms)+1001, config.GetVmName()) }
	if _, exists := m.vms[vmID]; exists { return nil, fmt.Errorf("VM with ID %s already exists", vmID) }
	fmt.Printf("Conceptual VMManager: Registering new VM. ID: %s, Name: %s\n", vmID, config.GetVmName())
	vmFd, err := m.hypervisor.CreateVM(vmID, config); if err != nil { return nil, fmt.Errorf("hypervisor.CreateVM failed for %s: %w", vmID, err) }
	vm := NewVirtualMachine(vmID, config, vmFd)
	if err := vm.setupMemory(); err != nil { // setupMemory includes setupInitialPaging
		m.hypervisor.CloseVMContext(vmFd); return nil, fmt.Errorf("vm.setupMemory failed for %s: %w", vmID, err)
	}
	m.vms[vmID] = vm; fmt.Printf("Conceptual VMManager: VM %s registered.\n", vmID); return vm, nil
}
func (m *VMManager) GetVM(vmID string) (*VirtualMachine, error) { /* ... */
	m.vmsLock.RLock(); defer m.vmsLock.RUnlock(); vm, ok := m.vms[vmID]; if !ok { return nil, fmt.Errorf("VM %s not found", vmID) }; return vm, nil
}
func (m *VMManager) DeleteVM(vmID string) error { /* ... */
	m.vmsLock.Lock(); defer m.vmsLock.Unlock(); vm, ok := m.vms[vmID]; if !ok { return fmt.Errorf("VM %s not found", vmID) }
	fmt.Printf("Conceptual VMManager: Deleting VM %s (status: %s)\n", vmID, vm.GetStatus())
	if status := vm.GetStatus(); status != STOPPED && status != CREATED && status != FAILED {
		if err := vm.Stop(true); err != nil { fmt.Printf("Error stopping VM %s during delete: %v\n", vmID, err); vm.SetStatus(STOPPED); vm.SetError(err) }
	}
	if len(vm.vcpus) > 0 { vm.resourceLock.Lock(); for _, v := range vm.vcpus { if v != nil {v.Close()}}; vm.vcpus = nil; vm.resourceLock.Unlock() }
	if err := vm.cleanupDevices(); err != nil { fmt.Printf("Error cleaning devices for VM %s: %v\n", vmID, err) }
	if err := vm.cleanupMemory(); err != nil { fmt.Printf("Error cleaning memory for VM %s: %v\n", vmID, err) }
	if vm.vmFd > 0 { if err := m.hypervisor.CloseVMContext(vm.vmFd); err != nil { fmt.Printf("Error closing VM context for %s: %v\n", vmID, err) } }
	delete(m.vms, vmID); fmt.Printf("Conceptual VMManager: VM %s deleted.\n", vmID); return nil
}
func (m *VMManager) ListVMs() map[string]string { /* ... */
	m.vmsLock.RLock(); defer m.vmsLock.RUnlock(); statuses := make(map[string]string)
	for id, vm := range m.vms { statuses[id] = vm.GetStatus().String() }; return statuses
}
// initializeDevices and cleanupDevices methods on VirtualMachine
func (vm *VirtualMachine) initializeDevices() error {
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual VM %s: Initializing I/O devices...\n", vm.ID)
	if vm.Config.GetSerialPorts() != nil && len(vm.Config.GetSerialPorts()) > 0 {
		vm.serialPorts = make([]*SerialPortDevice, 0, len(vm.Config.GetSerialPorts()))
		for i, spConfig := range vm.Config.GetSerialPorts() {
			var ioBase uint16
			if spConfig.GetIoBaseOverride() != 0 { ioBase = uint16(spConfig.GetIoBaseOverride()) } else {
				if i == 0 { ioBase = DEFAULT_SERIAL_IO_BASE_COM1 } else if i == 1 { ioBase = DEFAULT_SERIAL_IO_BASE_COM2 } else { continue }
			}
			devID := spConfig.GetId(); if devID == "" { devID = fmt.Sprintf("com%d", i+1) }
			outputToStdOut := (spConfig.GetType() == pb.SerialPortConfig_STDIO || spConfig.GetType() == pb.SerialPortConfig_LOG_ONLY)
			serialDev, err := NewSerialPortDevice(devID, spConfig, ioBase, 4 )
			if err != nil { return fmt.Errorf("failed to create serial port %s for VM %s: %w", deviceID, vm.ID, err) }
			vm.serialPorts = append(vm.serialPorts, serialDev)
		}
		fmt.Printf("Conceptual VM %s: Serial ports initialized.\n", vm.ID)
	} else {
		fmt.Printf("Conceptual VM %s: No serial port configurations found.\n", vm.ID)
	}
	fmt.Printf("Conceptual VM %s: All I/O devices conceptually initialized.\n", vm.ID)
	return nil
}
func (vm *VirtualMachine) cleanupDevices() error {
	vm.resourceLock.Lock(); defer vm.resourceLock.Unlock()
	fmt.Printf("Conceptual VM %s: Cleaning up I/O devices...\n", vm.ID)
	var lastErr error
	if len(vm.serialPorts) > 0 {
		for i, spDev := range vm.serialPorts {
			if spDev == nil { continue }
			if err := spDev.Close(); err != nil { lastErr = err }
		}
		vm.serialPorts = nil
		fmt.Printf("Conceptual VM %s: Serial ports cleaned up.\n", vm.ID)
	}
	if lastErr != nil { return fmt.Errorf("device cleanup error for VM %s: %w", vm.ID, lastErr) }
	fmt.Printf("Conceptual VM %s: All I/O devices conceptually cleaned up.\n", vm.ID)
	return nil
}
