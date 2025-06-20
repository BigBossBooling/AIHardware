package core_engine

import (
	"fmt"
	"sync"
	"unsafe" // Required for conceptual HostUserAddr, even if operations are stubbed

	// Assuming pb types are generated and accessible via this import path
	pb "github.com/V-Architect/v-architect-core/core_engine/pb"
	// "golang.org/x/sys/unix" // For actual syscalls and mmap for KVM_SET_USER_MEMORY_REGION
)

// VMStatus enum and String() method (assumed to be defined as per User Sub-Issue 1.2)
// ... (If not, they would be re-added here) ...

// VCPU struct placeholder (assumed to be defined as per User Sub-Issue 2.1 or 1.1)
// type VCPU struct { id int; vmFd int; vcpuFd int; /* ... */ }


// MemoryRegion describes a KVM memory slot.
type MemoryRegion struct {
	Slot          uint32  // KVM memory slot ID
	GuestPhysAddr uint64  // Starting guest physical address
	MemorySize    uint64  // Size of the region in bytes
	HostUserAddr  uintptr // Host userspace address (mmap'd region) for this slot
	Flags         uint32  // e.g., KVM_MEM_LOG_DIRTY_PAGES
}

// VirtualMachine struct updated for memory management fields
type VirtualMachine struct {
	ID             string
	Config         *pb.VMConfig
	Status         VMStatus
	vmFd           int // KVM VM File Descriptor

	vcpus          []*VCPU        // Populated by vCPU implementation step
	memoryRegions  []MemoryRegion // Manages KVM memory slots
	guestMem       []byte         // The actual mmap'd guest memory backing RAM (conceptual)
	ramSizeBytes   uint64         // Total RAM size for quick access
	serialPorts    []*SerialPortDevice // Added for serial port devices

	lastError    error
	statusLock   sync.RWMutex // Protects Status and lastError
	resourceLock sync.Mutex   // Protects memoryRegions, guestMem, vcpus, and other shared VM resources
}

// NewVirtualMachine updated to initialize new memory fields
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
		memoryRegions: make([]MemoryRegion, 0, 1), // Typically at least one main RAM region
		serialPorts:   make([]*SerialPortDevice, 0, len(config.GetSerialPorts())), // Initialize based on config
		ramSizeBytes:  ramSize,
	}
}

// setupMemory configures the guest's RAM.
// This method would be called by vm.Start() before vCPUs are created/run,
// or more appropriately, during the VM registration/creation phase in VMManager
// after the KVM VM fd is created but before the VM is made "runnable".
func (vm *VirtualMachine) setupMemory() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	// Check if already configured or if RAM size is invalid
	if vm.guestMem != nil || len(vm.memoryRegions) > 0 {
		// This could happen if called multiple times, or if cleanup wasn't perfect.
		// For a fresh VM, this should not be an error but an indication of bad state.
		fmt.Printf("Warning: VM %s memory might already be configured or guestMem not nil. Re-checking.\n", vm.ID)
		if vm.guestMem != nil && vm.ramSizeBytes == uint64(len(vm.guestMem)) && len(vm.memoryRegions) > 0 {
			fmt.Printf("Conceptual Memory: VM %s memory appears to be already set up with correct size. Skipping re-setup.\n", vm.ID)
			return nil // Already set up correctly
		}
		// If partially set up, it's safer to error or attempt a full cleanup first.
		// For conceptual, let's assume a clean state is required or it's an error.
		return fmt.Errorf("VM %s memory setup state inconsistent (guestMem: %p, regions: %d)", vm.ID, vm.guestMem, len(vm.memoryRegions))

	}

	if vm.ramSizeBytes == 0 {
		return fmt.Errorf("VM %s RAM size is 0 (VramConfig.SizeMb: %d), cannot setup memory", vm.ID, vm.Config.GetVramConfig().GetSizeMb())
	}

	fmt.Printf("Conceptual Memory: setupMemory for VM ID: %s, Size: %d bytes\n", vm.ID, vm.ramSizeBytes)

	// 1. Mmap anonymous memory on the host to back the guest RAM.
	//    In a real implementation:
	//    guest_mem_bytes_slice, err := unix.Mmap(-1, 0, int(vm.ramSizeBytes),
	//        unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANONYMOUS|unix.MAP_PRIVATE|unix.MAP_NORESERVE)
	//    if err != nil {
	//        return fmt.Errorf("failed to mmap guest RAM for VM %s: %w", vm.ID, err)
	//    }
	//    vm.guestMem = guest_mem_bytes_slice
	//    hostUserAddr := uintptr(unsafe.Pointer(&vm.guestMem[0]))

	// Conceptual placeholder for mmap
	vm.guestMem = make([]byte, vm.ramSizeBytes) // Conceptual allocation
	hostUserAddr := uintptr(0)
	if len(vm.guestMem) > 0 {
		hostUserAddr = uintptr(unsafe.Pointer(&vm.guestMem[0]))
	}
	fmt.Printf("Conceptual Memory: Mmap'd %d bytes for VM %s guest RAM at host addr (conceptual) 0x%x\n", vm.ramSizeBytes, vm.ID, hostUserAddr)


	// 2. Define the KVM memory region for main RAM (slot 0).
	mainRamRegion := MemoryRegion{
		Slot:          0,     // Main RAM slot
		GuestPhysAddr: 0x0,   // Guest RAM typically starts at GPA 0x0
		MemorySize:    vm.ramSizeBytes,
		HostUserAddr:  hostUserAddr,
		Flags:         0,     // No KVM_MEM_LOG_DIRTY_PAGES initially
	}

	// 3. Register this memory region with KVM using KVM_SET_USER_MEMORY_REGION ioctl.
	//    Conceptual KVM ioctl call:
	//    kvmUserSpaceMemoryRegion := struct { /* fields for KVM_SET_USER_MEMORY_REGION */ } { /* ...populated... */ }
	//    _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(vm.vmFd), KVM_SET_USER_MEMORY_REGION_CONCEPTUAL, uintptr(unsafe.Pointer(&kvmUserSpaceMemoryRegion)))
	//    if errno != 0 {
	//        // syscall.Munmap(vm.guestMem) // Cleanup mmap on error
	//        vm.guestMem = nil // Clear conceptual memory
	//        return fmt.Errorf("KVM_SET_USER_MEMORY_REGION failed for VM %s slot %d: %w", vm.ID, mainRamRegion.Slot, errno)
	//    }
	fmt.Printf("Conceptual Memory: KVM_SET_USER_MEMORY_REGION ioctl called for VM %s, Slot %d, GPA 0x%x, Size %d bytes, HVA 0x%x\n",
		vm.ID, mainRamRegion.Slot, mainRamRegion.GuestPhysAddr, mainRamRegion.MemorySize, mainRamRegion.HostUserAddr)

	vm.memoryRegions = append(vm.memoryRegions, mainRamRegion)

	// 4. Initial Page Table Setup (for x86-64 in Long Mode) - Conceptual Note
	//    The VMM (V-Architect Core Engine) must create initial page tables (PML4, PDPT, PD, PT)
	//    within vm.guestMem to identity map at least the initial portion of RAM
	//    and the area where the kernel/bootloader is loaded.
	//    The CR3 register of vCPUs (set in vcpu.setupInitialArchState()) will point to the GPA of the PML4 table.
	//    This is a complex, architecture-specific task.
	fmt.Printf("Conceptual Memory: Initial page tables (PML4, etc.) would be programmatically set up in vm.guestMem for VM %s by the VMM.\n", vm.ID)

	fmt.Printf("Conceptual Memory: Guest RAM setup complete for VM ID: %s.\n", vm.ID)
	return nil
}

// cleanupMemory unmaps guest RAM and clears memory region tracking.
// KVM memory slots are implicitly cleared when the VM fd is closed.
func (vm *VirtualMachine) cleanupMemory() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	fmt.Printf("Conceptual Memory: cleanupMemory for VM ID: %s\n", vm.ID)
	if vm.guestMem != nil {
		// In a real implementation:
		// err := syscall.Munmap(vm.guestMem)
		// if err != nil {
		//     // Log error but continue cleanup if possible
		//     fmt.Printf("Error unmapping guest RAM for VM %s: %v\n", vm.ID, err)
		// }
		vm.guestMem = nil // Mark as unmapped
		fmt.Printf("Conceptual Memory: Guest RAM (mmap'd host memory) unmapped for VM ID: %s\n", vm.ID)
	} else {
		fmt.Printf("Conceptual Memory: No guest RAM (vm.guestMem was nil) to unmap for VM ID: %s\n", vm.ID)
	}

	// Clear the tracked memory regions
	if len(vm.memoryRegions) > 0 {
		fmt.Printf("Conceptual Memory: Clearing %d tracked memory regions for VM ID: %s.\n", len(vm.memoryRegions), vm.ID)
		vm.memoryRegions = make([]MemoryRegion, 0, 1)
	}

	return nil
}

// Start, Stop, Pause, Resume, GetStatus, GetLastError methods remain as defined in User Sub-Issue 1.2 (Basic VM State Management)
// with the following adjustment to Start():
func (vm *VirtualMachine) Start(kvmSystemFd int) error {
	vm.statusLock.Lock()
	// Defer unlock until after all status changes are certain

	if vm.Status != CREATED && vm.Status != STOPPED {
		vm.statusLock.Unlock()
		return fmt.Errorf("VM %s is in state %s, cannot start", vm.ID, vm.Status)
	}
	fmt.Printf("Conceptual VM: VM %s changing status from %s to STARTING.\n", vm.ID, vm.Status)
	vm.Status = STARTING
	vm.lastError = nil
	vm.statusLock.Unlock() // Unlock before potentially long operations like setupMemory

	// Ensure memory is set up. This might have been done at registration,
	// but good to ensure or re-check, especially if a VM can be reconfigured.
	// The setupMemory method itself is internally locked with resourceLock.
	if vm.guestMem == nil || len(vm.memoryRegions) == 0 {
		fmt.Printf("Conceptual VM: Memory not yet set up for VM %s, calling setupMemory().\n", vm.ID)
		if err := vm.setupMemory(); err != nil {
			vm.statusLock.Lock()
			vm.Status = FAILED
			vm.lastError = fmt.Errorf("memory setup failed during start for VM %s: %w", vm.ID, err)
			vm.statusLock.Unlock()
			return vm.lastError
		}
	}

	fmt.Println("Conceptual VM: --- BEGIN Full VM Start Sequence (Conceptual Stubs) ---")
	fmt.Printf("Conceptual VM: VM %s memory setup assumed complete (guestMem: %p).\n", vm.ID, vm.guestMem)

	fmt.Printf("Conceptual VM: %d vCPUs would be created and initialized for VM %s.\n", vm.Config.GetVcpuConfig().GetCount(), vm.ID)
	if len(vm.vcpus) == 0 {
		for i := 0; i < int(vm.Config.GetVcpuConfig().GetCount()); i++ {
			vcpu_placeholder_fd := 2000 + i
			// In a real implementation, NewVCPU would be called here, using kvmSystemFd.
			// For this conceptual step, we just add placeholder VCPU structs.
			vm.resourceLock.Lock()
			// VCPU needs a reference to its parent VM for I/O dispatch (e.g., to serial ports)
			vm.vcpus = append(vm.vcpus, &VCPU{id: i, vmFd: vm.vmFd, vcpuFd: vcpu_placeholder_fd, vm: vm})
			vm.resourceLock.Unlock()
		}
	}

	// Initialize I/O Devices (including Serial Ports)
	if err := vm.initializeDevices(); err != nil {
		vm.statusLock.Lock()
		vm.Status = FAILED
		vm.lastError = fmt.Errorf("device initialization failed for VM %s: %w", vm.ID, err)
		vm.statusLock.Unlock()
		return vm.lastError
	}
	// Log message for device initialization is now within initializeDevices()

	fmt.Printf("Conceptual VM: KVM_RUN loops for %d vCPUs would be launched in goroutines for VM %s.\n", len(vm.vcpus), vm.ID)
	fmt.Println("Conceptual VM: --- END Full VM Start Sequence (Conceptual Stubs) ---")

	vm.statusLock.Lock()
	vm.Status = RUNNING
	vm.statusLock.Unlock()
	fmt.Printf("Conceptual VM: VM %s status changed to RUNNING.\n", vm.ID)
	return nil
}

// Stop, Pause, Resume, GetStatus, GetLastError as previously defined.
// ... (rest of the methods from User Sub-Issue 1.2) ...
func (vm *VirtualMachine) Stop(force bool) error {
	vm.statusLock.Lock()
	// Defer unlock until after all status changes are certain

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
	vm.statusLock.Unlock() // Unlock for potentially long cleanup operations

	fmt.Println("Conceptual VM: --- BEGIN Full VM Stop Sequence (Conceptual Stubs) ---")
	fmt.Printf("Conceptual VM: Signaling %d vCPU run loops to exit for VM %s.\n", len(vm.vcpus), vm.ID)
	fmt.Printf("Conceptual VM: Conceptually waiting for vCPU goroutines to finish for VM %s.\n", vm.ID)

	vm.resourceLock.Lock()
	if len(vm.vcpus) > 0 {
		fmt.Printf("Conceptual VM: vCPUs for VM %s would be closed and cleaned up.\n", vm.ID)
		// for _, v_cpu := range vm.vcpus { v_cpu.Close() } // Conceptual close
		vm.vcpus = nil
	}
	vm.resourceLock.Unlock()

	// Cleanup I/O Devices (including Serial Ports)
	if err := vm.cleanupDevices(); err != nil {
		fmt.Printf("Warning: Error during device cleanup in Stop for VM %s: %v\n", vm.ID, err)
		// Potentially set vm.lastError, but continue stopping
	}

	// For a full stop that intends to release all resources except the VM definition itself,
	// cleanupMemory is appropriate here. If Stop is just to halt execution for a later Start,
	// memory might be kept. The current DeleteVM handles the definitive cleanup.
	// Let's assume Stop also cleans up the memory mapping.
	if err := vm.cleanupMemory(); err != nil {
		 fmt.Printf("Warning: Error during memory cleanup in Stop for VM %s: %v\n", vm.ID, err)
		 // Potentially set vm.lastError
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


// VMManager struct and its methods (NewVMManager, GetVM, ListVMs) remain as previously defined.
// RegisterNewVM and DeleteVM need to be updated for memory setup/cleanup calls.
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
	// Ensure the config object has the final vm_id.
	// If config is used elsewhere, this might modify it unexpectedly.
	// It might be better to store vmID only in VirtualMachine.ID and pass config as is.
	// For now, updating config.VmId if it was generated.
	if config.VmId == "" && idSuggestion == "" { // Only if we truly generated it
	    // This part is tricky, as pb.VMConfig might not be mutable if it's a copy.
	    // Let's assume config is a pointer and we can modify it, or that vm.Config is the source of truth.
	    // For simplicity, we'll assume vm.Config which is a pointer is the one to rely on.
	    // The actual config object used for VM creation should have the correct ID.
	    // This implies NewVirtualMachine should ensure config.VmId is set if vmID is generated.
	    // Let's refine NewVirtualMachine to handle this or assume config passed in is already final.
	    // For now, let vm.Config reflect the given config, and vm.ID be the manager's source of truth.
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

	// Initial memory setup is now part of VMManager's responsibility during registration.
	// It requires vmFd, so it's done after hypervisor.CreateVM.
	if err := vm.setupMemory(); err != nil {
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
			vm.statusLock.Lock()
			vm.Status = STOPPED
			vm.lastError = fmt.Errorf("forced stop during deletion: %w", err)
			vm.statusLock.Unlock()
		}
	}

	// Ensure vCPUs are cleaned (should be handled by vm.Stop)
	if len(vm.vcpus) > 0 {
		fmt.Printf("Conceptual VMManager: Ensuring VCPUs are cleaned for VM %s post-stop.\n", vmID)
		vm.resourceLock.Lock()
		// for _, v_cpu := range vm.vcpus { if v_cpu != nil { v_cpu.Close() } } // Conceptual close
		vm.vcpus = nil
		vm.resourceLock.Unlock()
	}

	if err := vm.cleanupMemory(); err != nil {
		fmt.Printf("Error cleaning up memory for VM %s during delete: %v. Proceeding.\n", vmID, err)
	}

	// Ensure devices are cleaned up if vm.Stop didn't fully run or if VM was not in a running state.
	if len(vm.serialPorts) > 0 { // Check one of the device slices
		fmt.Printf("Conceptual VMManager: Ensuring devices are cleaned for VM %s during delete (post-stop/failed state).\n", vmID)
		if err := vm.cleanupDevices(); err != nil {
			fmt.Printf("Error during final device cleanup for VM %s during delete: %v. Proceeding.\n", vmID, err)
		}
	} else {
		fmt.Printf("Conceptual VMManager: No devices appeared to need explicit cleanup for VM %s during delete (already cleaned by Stop or none existed).\n", vmID)
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

// initializeDevices creates and configures all I/O devices for the VM based on its Config.
func (vm *VirtualMachine) initializeDevices() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	fmt.Printf("Conceptual VM %s: Initializing I/O devices...\n", vm.ID)

	// Initialize Serial Ports
	if vm.Config.GetSerialPorts() != nil && len(vm.Config.GetSerialPorts()) > 0 {
		// Ensure slice is clean if this method could be called multiple times (though not typical for init)
		vm.serialPorts = make([]*SerialPortDevice, 0, len(vm.Config.GetSerialPorts()))
		fmt.Printf("Conceptual VM %s: Found %d serial port configs.\n", vm.ID, len(vm.Config.GetSerialPorts()))
		for i, spConfig := range vm.Config.GetSerialPorts() {
			var ioBase uint16
			if spConfig.GetIoBaseAddressOverride() != 0 {
				ioBase = uint16(spConfig.GetIoBaseAddressOverride())
			} else {
				// Assign default I/O bases for COM1, COM2, etc.
				switch i { // Assuming order in config maps to COM1, COM2 ...
				case 0:
					ioBase = DEFAULT_SERIAL_IO_BASE_COM1
				case 1:
					ioBase = DEFAULT_SERIAL_IO_BASE_COM2
				// Add more cases for COM3, COM4 if needed
				default:
					fmt.Printf("Warning: Too many serial ports defined for default I/O base assignment (port index %d for VM %s). Skipping this serial port.\n", i, vm.ID)
					continue // Skip this serial port config
				}
				// This check was inside the switch, ioBase would be 0 if default case hit and continued.
				// if ioBase == 0 { continue } // Skip if no valid base assigned - redundant due to continue above
			}

			deviceID := spConfig.GetId()
			if deviceID == "" {
				deviceID = fmt.Sprintf("com%d", i+1) // Default ID if not provided
			}

			outputToStdOut := true // Default for conceptual simplicity
			if spConfig.GetType() == pb.SerialPortConfig_PTY { // Check against protobuf enum
				outputToStdOut = false
			}
			// Add other types like FILE, TCP_SOCKET later from pb.SerialPortConfig_SerialPortType

			serialDev, err := NewSerialPortDevice(deviceID, ioBase, outputToStdOut /*, spConfig */)
			if err != nil {
				// Log error and continue trying to initialize other devices? Or fail fast?
				// For now, fail fast for any device initialization error.
				return fmt.Errorf("failed to create serial port %s for VM %s: %w", deviceID, vm.ID, err)
			}
			vm.serialPorts = append(vm.serialPorts, serialDev)
			fmt.Printf("Conceptual VM %s: Serial port '%s' (index %d) initialized at I/O base 0x%X.\n", vm.ID, deviceID, i, ioBase)
		}
	} else {
		fmt.Printf("Conceptual VM %s: No serial port configurations found.\n", vm.ID)
	}

	// Conceptual: Initialize VirtIO-blk devices (User Sub-Issue 2.2's virtio_blk.go)
	// if vm.Config.GetStorageDevices() != nil {
	//    vm.virtioBlkDevices = make([]*VirtIOBlkDevice, 0, len(vm.Config.GetStorageDevices()))
	//    for _, storageConf := range vm.Config.GetStorageDevices() {
	//        if storageConf.GetControllerType() == pb.StorageDevice_VIRTIO_BLK {
	//            // virtioBlkDev, err := NewVirtIOBlkDevice(vm, storageConf, storageConf.GetDiskId())
	//            // if err != nil { return fmt.Errorf("failed to init virtio-blk %s: %w", storageConf.GetDiskId(), err) }
	//            // vm.virtioBlkDevices = append(vm.virtioBlkDevices, virtioBlkDev)
	//            // TODO: Register virtioBlkDev with PCI/MMIO bus emulator for the VM
	//            fmt.Printf("Conceptual VM %s: VirtIO-blk device '%s' would be initialized here.\n", vm.ID, storageConf.GetDiskId())
	//        }
	//    }
	// }


	// Conceptual: Initialize VirtIO-net devices (User Sub-Issue 2.4's virtio_net.go)
	// if vm.Config.GetNetworkInterfaces() != nil {
	//    vm.virtioNetDevices = make([]*VirtIONetDevice, 0, len(vm.Config.GetNetworkInterfaces()))
	//    for _, netConf := range vm.Config.GetNetworkInterfaces() {
	//        if netConf.GetVnicModel() == pb.NetworkInterface_VIRTIO_NET {
	//            // virtioNetDev, err := NewVirtIONetDevice(vm, netConf, netConf.GetNicId())
	//            // if err != nil { return fmt.Errorf("failed to init virtio-net %s: %w", netConf.GetNicId(), err) }
	//            // vm.virtioNetDevices = append(vm.virtioNetDevices, virtioNetDev)
	//            // err = AddTapToBridge(virtioNetDev.tapName, netConf.GetNetworkAttachmentId()) // Host setup
	//            // if err != nil { return fmt.Errorf("failed to add tap %s to bridge for %s: %w", virtioNetDev.tapName, netConf.GetNicId(), err)}
	//            // TODO: Register virtioNetDev with PCI/MMIO bus emulator for the VM
	//            fmt.Printf("Conceptual VM %s: VirtIO-net device '%s' would be initialized here.\n", vm.ID, netConf.GetNicId())
	//        }
	//    }
	// }

	fmt.Printf("Conceptual VM %s: All I/O devices conceptually initialized.\n", vm.ID)
	return nil
}

// cleanupDevices closes and cleans up all I/O devices for the VM.
func (vm *VirtualMachine) cleanupDevices() error {
	vm.resourceLock.Lock()
	defer vm.resourceLock.Unlock()

	fmt.Printf("Conceptual VM %s: Cleaning up I/O devices...\n", vm.ID)
	var lastErr error

	// Cleanup Serial Ports
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
				lastErr = err // Keep last error encountered
			}
		}
		vm.serialPorts = nil // Clear the slice
	} else {
		fmt.Printf("Conceptual VM %s: No serial ports to clean up.\n", vm.ID)
	}

	// Conceptual: Cleanup VirtIO-blk devices
	// if len(vm.virtioBlkDevices) > 0 {
	//    fmt.Printf("Conceptual VM %s: Cleaning up %d VirtIO-blk device(s).\n", vm.ID, len(vm.virtioBlkDevices))
	//    for _, blkDev := range vm.virtioBlkDevices { if blkDev != nil { blkDev.Close() } }
	//    vm.virtioBlkDevices = nil
	// }


	// Conceptual: Cleanup VirtIO-net devices
	// if len(vm.virtioNetDevices) > 0 {
	//    fmt.Printf("Conceptual VM %s: Cleaning up %d VirtIO-net device(s).\n", vm.ID, len(vm.virtioNetDevices))
	//    for _, netDev := range vm.virtioNetDevices { if netDev != nil { netDev.Close() } } // This would also delete TAP
	//    vm.virtioNetDevices = nil
	// }

	if lastErr != nil {
		return fmt.Errorf("encountered error(s) during device cleanup for VM %s: %w", vm.ID, lastErr)
	}
	fmt.Printf("Conceptual VM %s: All I/O devices conceptually cleaned up.\n", vm.ID)
	return nil
}
