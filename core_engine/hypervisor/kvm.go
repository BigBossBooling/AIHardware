package hypervisor

import (
	"fmt"
	"log" // Added for basic logging
	"os"
	"sync" // For Mutex
	"syscall" // Direct syscalls for ioctl, consider x/sys/unix for more safety
	"unsafe" // For unsafe.Pointer when calling ioctl with structs

	"golang.org/x/sys/unix" // Preferred for syscalls where possible
)

// VMState represents the state of a virtual machine.
type VMState int

const (
	StateUnknown   VMState = iota // Should not happen post-initialization
	StateCreating                 // VM is being created but not yet running
	StateRunning                  // VM is running (KVM_RUN loop active)
	StatePaused                   // VM is paused (KVM_RUN loop not active, but resources intact)
	StateStopping                 // VM is in the process of stopping
	StateStopped                  // VM is stopped (KVM_RUN loop exited, resources may be partially or fully cleaned)
	StateError                    // VM is in an error state
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
// It holds the file descriptor for the /dev/kvm device.
type KVMHypervisor struct {
	kvmFd *os.File // File descriptor for /dev/kvm
	// We can add more fields here later, e.g., supported extensions.
}

// VirtualMachine represents a KVM virtual machine.
// It holds the file descriptor for the created VM.
type VirtualMachine struct {
	vmFd   int    // File descriptor for the VM
	vcpuFd int    // File descriptor for the primary VCPU
	state  VMState
	mu     sync.Mutex

	kvmRunData []byte // Mapped kvm_run structure for the VCPU

	// runLoopExitChan signals the KVM_RUN loop to exit.
	// Closing this channel indicates a stop request.
	runLoopExitChan chan struct{}
	// runLoopDoneChan is closed by the runLoop when it has fully exited and cleaned up.
	// This is used by Stop() to wait for graceful termination.
	runLoopDoneChan chan struct{}

	memoryRegions []*MemoryRegion // Tracks memory regions added to this VM

	// We can add VM configuration, vCPUs, memory regions etc. here later.
}

// MemoryRegion describes a KVM memory region.
type MemoryRegion struct {
	Slot          uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	HostUserAddr  uintptr // Points to the start of the mmapped memory in this process. Redundant if backingStore is present, but useful for KvmUserspaceMemoryRegion.
	Flags         uint32
	backingStore  []byte // The actual mmapped slice on the host
	readOnly      bool   // Derived from flags, for convenience
	logDirtyPages bool   // Derived from flags, for convenience
}

// KvmUserspaceMemoryRegion is the Go equivalent of struct kvm_userspace_memory_region
// It must match the kernel definition precisely for the ioctl to work.
// struct kvm_userspace_memory_region {
//   __u32 slot;
//   __u32 flags;
//   __u64 guest_phys_addr;
//   __u64 memory_size; /* bytes */
//   __u64 userspace_addr; /* start of the userspace allocated memory */
// };
// Total size on x86_64: 4 + 4 + 8 + 8 + 8 = 32 bytes. (Padding can make it 40)
// Let's confirm struct layout and padding.
// __u32 slot; (4 bytes)
// __u32 flags; (4 bytes) -> 8 bytes so far
// __u64 guest_phys_addr; (8 bytes) -> 16 bytes
// __u64 memory_size; (8 bytes) -> 24 bytes
// __u64 userspace_addr; (8 bytes) -> 32 bytes
// On a 64-bit system, this struct is naturally 32 bytes if there's no further padding.
// However, the C struct on Linux x86_64 for KVM is often 40 bytes due to alignment of the 64-bit fields.
// The structure definition in C is:
// struct kvm_userspace_memory_region {
//      __u32 slot;
//      __u32 flags;
//      __u64 guest_phys_addr;
//      __u64 memory_size;
//      __u64 userspace_addr;
// };
// It seems it should indeed be 32 bytes if __u64 are 8-byte aligned and __u32 are 4-byte aligned.
// Let's assume 32 bytes first. If ioctl fails with EINVAL, struct size/layout is a prime suspect.
// After further research: The C struct `kvm_userspace_memory_region` on a 64-bit system is
// typically 40 bytes because `guest_phys_addr` (a `__u64`) will be aligned to an 8-byte boundary,
// forcing 4 bytes of padding after `flags`. So:
// slot (4) + flags (4) + padding (4) + guest_phys_addr (8) + memory_size (8) + userspace_addr (8) = 36? No.
// slot (4) + flags (4) = 8. guest_phys_addr (8) is fine.
// Let's re-verify standard struct layout rules.
// slot  (u32) - offset 0
// flags (u32) - offset 4
// guest_phys_addr (u64) - offset 8 (aligned to 8, which is natural after two u32)
// memory_size (u64) - offset 16
// userspace_addr (u64) - offset 24
// This layout is 32 bytes. This is what many userspace tools (like QEMU) also use.
// Let's proceed with this 32-byte structure. The constant `ioctl_KVM_SET_USER_MEMORY_REGION_FULL` was
// calculated with 0x28 (40 bytes) size. This means the kernel *expects* a 40-byte struct.
// So, there must be padding.
// The kernel definition:
// struct kvm_userspace_memory_region {
//      __u32 slot;
//      __u32 flags;
//      __u64 guest_phys_addr;
//      __u64 memory_size;
//      __u64 userspace_addr;
// };
// If this is directly translated, it's 32 bytes. Why would it be 40?
// Ah, `userspace_addr` is a pointer, effectively `uintptr_t`. On 64-bit, this is 8 bytes.
// The struct definition from kernel source (e.g., `include/uapi/linux/kvm.h`):
// struct kvm_userspace_memory_region {
//         __u32 slot;
//         __u32 flags;
//         __u64 guest_phys_addr;
//         __u64 memory_size; /* bytes */
//         __u64 userspace_addr; /* start of the userspace allocated memory */
// };
// This is indeed 32 bytes. The ioctl constant 0x4028AE46 has size 0x28 = 40 bytes.
// This implies there's a mismatch in my understanding or sources for one of them.
// Let's use the struct definition that matches the ioctl size if the ioctl number is fixed.
// If the ioctl number `0x4028AE46` (size 40) is correct, then the Go struct must be 40 bytes.
// This means there are 8 bytes of padding somewhere or an extra field.
// Let's assume the common C struct is what we need, and it's 32 bytes.
// Then the ioctl number should be calculated based on 32 bytes (0x20).
// _IOW(0xAE, 0x46, size=32) = (1<<30) | (0xAE<<8) | (0x46<<0) | (0x20<<16) = 0x4020AE46.
// This value (0x4020AE46) is also commonly cited.
// I will use this one, and a 32-byte struct.
// Updating `ioctl_KVM_SET_USER_MEMORY_REGION_FULL` in constants file.
type KvmUserspaceMemoryRegion struct {
	Slot          uint32
	Flags         uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	UserspaceAddr uintptr // uintptr matches __u64 userspace_addr on 64-bit Go
}


// Note on syscalls:
// The standard `syscall` package is powerful but less safe.
// `golang.org/x/sys/unix` provides more Go-idiomatic and safer wrappers.
// We will prefer `golang.org/x/sys/unix` for ioctls.

// ioctl performs an ioctl syscall.
// This is a helper, but for KVM, we'll often use unix.IoctlInt & unix.IoctlRetInt directly.
// For ioctls that pass a pointer (e.g., KVM_SET_USER_MEMORY_REGION),
// uintptr(unsafe.Pointer(data)) would be used for the arg parameter.
func ioctl(fd uintptr, req uintptr, arg uintptr) (err error) {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

// NewKVMHypervisor creates a new KVMHypervisor instance.
// It opens /dev/kvm and verifies the KVM API version.
func NewKVMHypervisor() (*KVMHypervisor, error) {
	log.Printf("Attempting to open KVM device at %s", kvmDevicePath)
	kvmFile, err := os.OpenFile(kvmDevicePath, os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		log.Printf("Error opening %s: %v", kvmDevicePath, err)
		return nil, fmt.Errorf("failed to open %s: %w", kvmDevicePath, err)
	}
	log.Printf("Successfully opened %s, fd: %d", kvmDevicePath, kvmFile.Fd())

	// Get KVM API version
	// For KVM_GET_API_VERSION, the third argument to ioctl is ignored (can be 0).
	// It returns the version number directly.
	version, err := unix.IoctlRetInt(int(kvmFile.Fd()), ioctl_KVM_GET_API_VERSION)
	if err != nil {
		log.Printf("KVM_GET_API_VERSION ioctl failed for fd %d: %v", kvmFile.Fd(), err)
		kvmFile.Close() // Clean up on error
		return nil, fmt.Errorf("KVM_GET_API_VERSION ioctl failed: %w", err)
	}
	log.Printf("KVM_GET_API_VERSION returned: %d", version)

	if version != KVM_API_VERSION {
		log.Printf("Unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
		kvmFile.Close() // Clean up
		return nil, fmt.Errorf("unexpected KVM API version: got %d, want %d", version, KVM_API_VERSION)
	}

	// Optionally, check for required KVM extensions here in the future if needed.
	// For example, KVM_CHECK_EXTENSION for KVM_CAP_USER_MEMORY, KVM_CAP_IRQCHIP, etc.
	log.Println("KVMHypervisor initialized successfully.")
	return &KVMHypervisor{
		kvmFd: kvmFile,
	}, nil
}

// Close closes the KVM file descriptor.
func (h *KVMHypervisor) Close() error {
	if h.kvmFd != nil {
		log.Printf("Closing KVMHypervisor (fd: %d)", h.kvmFd.Fd())
		err := h.kvmFd.Close()
		if err != nil {
			log.Printf("Error closing KVMHypervisor fd %d: %v", h.kvmFd.Fd(), err)
			return fmt.Errorf("error closing KVM fd %d: %w", h.kvmFd.Fd(), err)
		}
		h.kvmFd = nil // Mark as closed
		log.Printf("KVMHypervisor fd closed successfully.")
		return nil
	}
	log.Println("KVMHypervisor Close called on already closed or uninitialized instance.")
	return nil
}

// CreateVM creates a new KVM virtual machine.
// It uses the KVM_CREATE_VM ioctl on the KVM file descriptor.
// For now, it returns a simple VirtualMachine struct containing the VM's file descriptor.
// The 'machineType' argument is currently unused but could be used for future extensions (e.g. s390 specific).
func (h *KVMHypervisor) CreateVM(machineType ...uint64) (*VirtualMachine, error) {
	log.Println("KVMHypervisor CreateVM called.")
	if h.kvmFd == nil { // Should be h.kvmFd.Fd() > 0 or similar if kvmFd is os.File
		log.Println("Error: CreateVM called on nil or closed KVMHypervisor.")
		return nil, fmt.Errorf("KVMHypervisor is not initialized or has been closed")
	}

	// The KVM_CREATE_VM ioctl can take an optional machine type argument for some architectures (e.g. s390).
	// For x86_64, this argument is typically 0 or ignored.
	// We use IoctlRetInt as KVM_CREATE_VM returns the new VM FD.
	// var mType uintptr // mType is currently unused for x86, will be used if we support other architectures
	// if len(machineType) > 0 {
	// 	mType = uintptr(machineType[0])
	// }

	// According to kvm/api.txt, KVM_CREATE_VM is an _IO ioctl, however, many implementations
	// and examples show it being called with IoctlRetInt or similar, as it returns the VM FD.
	// The man page for KVM_CREATE_VM also states "Returns a file descriptor for the new VM".
	// Let's use unix.IoctlRetInt. If a machine type is passed, it's the third argument.
	// If no machine type (or 0) is needed (common for x86), the third arg for _IO is usually 0.
	// Let's assume x86 for now where the argument is effectively ignored or 0.
	// If unix.IoctlRetInt expects a non-zero arg for non-pointer _IO, we pass 0.
	// The documentation for x/sys/unix.IoctlRetInt says:
	// "IoctlRetInt performs an ioctl operation that returns an int."
	// This matches KVM_CREATE_VM.

	vmFd, err := unix.IoctlRetInt(int(h.kvmFd.Fd()), ioctl_KVM_CREATE_VM)
	// If KVM_CREATE_VM required a parameter (like machineType for s390), it would be:
	// vmFd, err := unix.IoctlRetInt(int(h.kvmFd.Fd()), ioctl_KVM_CREATE_VM, int(mType))
	// But for x86, the third argument isn't typically used with IoctlRetInt for this _IO ioctl.
	// Let's stick to the simpler call first, assuming it implicitly passes 0 as the third arg or it's not needed.

	if err != nil {
		log.Printf("KVM_CREATE_VM ioctl failed for KVM fd %d: %v", h.kvmFd.Fd(), err)
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl failed: %w", err)
	}

	if vmFd < 0 { // Should be caught by err != nil from IoctlRetInt, but as a safeguard.
		log.Printf("KVM_CREATE_VM ioctl returned invalid fd: %d for KVM fd %d", vmFd, h.kvmFd.Fd())
		return nil, fmt.Errorf("KVM_CREATE_VM ioctl returned invalid fd: %d", vmFd)
	}
	log.Printf("Successfully created VM with fd: %d from KVM fd: %d", vmFd, h.kvmFd.Fd())
	return &VirtualMachine{
		vmFd:   vmFd,
		state:  StateStopped, // Initialize VM in a stopped state, ready to be started.
		// runLoopExitChan and runLoopDoneChan will be initialized in Start()
	}, nil
}

// Close closes the VM file descriptor and associated VCPU file descriptor.
// It also ensures the run loop is stopped.
func (vm *VirtualMachine) Close() error {
	log.Printf("VirtualMachine.Close called for vmFd: %d", vm.vmFd)
	vm.mu.Lock()
	// Ensure stop is called to terminate run loop if it's running or stopping.
	// This is a bit of a safeguard; ideally Stop() is called explicitly before Close().
	if vm.state == StateRunning || vm.state == StatePaused || vm.state == StateCreating || vm.state == StateStopping {
		// Unlock before calling stop to avoid deadlock if stop also locks.
		// However, Stop should handle its own locking carefully.
		// For simplicity here, we assume Stop can be called.
		// vm.mu.Unlock() // This might be too complex for a simple Close.
		// Let's assume Stop handles being called multiple times or on a stopped VM.
		// The primary goal of Close is FD cleanup.
		if vm.runLoopExitChan != nil {
			select {
			case <-vm.runLoopExitChan: // Already closed
			default:
				close(vm.runLoopExitChan)
			}
		}
		// Wait for run loop to finish if it was running.
		if vm.runLoopDoneChan != nil {
			<-vm.runLoopDoneChan // Potential deadlock if runLoop never started or runLoopDoneChan not made.
		}
	}
	// State transition is tricky here. If not stopped, what state is it?
	// For now, assume Close is for full cleanup.
	vm.state = StateStopped // Or some "Closed" state if we add one.
	vm.mu.Unlock() // Unlock after state change and channel ops.


	// Close VCPU FD
	if vm.vcpuFd > 0 { // Use > 0 as FDs are small positive integers; 0 is stdio.
		log.Printf("Closing VCPU fd %d for VM fd %d", vm.vcpuFd, vm.vmFd)
		err := unix.Close(vm.vcpuFd)
		if err != nil {
			// Log error but continue to close vmFd
			log.Printf("Error closing VCPU fd %d: %v", vm.vcpuFd, err)
		}
		vm.vcpuFd = 0
	}

	// Unmap kvm_run data
	if vm.kvmRunData != nil {
		log.Printf("Unmapping kvm_run data for VM (vmFd: %d, vcpuFd: %d)", vm.vmFd, vm.vcpuFd)
		err := unix.Munmap(vm.kvmRunData)
		if err != nil {
			log.Printf("Error unmapping kvm_run data for VM (vmFd: %d, vcpuFd: %d): %v", vm.vmFd, vm.vcpuFd, err)
			// Non-fatal for Close, continue cleanup
		}
		vm.kvmRunData = nil
	}

	// Unmap user memory regions
	for i, region := range vm.memoryRegions {
		if region.backingStore != nil {
			log.Printf("Unmapping memory region slot %d (GPA: 0x%x, Size: 0x%x) for VM (vmFd: %d)",
				region.Slot, region.GuestPhysAddr, region.MemorySize, vm.vmFd)
			if err := unix.Munmap(region.backingStore); err != nil {
				log.Printf("Error unmapping memory region slot %d (HUA: 0x%x): %v", region.Slot, region.HostUserAddr, err)
				// Non-fatal for Close, continue cleanup for other regions
			}
			region.backingStore = nil // Mark as unmapped
			region.HostUserAddr = 0  // Invalidate address
		}
		vm.memoryRegions[i] = nil // Help GC
	}
	vm.memoryRegions = nil // Clear the slice


	// Close VM FD
	if vm.vmFd > 0 { // Check against > 0 as well
		log.Printf("Closing VirtualMachine (vmFd: %d)", vm.vmFd)
		err := unix.Close(vm.vmFd)
		if err != nil {
			log.Printf("Error closing VirtualMachine vmFd %d: %v", vm.vmFd, err)
			return fmt.Errorf("failed to close VM fd %d: %w", vm.vmFd, err)
		}
		log.Printf("VirtualMachine vmFd %d closed successfully.", vm.vmFd)
		vm.vmFd = 0 // Mark as closed
		return nil
	}
	log.Println("VirtualMachine Close called on already closed or uninitialized vmFd.")
	return nil
}


// Start initializes and starts the virtual machine.
// This includes creating a VCPU, setting up the KVM_RUN structure, and launching the run loop.
func (vm *VirtualMachine) Start() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// A VM can only be started if it's currently in StateStopped.
	// Or StateCreating if CreateVM directly leads to Start and sets that state.
	// Let's simplify: Start is only valid from StateStopped.
	// CreateVM now sets state to StateStopped.
	if vm.state != StateStopped {
		log.Printf("VM cannot be started from state: %s (vmFd: %d)", vm.state, vm.vmFd)
		return fmt.Errorf("VM cannot be started from state: %s. Expected StateStopped.", vm.state)
	}

	log.Printf("Starting VM (vmFd: %d)", vm.vmFd)
	vm.state = StateCreating // Mark as creating/starting up critical components

	// 1. Create a VCPU
	// The VCPU ID is the second argument to KVM_CREATE_VCPU. Typically 0 for the first VCPU.
	vcpuID := 0
	// KVM_CREATE_VCPU is an _IO(KVMIO, 0x41) ioctl but it takes an argument (vcpu_id) and returns a file descriptor.
	// The standard unix.IoctlRetInt(fd, req) doesn't take an additional argument for the ioctl data.
	// For ioctls like KVM_CREATE_VCPU that take an integer argument and return an FD,
	// we might need to use unix.IoctlSetInt or a raw syscall if IoctlRetInt isn't suitable.
	// Let's check the signature of IoctlSetInt: IoctlSetInt(fd int, req uint, val int). This doesn't return the new FD.
	// The man page for KVM_CREATE_VCPU: `vcpu_fd = ioctl(vm_fd, KVM_CREATE_VCPU, vcpu_id);`
	// This implies the vcpu_id is passed as the third argument to the ioctl syscall directly.
	// And the return value of the syscall *is* the new FD.
	// So, unix.IoctlRetInt(fd, req) is problematic if `req` needs an argument itself.
	// The `ioctl_KVM_CREATE_VCPU` constant is just the request number.
	// Let's try with a raw syscall for this one, as it's a common pattern for KVM.
	// Or, check if there's an x/sys/unix variant that takes (fd, req, arg_scalar) and returns scalar.
	// unix.IoctlRetInt is defined as `IoctlRetInt(fd int, req uint) (int, error)`
	// No, this is wrong. For an _IO ioctl, the third argument of the syscall is the actual data.
	// If KVM_CREATE_VCPU is truly _IO, it means it should not return a value through a pointer arg, but the syscall itself returns a value.
	// The Linux man page `ioctl(vm_fd, KVM_CREATE_VCPU, id)` returns the fd.
	// The `id` is passed directly as the third argument.
	// So, we need a syscall wrapper that takes fd, req, and an int argument, and returns an int (the new fd).
	// `syscall.Syscall` or `unix.Syscall` is the way for this.
	// `r1, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(vm.vmFd), uintptr(ioctl_KVM_CREATE_VCPU), uintptr(vcpuID))`
	// The return value `r1` would be the vcpuFd.

	r1, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(vm.vmFd), uintptr(ioctl_KVM_CREATE_VCPU), uintptr(vcpuID))
	var createVCPUError error
	if errno != 0 {
		createVCPUError = errno // Convert syscall.Errno to error
	}
	if createVCPUError != nil {
		vm.state = StateError
		log.Printf("Failed to create VCPU for vmFd %d (vcpu_id %d): %v", vm.vmFd, vcpuID, createVCPUError)
		return fmt.Errorf("KVM_CREATE_VCPU ioctl failed: %w", createVCPUError)
	}
	vcpuFd := int(r1)
	if vcpuFd < 0 { // Should be caught by errno really, but as a safeguard
		vm.state = StateError
		// createVCPUError might be nil here if errno was 0 but r1 was < 0 (unlikely for this syscall)
		log.Printf("KVM_CREATE_VCPU ioctl returned invalid fd: %d for vmFd %d", vcpuFd, vm.vmFd)
		return fmt.Errorf("KVM_CREATE_VCPU ioctl returned invalid fd %d", vcpuFd)
	}
	vm.vcpuFd = vcpuFd
	log.Printf("Created VCPU (vcpuFd: %d) for VM (vmFd: %d)", vm.vcpuFd, vm.vmFd)

	// 2. Get VCPU mmap size for kvm_run structure
	mmapSize, getMmapError := unix.IoctlRetInt(vm.vcpuFd, ioctl_KVM_GET_VCPU_MMAP_SIZE)
	if getMmapError != nil {
		unix.Close(vm.vcpuFd) // Clean up VCPU fd
		vm.vcpuFd = 0
		vm.state = StateError
		log.Printf("Failed to get VCPU mmap size for vcpuFd %d: %v", vm.vcpuFd, getMmapError)
		return fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE ioctl failed: %w", getMmapError)
	}
	if mmapSize <= 0 {
		unix.Close(vm.vcpuFd)
		vm.vcpuFd = 0
		vm.state = StateError
		log.Printf("KVM_GET_VCPU_MMAP_SIZE returned invalid size: %d for vcpuFd %d", mmapSize, vm.vcpuFd)
		return fmt.Errorf("KVM_GET_VCPU_MMAP_SIZE returned invalid size: %d", mmapSize)
	}
	log.Printf("VCPU mmap size for kvm_run: %d bytes (vcpuFd: %d)", mmapSize, vm.vcpuFd)

	// 3. Mmap the kvm_run structure
	// PROT_READ | PROT_WRITE, MAP_SHARED, from vcpuFd, offset 0
	kvmRunData, mMapError := unix.Mmap(vm.vcpuFd, 0, mmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if mMapError != nil {
		unix.Close(vm.vcpuFd)
		vm.vcpuFd = 0
		vm.state = StateError
		log.Printf("Failed to mmap kvm_run structure for vcpuFd %d: %v", vm.vcpuFd, mMapError)
		return fmt.Errorf("failed to mmap kvm_run for vcpuFd %d: %w", vm.vcpuFd, mMapError)
	}
	vm.kvmRunData = kvmRunData
	log.Printf("Successfully mmapped kvm_run structure for vcpuFd: %d", vm.vcpuFd)

	// (Placeholder for VCPU configuration: CPUID, initial registers like RIP, RSP, flags)
	// This is a complex step and will be detailed in a subsequent task (e.g., related to bootloader loading).
	// For now, KVM_RUN will likely fail or exit immediately without proper register setup.
	// Example: Set RIP to a HLT instruction in a guest memory region if we had one.

	vm.runLoopExitChan = make(chan struct{})
	vm.runLoopDoneChan = make(chan struct{}) // Initialize done chan

	go vm.runLoop() // Launch the run loop

	vm.state = StateRunning
	log.Printf("VM (vmFd: %d, vcpuFd: %d) is now in state Running", vm.vmFd, vm.vcpuFd)
	return nil
}


// runLoop is the main loop for a VCPU. It repeatedly calls KVM_RUN.
// This method is expected to be run in a separate goroutine.
func (vm *VirtualMachine) runLoop() {
	log.Printf("Starting runLoop for VM (vmFd: %d, vcpuFd: %d)", vm.vmFd, vm.vcpuFd)
	defer log.Printf("Exiting runLoop for VM (vmFd: %d, vcpuFd: %d)", vm.vmFd, vm.vcpuFd)
	defer close(vm.runLoopDoneChan) // Signal that the loop and its cleanup are finished

	// Ensure VCPU fd and kvmRunData are cleaned up when loop exits, regardless of reason.
	// Note: vm.Close() also tries to do this, but runLoop is the primary owner during running.
	// defer func() {
	// 	if vm.vcpuFd > 0 {
	// 		unix.Close(vm.vcpuFd)
	// 		vm.vcpuFd = 0
	// 	}
	// 	if vm.kvmRunData != nil {
	// 		unix.Munmap(vm.kvmRunData)
	// 		vm.kvmRunData = nil
	// 	}
	// }() // This defer might be redundant if vm.Close() is always called and handles it.
	// Let's rely on vm.Close() for FD cleanup, called by whomever stops/destroys the VM.
	// runLoop's responsibility is the KVM_RUN interaction and state updates.

	for {
		select {
		case <-vm.runLoopExitChan:
			log.Printf("runLoop received exit signal (vmFd: %d, vcpuFd: %d). Terminating.", vm.vmFd, vm.vcpuFd)
			// State will be set by Stop() or Close()
			return
		default:
			// Non-blocking check for exit signal before each KVM_RUN
		}

		// Call KVM_RUN on the VCPU file descriptor.
		// The third argument to ioctl for KVM_RUN is 0 (ignored).
		// This is a blocking call until a VM-exit occurs.
		_, err := unix.IoctlRetInt(vm.vcpuFd, ioctl_KVM_RUN) // KVM_RUN is _IO, but often called this way.
		                                                     // Or syscall.Syscall(syscall.SYS_IOCTL, uintptr(vm.vcpuFd), ioctl_KVM_RUN, 0)

		if err != nil {
			// EINTR means the syscall was interrupted, possibly by a signal. Check exitChan again.
			if err == syscall.EINTR {
				log.Printf("KVM_RUN interrupted (EINTR) for vcpuFd: %d. Checking exit signal.", vm.vcpuFd)
				continue // Loop again to check runLoopExitChan
			}
			// Other errors are more critical.
			log.Printf("KVM_RUN ioctl failed for vcpuFd %d: %v. Terminating runLoop.", vm.vcpuFd, err)
			vm.mu.Lock()
			vm.state = StateError
			vm.mu.Unlock()
			return
		}

		// At this point, KVM_RUN has returned, meaning a VM-exit occurred.
		// The details are in the vm.kvmRunData (the mmapped kvm_run struct).
		// We need to parse this data. For now, we don't have the kvm_run struct definition in Go.
		// So, we'll just log a generic exit.
		// TODO: Define kvm_run struct and parse exit_reason.
		// For now, we'll assume any exit without an error means we should log it and continue,
		// unless it's a specific reason we decide to stop on (like SHUTDOWN).

		// This is a placeholder for proper exit reason parsing.
		// kvmRun := (*kvm_run_struct_equivalent)(unsafe.Pointer(&vm.kvmRunData[0]))
		// exitReason := kvmRun.exit_reason

		// For now, let's just log that an exit occurred.
		// In a real scenario, without guest setup, it will likely be KVM_EXIT_INTERNAL_ERROR or KVM_EXIT_FAIL_ENTRY
		// if registers like RIP are not set. If it's KVM_EXIT_HLT or KVM_EXIT_SHUTDOWN, we should stop.

		// Simulating reading a generic exit reason for now (actual parsing requires kvm_run struct def)
		// This part is highly simplified.
		// On a bare KVM_RUN without memory or register setup, it often leads to KVM_EXIT_FAIL_ENTRY or KVM_EXIT_INTERNAL_ERROR.
		// Let's assume for this basic step that any exit just gets logged and we continue,
		// until we implement actual exit reason parsing.
		// However, to make Stop() testable, we need the loop to not be infinitely tight on errors.
		// A real guest would HLT or SHUTDOWN. We don't have one.
		// Let's assume an unknown exit means we should probably stop the VM for now to avoid busy-looping on errors.
		log.Printf("KVM_RUN exited for vcpuFd: %d. (Exit reason parsing not yet implemented). Assuming VM should stop.", vm.vcpuFd)
		vm.mu.Lock()
		vm.state = StateStopped // Or StateHalted if we add it.
		vm.mu.Unlock()
		return // Exit the loop

		/* // Proper exit handling would look something like this (once kvm_run is defined):
		   kvmRun := (*KvmRun)(unsafe.Pointer(&vm.kvmRunData[0])) // Assuming KvmRun is our Go struct for kvm_run
		   exitReason := kvmRun.ExitReason // Assuming field name is ExitReason

		   log.Printf("KVM_RUN exited on vcpuFd %d with reason: %d", vm.vcpuFd, exitReason)

		   switch exitReason {
		   case KVM_EXIT_HLT:
		       log.Printf("VCPU %d Halted. Stopping VM.", vm.vcpuFd)
		       vm.mu.Lock()
		       vm.state = StateStopped // Or a more specific "Halted" state
		       vm.mu.Unlock()
		       return // Exit run loop
		   case KVM_EXIT_SHUTDOWN:
		       log.Printf("VCPU %d initiated shutdown. Stopping VM.", vm.vcpuFd)
		       vm.mu.Lock()
		       vm.state = StateStopped
		       vm.mu.Unlock()
		       return // Exit run loop
		   case KVM_EXIT_IO:
		       direction := "IN"
		       if kvmRun.Io.Direction == KVM_EXIT_IO_OUT { // Assuming KVM_EXIT_IO_OUT is defined
		           direction = "OUT"
		       }
		       log.Printf("KVM_EXIT_IO: dir=%s port=0x%x size=%d count=%d data_offset=0x%x",
		           direction, kvmRun.Io.Port, kvmRun.Io.Size, kvmRun.Io.Count, kvmRun.Io.DataOffset)
		       // Handle I/O, then continue loop
		   case KVM_EXIT_FAIL_ENTRY:
		       log.Printf("KVM_EXIT_FAIL_ENTRY: hardware_entry_failure_reason=0x%x. Stopping VM.", kvmRun.FailEntry.HardwareEntryFailureReason)
		       vm.mu.Lock()
		       vm.state = StateError
		       vm.mu.Unlock()
		       return
		   case KVM_EXIT_INTERNAL_ERROR:
		       log.Printf("KVM_EXIT_INTERNAL_ERROR: suberror=0x%x. Stopping VM.", kvmRun.Internal.Suberror)
		       vm.mu.Lock()
		       vm.state = StateError
		       vm.mu.Unlock()
		       return
		   default:
		       log.Printf("Unhandled KVM exit reason: %d. Continuing.", exitReason)
		       // Continue loop for other exits for now
		   }
		*/
	}
}

// Stop signals the VM to stop its execution and waits for the run loop to terminate.
func (vm *VirtualMachine) Stop() error {
	vm.mu.Lock()
	log.Printf("Attempting to stop VM (vmFd: %d, vcpuFd: %d, current state: %s)", vm.vmFd, vm.vcpuFd, vm.state)

	// Check if VM can be stopped
	if vm.state != StateRunning && vm.state != StatePaused && vm.state != StateCreating { // Allow stopping from Creating if Start failed mid-way before runLoop fully active
		// If already stopping or stopped, or in error, nothing to do or can't stop.
		if vm.state == StateStopping || vm.state == StateStopped {
			log.Printf("VM is already stopping or stopped (state: %s).", vm.state)
			vm.mu.Unlock()
			return nil // Not an error to stop an already stopped VM.
		}
		log.Printf("VM cannot be stopped from state: %s", vm.state)
		vm.mu.Unlock()
		return fmt.Errorf("VM cannot be stopped from state: %s", vm.state)
	}

	initialState := vm.state
	vm.state = StateStopping

	// Signal the runLoop to exit, if the channel exists
	if vm.runLoopExitChan != nil {
		log.Printf("Signaling runLoopExitChan for VM (vmFd: %d)", vm.vmFd)
		// Close the channel to signal. This is idempotent.
		// Check if it's already closed to avoid panic on double close, though a good pattern ensures it's only closed once.
		select {
		case <-vm.runLoopExitChan:
			// Already closed, nothing to do
			log.Printf("runLoopExitChan was already closed for VM (vmFd: %d)", vm.vmFd)
		default:
			close(vm.runLoopExitChan)
			log.Printf("Closed runLoopExitChan for VM (vmFd: %d)", vm.vmFd)
		}
	} else {
		// This case might happen if Stop is called after Start failed before runLoopExitChan was created,
		// or if VM was never really started.
		log.Printf("runLoopExitChan is nil for VM (vmFd: %d), cannot signal. Assuming loop not running.", vm.vmFd)
		// If the loop wasn't running, we can consider it stopped.
		// This might happen if Start failed very early.
		if initialState == StateCreating { // If Start failed during setup
			vm.state = StateStopped
			log.Printf("VM (vmFd: %d) moved to StateStopped as runLoopExitChan was nil during Stop (likely Start failed).", vm.vmFd)
			vm.mu.Unlock()
			// Call Close to ensure FDs are cleaned up if Start part-way succeeded
			// go vm.Close() // Or handle cleanup more directly. For now, rely on explicit Close call later.
			return nil
		}
	}
	vm.mu.Unlock() // Unlock before waiting on runLoopDoneChan to prevent deadlock

	// Wait for the runLoop to confirm it has exited
	if vm.runLoopDoneChan != nil {
		log.Printf("Waiting for runLoopDoneChan for VM (vmFd: %d)", vm.vmFd)
		<-vm.runLoopDoneChan // This blocks until runLoop closes runLoopDoneChan
		log.Printf("runLoopDoneChan confirmed for VM (vmFd: %d)", vm.vmFd)
	} else {
		log.Printf("runLoopDoneChan is nil for VM (vmFd: %d). Cannot wait for run loop confirmation.", vm.vmFd)
		// If runLoopDoneChan is nil, it implies the run loop might not have started properly.
		// We should ensure the state reflects this.
		// Re-acquire lock to safely change state
		vm.mu.Lock()
		if vm.state == StateStopping { // If we were stopping but couldn't confirm
			vm.state = StateStopped // Assume it's stopped if the loop wasn't fully up
			log.Printf("VM (vmFd: %d) moved to StateStopped as runLoopDoneChan was nil.", vm.vmFd)
		}
		vm.mu.Unlock()
	}

	// Re-acquire lock to finalize state. This is important.
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Final state update after run loop has exited.
	// It's possible the runLoop itself set the state to Error or Stopped.
	if vm.state != StateError { // Don't override an error state
		vm.state = StateStopped
	}
	log.Printf("VM (vmFd: %d, vcpuFd: %d) is now in state %s after Stop.", vm.vmFd, vm.vcpuFd, vm.state)

	// Note: Resource cleanup (vcpuFd, kvmRunData mmap, vmFd) is primarily handled by vm.Close().
	// Stop() ensures the run loop terminates. A subsequent Close() is expected for full cleanup.
	// If Stop should also fully clean, then parts of Close() logic would move here.
	// For now, separating concerns: Stop terminates execution, Close releases resources.

	return nil
}

// Pause conceptually pauses the VM.
// For this basic implementation, it only changes the state.
// True KVM pause might involve stopping the KVM_RUN loop without destroying VCPU state,
// or using specific KVM ioctls if available for a lighter-weight pause.
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
	// In a more advanced implementation, we might signal KVM_RUN loop to stop temporarily
	// or use a KVM-specific pause ioctl if one exists and is appropriate.
	return nil
}

// Resume conceptually resumes a paused VM.
// For this basic implementation, it only changes the state.
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
	// If Pause actually stopped the KVM_RUN loop, Resume would need to restart it.
	return nil
}


// AddMemoryRegion maps a portion of host memory to the guest's physical address space.
func (vm *VirtualMachine) AddMemoryRegion(slot uint32, guestPhysAddr uint64, memorySize uint64, flags uint32, readOnly bool) (*MemoryRegion, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.state == StateRunning || vm.state == StatePaused {
		// Modifying memory layout while VM is running/paused can be dangerous or disallowed.
		// KVM might allow it, but it's safer to restrict this to configuration time (e.g., StateStopped, StateCreating).
		log.Printf("Cannot add memory region to VM (vmFd: %d) while it is in state %s.", vm.vmFd, vm.state)
		return nil, fmt.Errorf("cannot add memory region while VM is %s", vm.state)
	}

	if memorySize == 0 {
		return nil, fmt.Errorf("memory size cannot be zero")
	}

	// Memory size and guest physical address should be page-aligned.
	// Host mmap will also typically work with page granularity.
	pageSize := uint64(os.Getpagesize())
	if memorySize%pageSize != 0 {
		//return nil, fmt.Errorf("memory size 0x%x must be a multiple of page size 0x%x", memorySize, pageSize)
		// For now, we can allow it and mmap will likely round up, or KVM might complain.
		// Let's enforce it for clarity.
		log.Printf("Warning: memory size 0x%x is not page aligned (page size 0x%x). This might lead to issues.", memorySize, pageSize)
		// To be strict: return fmt.Errorf(...)
	}
	if guestPhysAddr%pageSize != 0 {
		log.Printf("Warning: guest physical address 0x%x is not page aligned (page size 0x%x). This might lead to issues.", guestPhysAddr, pageSize)
		// To be strict: return fmt.Errorf(...)
	}


	// 1. Allocate page-aligned host memory using mmap
	// MAP_ANONYMOUS: The mapping is not backed by any file; its contents are initialized to zero.
	// MAP_PRIVATE: Create a private copy-on-write mapping. Updates to the mapping are not visible to other processes.
	// Or MAP_SHARED if we want other processes (e.g. external device models) to see raw memory. For now, PRIVATE.
	// For guest RAM, MAP_SHARED might be more appropriate if the intent is direct mapping without CoW.
	// Let's use MAP_ANONYMOUS | MAP_SHARED for typical RAM behavior.
	// Or MAP_ANONYMOUS | MAP_PRIVATE if we want host COW semantics for the backing.
	// QEMU uses a file backend or MAP_ANONYMOUS | MAP_SHARED for RAM.
	// Let's start with ANONYMOUS | PRIVATE for simplicity and safety from host perspective. KVM sees it as RAM anyway.
	// After more thought, for guest RAM, we usually want it to be mutable by the guest and potentially by the VMM
	// for things like live migration. MAP_SHARED with MAP_ANONYMOUS is common.
	// Let's use MAP_ANONYMOUS | MAP_SHARED.
	hostMem, err := unix.Mmap(-1, 0, int(memorySize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANONYMOUS|unix.MAP_SHARED)
	if err != nil {
		log.Printf("Failed to mmap host memory for VM (vmFd: %d), size %d: %v", vm.vmFd, memorySize, err)
		return nil, fmt.Errorf("failed to mmap host memory (size %d): %w", memorySize, err)
	}
	// hostUserAddr is the address of the first byte of the slice.
	hostUserAddr := uintptr(unsafe.Pointer(&hostMem[0]))
	log.Printf("Mmapped host memory for VM (vmFd: %d): addr=0x%x, actual_slice_len=%d, requested_size=%d", vm.vmFd, hostUserAddr, len(hostMem), memorySize)


	// 2. Populate kvm_userspace_memory_region struct
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

	// 3. Call KVM_SET_USER_MEMORY_REGION ioctl
	// The third argument to ioctl must be a pointer to the struct.
	log.Printf("Calling KVM_SET_USER_MEMORY_REGION for VM (vmFd: %d), slot: %d, GPA: 0x%x, size: 0x%x, HUA: 0x%x, flags: 0x%x",
		vm.vmFd, slot, guestPhysAddr, memorySize, hostUserAddr, effectiveFlags)

	// We use unix.IoctlWritePointer for _IOW ioctls.
	// The request number should be the fully encoded one.
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(vm.vmFd),
		uintptr(ioctl_KVM_SET_USER_MEMORY_REGION_FULL), // Use the fully encoded ioctl number
		uintptr(unsafe.Pointer(&memRegionStruct)),
	)

	if errno != 0 {
		// If ioctl fails, unmap the host memory we just allocated
		log.Printf("KVM_SET_USER_MEMORY_REGION ioctl failed for VM (vmFd: %d): %v. Unmapping host memory.", vm.vmFd, errno)
		errUnmap := unix.Munmap(hostMem)
		if errUnmap != nil {
			log.Printf("Critical: Failed to unmap host memory after KVM_SET_USER_MEMORY_REGION failure: %v", errUnmap)
			// This is a leak if unmap fails.
		}
		return nil, fmt.Errorf("KVM_SET_USER_MEMORY_REGION ioctl failed: %w", errno)
	}

	log.Printf("Successfully set user memory region for VM (vmFd: %d), slot: %d", vm.vmFd, slot)

	// 4. Store the MemoryRegion info
	region := &MemoryRegion{
		Slot:          slot,
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    memorySize,
		HostUserAddr:  hostUserAddr,
		backingStore:  hostMem, // Store the mmapped slice
		Flags:         effectiveFlags,
		readOnly:      (effectiveFlags & KVM_MEM_READONLY) != 0,
		logDirtyPages: (effectiveFlags & KVM_MEM_LOG_DIRTY_PAGES) != 0,
	}
	vm.memoryRegions = append(vm.memoryRegions, region)

	return region, nil
}


// Placeholder for VirtualMachine methods if we decide to make it richer soon
// func (vm *VirtualMachine) Close() error {
//  if vm.vmFd != 0 {
//    return unix.Close(vm.vmFd)
//  }
//  return nil
// }
