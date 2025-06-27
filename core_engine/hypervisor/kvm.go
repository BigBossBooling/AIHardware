package hypervisor

import (
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// VMState represents the state of a virtual machine.
type VMState int

const (
	StateUnknown   VMState = iota
	StateCreating
	StateRunning
	StatePaused
	StateStopping
	StateStopped
	StateError
)

func (s VMState) String() string {
	switch s {
	case StateCreating:
		return "Creating"
	case StateRunning:
		return "Running"
	case StatePaused:
		return "Paused"
	case StateStopping:
		return "Stopping"
	case StateStopped:
		return "Stopped"
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// KVMHypervisor represents the KVM hypervisor context.
type KVMHypervisor struct {
	kvmFd *os.File
}

// VirtualMachine represents a KVM virtual machine.
type VirtualMachine struct {
	vmFd  int
	state VMState
	mu    sync.Mutex

	memoryRegions []*MemoryRegion
	vcpus         []*VCPU
}

// VCPU represents a virtual CPU.
type VCPU struct {
	id         uint
	fd         int
	kvmRunData []byte
	vm         *VirtualMachine
	runLoopExitChan chan struct{}
	runLoopDoneChan chan struct{}
}

// MemoryRegion describes a KVM memory region.
type MemoryRegion struct {
	Slot          uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	HostUserAddr  uintptr
	backingStore  []byte
	Flags         uint32
	readOnly      bool
	logDirtyPages bool
}

// KvmUserspaceMemoryRegion is the Go equivalent of struct kvm_userspace_memory_region
type KvmUserspaceMemoryRegion struct {
	Slot          uint32
	Flags         uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	UserspaceAddr uintptr
}


func NewKVMHypervisor() (*KVMHypervisor, error) {
	log.Printf("Attempting to open KVM device at %s", kvmDevicePath)
	kvmFile, err := os.OpenFile(kvmDevicePath, os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		log.Printf("Error opening %s: %v", kvmDevicePath, err)
		return nil, fmt.Errorf("failed to open %s: %w", kvmDevicePath, err)
	}
	log.Printf("Successfully opened %s, fd: %d", kvmDevicePath, kvmFile.Fd())

	version, err := unix.IoctlRetInt(int(kvmFile.Fd()), ioctl_KVM_GET_API_VERSION)
	if err != nil {
		log.Printf("KVM_GET_API_VERSION ioctl failed for fd %d: %v", kvmFile.Fd(), err)
		kvmFile.Close()
		return nil, fmt.Errorf("KVM_GET_API_VERSION ioctl failed: %w", err)
	}
	log.Printf("KVM_GET_API_VERSION returned: %d", version)

	if version != KVM_API_VERSION {
		log.Printf("Unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
		kvmFile.Close()
		return nil, fmt.Errorf("unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
	}
	log.Println("KVMHypervisor initialized successfully.")
	return &KVMHypervisor{
		kvmFd: kvmFile,
	}, nil
}

func (h *KVMHypervisor) Close() error {
	if h.kvmFd != nil {
		log.Printf("Closing KVMHypervisor (fd: %d)", h.kvmFd.Fd())
		err := h.kvmFd.Close()
		if err != nil {
			log.Printf("Error closing KVMHypervisor fd %d: %v", h.kvmFd.Fd(), err)
			return fmt.Errorf("error closing KVM fd %d: %w", h.kvmFd.Fd(), err)
		}
		h.kvmFd = nil
		log.Printf("KVMHypervisor fd closed successfully.")
		return nil
	}
	log.Println("KVMHypervisor Close called on already closed or uninitialized instance.")
	return nil
}

func (h *KVMHypervisor) CreateVM(machineType ...uint64) (*VirtualMachine, error) {
	log.Println("KVMHypervisor CreateVM called.")
	if h.kvmFd == nil {
		log.Println("Error: CreateVM called on nil or closed KVMHypervisor.")
		return nil, fmt.Errorf("KVMHypervisor is not initialized or has been closed")
	}

	vmFd, err := unix.IoctlRetInt(int(h.kvmFd.Fd()), ioctl_KVM_CREATE_VM)
	if err != nil {
		log.Printf("KVM_CREATE_VM ioctl failed for KVM fd %d: %v", h.kvmFd.Fd(), err)
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl failed: %w", err)
	}
	if vmFd < 0 {
		log.Printf("KVM_CREATE_VM ioctl returned invalid fd: %d for KVM fd %d", vmFd, h.kvmFd.Fd())
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl returned invalid fd: %d", vmFd)
	}
	log.Printf("Successfully created VM with fd: %d from KVM fd: %d", vmFd, h.kvmFd.Fd())
	return &VirtualMachine{
		vmFd:   vmFd,
		state:  StateStopped,
	}, nil
}

// GetState returns the current state of the VM.
func (vm *VirtualMachine) GetState() VMState {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.state
}

// SetState sets the current state of the VM.
func (vm *VirtualMachine) SetState(newState VMState) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if vm.state != newState {
		log.Printf("VM (vmFd: %d) state changing from %s to %s", vm.vmFd, vm.state, newState)
		vm.state = newState
	}
}

func (vm *VirtualMachine) Close() error {
	log.Printf("VirtualMachine.Close called for vmFd: %d", vm.vmFd)
	vm.mu.Lock()

	// Stop all VCPUs first
	for _, vcpu := range vm.vcpus {
		if vcpu.runLoopExitChan != nil {
			select {
			case <-vcpu.runLoopExitChan: // Already closed
			default:
				close(vcpu.runLoopExitChan)
			}
		}
	}
	// Wait for all VCPU runLoops to finish
	for _, vcpu := range vm.vcpus {
		if vcpu.runLoopDoneChan != nil {
			<-vcpu.runLoopDoneChan
		}
		// Now that the runLoop is done, VCPU.Close can be called to clean vcpu fd and mmap
		if vcpu != nil { // Check if vcpu itself is nil before calling Close
			vcpu.Close()
		}
	}
	vm.vcpus = nil // Clear the slice

	vm.state = StateStopped
	vm.mu.Unlock()

	// Unmap user memory regions
	for i, region := range vm.memoryRegions {
		if region.backingStore != nil {
			log.Printf("Unmapping memory region slot %d (GPA: 0x%x, Size: 0x%x) for VM (vmFd: %d)",
				region.Slot, region.GuestPhysAddr, region.MemorySize, vm.vmFd)
			if err := unix.Munmap(region.backingStore); err != nil {
				log.Printf("Error unmapping memory region slot %d (HUA: 0x%x): %v", region.Slot, region.HostUserAddr, err)
			}
			region.backingStore = nil
			region.HostUserAddr = 0
		}
		vm.memoryRegions[i] = nil
	}
	vm.memoryRegions = nil


	if vm.vmFd > 0 {
		log.Printf("Closing VirtualMachine (vmFd: %d)", vm.vmFd)
		err := unix.Close(vm.vmFd)
		if err != nil {
			log.Printf("Error closing VirtualMachine vmFd %d: %v", vm.vmFd, err)
			return fmt.Errorf("failed to close VM fd %d: %w", vm.vmFd, err)
		}
		log.Printf("VirtualMachine vmFd %d closed successfully.", vm.vmFd)
		vm.vmFd = 0
		return nil
	}
	log.Println("VirtualMachine Close called on already closed or uninitialized vmFd.")
	return nil
}

func (vm *VirtualMachine) Start(numVCPUs uint) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state != StateStopped {
		log.Printf("VM cannot be started from state: %s (vmFd: %d)", vm.state, vm.vmFd)
		return fmt.Errorf("VM cannot be started from state: %s. Expected StateStopped.", vm.state)
	}
	if numVCPUs == 0 {
		return fmt.Errorf("cannot start VM with zero VCPUs")
	}

	log.Printf("Starting VM (vmFd: %d) with %d VCPU(s)", vm.vmFd, numVCPUs)
	vm.state = StateCreating

	vm.vcpus = make([]*VCPU, 0, numVCPUs)
	for i := uint(0); i < numVCPUs; i++ {
		vcpu, err := vm.createAndInitVCPU(i)
		if err != nil {
			log.Printf("Failed to create and init VCPU %d for VM %d: %v. Cleaning up.", i, vm.vmFd, err)
			for _, existingVCPU := range vm.vcpus {
				existingVCPU.Close()
			}
			vm.vcpus = nil
			vm.state = StateError
			return fmt.Errorf("failed to create VCPU %d: %w", i, err)
		}
		vm.vcpus = append(vm.vcpus, vcpu)
	}

	for _, vcpu := range vm.vcpus {
		go vcpu.runLoop()
	}

	vm.state = StateRunning
	log.Printf("VM (vmFd: %d) with %d VCPU(s) is now in state Running", vm.vmFd, len(vm.vcpus))
	return nil
}

func (vm *VirtualMachine) createAndInitVCPU(id uint) (*VCPU, error) {
	r1, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(vm.vmFd), uintptr(ioctl_KVM_CREATE_VCPU), uintptr(id))
	if errno != 0 {
		return nil, fmt.Errorf("KVM_CREATE_VCPU ioctl for vcpu_id %d failed: %w", id, errno)
	}
	vcpuFd := int(r1)
	if vcpuFd < 0 {
		return nil, fmt.Errorf("KVM_CREATE_VCPU ioctl for vcpu_id %d returned invalid fd %d", id, vcpuFd)
	}
	log.Printf("Created VCPU (id: %d, fd: %d) for VM (vmFd: %d)", id, vcpuFd, vm.vmFd)

	mmapSize, err := unix.IoctlRetInt(vcpuFd, ioctl_KVM_GET_VCPU_MMAP_SIZE)
	if err != nil {
		unix.Close(vcpuFd)
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE for vcpu %d (fd %d) failed: %w", id, vcpuFd, err)
	}
	if mmapSize <= 0 {
		unix.Close(vcpuFd)
		return nil, fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE for vcpu %d (fd %d) returned invalid size: %d", id, vcpuFd, mmapSize)
	}
	log.Printf("VCPU %d (fd %d) mmap size for kvm_run: %d bytes", id, vcpuFd, mmapSize)

	kvmRunData, err := unix.Mmap(vcpuFd, 0, mmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(vcpuFd)
		return nil, fmt.Errorf("failed to mmap kvm_run for vcpu %d (fd %d): %w", id, vcpuFd, err)
	}
	log.Printf("Successfully mmapped kvm_run for vcpu %d (fd %d)", id, vcpuFd)

	vcpu := &VCPU{
		id:              id,
		fd:              vcpuFd,
		kvmRunData:      kvmRunData,
		vm:              vm,
		runLoopExitChan: make(chan struct{}),
		runLoopDoneChan: make(chan struct{}),
	}
	return vcpu, nil
}

func (vcpu *VCPU) Close() error {
	log.Printf("Closing VCPU (id: %d, fd: %d)", vcpu.id, vcpu.fd)
	var firstErr error
	if vcpu.kvmRunData != nil {
		err := unix.Munmap(vcpu.kvmRunData)
		if err != nil {
			log.Printf("Error unmapping kvm_run for VCPU (id: %d, fd: %d): %v", vcpu.id, vcpu.fd, err)
			firstErr = err
		}
		vcpu.kvmRunData = nil
	}
	if vcpu.fd > 0 {
		err := unix.Close(vcpu.fd)
		if err != nil {
			log.Printf("Error closing VCPU fd (id: %d, fd: %d): %v", vcpu.id, vcpu.fd, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		vcpu.fd = 0
	}
	return firstErr
}

// runLoop is the main loop for this VCPU. It repeatedly calls KVM_RUN.
func (vcpu *VCPU) runLoop() {
	log.Printf("Starting runLoop for VCPU (id: %d, fd: %d) of VM (vmFd: %d)", vcpu.id, vcpu.fd, vcpu.vm.vmFd)
	defer log.Printf("Exiting runLoop for VCPU (id: %d, fd: %d) of VM (vmFd: %d)", vcpu.id, vcpu.fd, vcpu.vm.vmFd)
	defer close(vcpu.runLoopDoneChan)

	for {
		select {
		case <-vcpu.runLoopExitChan:
			log.Printf("runLoop received exit signal for VCPU (id: %d, fd: %d). Terminating.", vcpu.id, vcpu.fd)
			return
		default:
		}

		_, err := unix.IoctlRetInt(vcpu.fd, ioctl_KVM_RUN)
		if err != nil {
			if err == syscall.EINTR {
				log.Printf("KVM_RUN interrupted (EINTR) for VCPU (id: %d, fd: %d). Checking exit signal.", vcpu.id, vcpu.fd)
				continue
			}
			log.Printf("KVM_RUN ioctl failed for VCPU (id: %d, fd: %d): %v. Terminating runLoop.", vcpu.id, vcpu.fd, err)
			vcpu.vm.SetState(StateError)
			return
		}

		exitReason := GetExitReasonVerified(vcpu.kvmRunData)
		log.Printf("KVM_RUN exited on VCPU (id: %d, fd: %d) with reason: 0x%x (%s)", vcpu.id, vcpu.fd, exitReason, KvmExitReasonToString(exitReason))

		stopVM := false
		continueLoop := true

		switch exitReason {
		case KVM_EXIT_HLT:
			log.Printf("VCPU %d Halted.", vcpu.id)
			stopVM = true
			continueLoop = false
		case KVM_EXIT_SHUTDOWN:
			log.Printf("VCPU %d initiated shutdown.", vcpu.id)
			stopVM = true
			continueLoop = false
		case KVM_EXIT_IO:
			ioData := GetIoDataVerified(vcpu.kvmRunData)
			direction := "IN"
			if ioData.Direction == 1 { // KVM_EXIT_IO_OUT usually 1
				direction = "OUT"
			}
			log.Printf("KVM_EXIT_IO on VCPU %d: dir=%s port=0x%x size=%d count=%d data_offset=0x%x",
				vcpu.id, direction, ioData.Port, ioData.Size, ioData.Count, ioData.DataOffset)
			continueLoop = true
		case KVM_EXIT_FAIL_ENTRY:
			failEntry := GetFailEntryDataVerified(vcpu.kvmRunData)
			log.Printf("KVM_EXIT_FAIL_ENTRY on VCPU %d: hardware_entry_failure_reason=0x%x. Stopping VM.", vcpu.id, failEntry.HardwareEntryFailureReason)
			vcpu.vm.SetState(StateError)
			stopVM = true
			continueLoop = false
		case KVM_EXIT_INTERNAL_ERROR:
			internalErr := GetInternalErrorDataVerified(vcpu.kvmRunData)
			log.Printf("KVM_EXIT_INTERNAL_ERROR on VCPU %d: suberror=0x%x. Stopping VM. Data[0]=0x%x", vcpu.id, internalErr.Suberror, internalErr.Data[0])
			vcpu.vm.SetState(StateError)
			stopVM = true
			continueLoop = false
		default:
			log.Printf("Unhandled KVM exit reason 0x%x (%s) on VCPU %d. VM will be stopped.", exitReason, KvmExitReasonToString(exitReason), vcpu.id)
			stopVM = true
			continueLoop = false
		}

		if stopVM {
			currentState := vcpu.vm.GetState()
			if currentState != StateError && currentState != StateStopping && currentState != StateStopped {
				vcpu.vm.SetState(StateStopped)
			}
		}

		if !continueLoop {
			return
		}
	}
}

func (vm *VirtualMachine) Stop() error {
	vm.mu.Lock()
	log.Printf("Attempting to stop VM (vmFd: %d, current state: %s)", vm.vmFd, vm.state)

	if vm.state != StateRunning && vm.state != StatePaused && vm.state != StateCreating {
		if vm.state == StateStopping || vm.state == StateStopped {
			log.Printf("VM is already stopping or stopped (state: %s).", vm.state)
			vm.mu.Unlock()
			return nil
		}
		log.Printf("VM cannot be stopped from state: %s", vm.state)
		vm.mu.Unlock()
		return fmt.Errorf("VM cannot be stopped from state: %s", vm.state)
	}

	vm.state = StateStopping

	// Signal all VCPU runLoops to exit
	for _, vcpu := range vm.vcpus {
		if vcpu.runLoopExitChan != nil {
			select {
			case <-vcpu.runLoopExitChan:
			default:
				close(vcpu.runLoopExitChan)
			}
		}
	}
	vm.mu.Unlock() // Unlock before waiting

	// Wait for all VCPU runLoops to confirm exit
	for _, vcpu := range vm.vcpus {
		if vcpu.runLoopDoneChan != nil {
			<-vcpu.runLoopDoneChan
		}
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state != StateError {
		vm.state = StateStopped
	}
	log.Printf("VM (vmFd: %d) is now in state %s after Stop.", vm.vmFd, vm.state)
	return nil
}

func (vm *VirtualMachine) Pause() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	log.Printf("Attempting to pause VM (vmFd: %d, current state: %s)", vm.vmFd, vm.state)

	if vm.state != StateRunning {
		log.Printf("VM cannot be paused from state: %s", vm.state)
		return fmt.Errorf("VM cannot be paused from state: %s. Expected StateRunning.", vm.state)
	}

	vm.state = StatePaused
	log.Printf("VM (vmFd: %d) is now in state Paused (conceptually).", vm.vmFd)
	return nil
}

func (vm *VirtualMachine) Resume() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	log.Printf("Attempting to resume VM (vmFd: %d, current state: %s)", vm.vmFd, vm.state)

	if vm.state != StatePaused {
		log.Printf("VM cannot be resumed from state: %s", vm.state)
		return fmt.Errorf("VM cannot be resumed from state: %s. Expected StatePaused.", vm.state)
	}

	vm.state = StateRunning
	log.Printf("VM (vmFd: %d) is now in state Running (conceptually resumed).", vm.vmFd)
	return nil
}


func (vm *VirtualMachine) AddMemoryRegion(slot uint32, guestPhysAddr uint64, memorySize uint64, flags uint32, readOnly bool) (*MemoryRegion, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state == StateRunning || vm.state == StatePaused {
		log.Printf("Cannot add memory region to VM (vmFd: %d) while it is in state %s.", vm.vmFd, vm.state)
		return nil, fmt.Errorf("cannot add memory region while VM is %s", vm.state)
	}

	if memorySize == 0 {
		return nil, fmt.Errorf("memory size cannot be zero")
	}

	pageSize := uint64(os.Getpagesize())
	if memorySize%pageSize != 0 {
		log.Printf("Warning: memory size 0x%x is not page aligned (page size 0x%x). This might lead to issues.", memorySize, pageSize)
	}
	if guestPhysAddr%pageSize != 0 {
		log.Printf("Warning: guest physical address 0x%x is not page aligned (page size 0x%x). This might lead to issues.", guestPhysAddr, pageSize)
	}

	hostMem, err := unix.Mmap(-1, 0, int(memorySize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANONYMOUS|unix.MAP_SHARED)
	if err != nil {
		log.Printf("Failed to mmap host memory for VM (vmFd: %d), size %d: %v", vm.vmFd, memorySize, err)
		return nil, fmt.Errorf("failed to mmap host memory (size %d): %w", memorySize, err)
	}
	hostUserAddr := uintptr(unsafe.Pointer(&hostMem[0]))
	log.Printf("Mmapped host memory for VM (vmFd: %d): addr=0x%x, actual_slice_len=%d, requested_size=%d", vm.vmFd, hostUserAddr, len(hostMem), memorySize)

	var effectiveFlags uint32 = flags
	if readOnly {
		effectiveFlags |= KVM_MEM_READONLY
	}

	memRegionStruct := KvmUserspaceMemoryRegion{
		Slot:          slot,
		Flags:         effectiveFlags,
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    memorySize,
		UserspaceAddr: hostUserAddr,
	}

	log.Printf("Calling KVM_SET_USER_MEMORY_REGION for VM (vmFd: %d), slot: %d, GPA: 0x%x, size: 0x%x, HUA: 0x%x, flags: 0x%x",
		vm.vmFd, slot, guestPhysAddr, memorySize, hostUserAddr, effectiveFlags)

	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(vm.vmFd),
		uintptr(ioctl_KVM_SET_USER_MEMORY_REGION_FULL),
		uintptr(unsafe.Pointer(&memRegionStruct)),
	)

	if errno != 0 {
		log.Printf("KVM_SET_USER_MEMORY_REGION ioctl failed for VM (vmFd: %d): %v. Unmapping host memory.", vm.vmFd, errno)
		errUnmap := unix.Munmap(hostMem)
		if errUnmap != nil {
			log.Printf("Critical: Failed to unmap host memory after KVM_SET_USER_MEMORY_REGION failure: %v", errUnmap)
		}
		return nil, fmt.Errorf("KVM_SET_USER_MEMORY_REGION ioctl failed: %w", errno)
	}

	log.Printf("Successfully set user memory region for VM (vmFd: %d), slot: %d", vm.vmFd, slot)

	region := &MemoryRegion{
		Slot:          slot,
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    memorySize,
		HostUserAddr:  hostUserAddr,
		backingStore:  hostMem,
		Flags:         effectiveFlags,
		readOnly:      (effectiveFlags & KVM_MEM_READONLY) != 0,
		logDirtyPages: (effectiveFlags & KVM_MEM_LOG_DIRTY_PAGES) != 0,
	}
	vm.memoryRegions = append(vm.memoryRegions, region)

	return region, nil
}
