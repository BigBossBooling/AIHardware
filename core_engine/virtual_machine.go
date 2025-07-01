package core_engine

import (
	"fmt"
	// "io" // No longer directly used
	"os" // For os.Stdout, if used directly in device setup
	"unsafe" // Needed if using KVM's mmap-related operations or unsafe casts
	"syscall" // For syscall.SYS_IOCTL and other low-level operations
	"golang.org/x/sys/unix" // For Linux-specific syscalls like MemfdCreate

	// Import hypervisor and devices packages once they are created
	"v-architect/core_engine/hypervisor"
	"v-architect/core_engine/devices"
)

// VirtualMachine represents a single virtual machine instance.
type VirtualMachine struct {
	vmFD       int           // File descriptor for the KVM VM
	vcpu       *VCpu         // The virtual CPU
	memorySize uint64        // Size of the VM's guest memory
	memFD      int           // File descriptor for the shared memory region

	// Devices
	serialPort *devices.SerialPortDevice // Virtual serial port (COM1)
	pitDevice  *devices.PITDevice        // Programmable Interval Timer
	rtcDevice  *devices.RTCDevice        // Real-Time Clock
	pic        *devices.PICController    // Programmable Interrupt Controller
	// Add other devices here as they are implemented
}

// CreateVM initializes a new KVM virtual machine and loads the provided bootloader.
func CreateVM(memorySize uint64, bootloader []byte) (*VirtualMachine, error) {
	// Initialize KVM system-wide
	kvmFD, err := syscall.Open("/dev/kvm", syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/kvm: %w", err)
	}

	// Create a new VM instance
	vmFD, err := hypervisor.KvmIoctlCreateVM(kvmFD)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVM VM: %w", err)
	}

	// Allocate guest memory (simplified)
	// Use unix.MFD_CLOEXEC for flags, common practice with memfd_create
	memFD, err := unix.MemfdCreate("guest_mem", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("failed to create memfd for guest memory: %w", err)
	}
	if err := unix.Ftruncate(int(memFD), int64(memorySize)); err != nil { // unix.Ftruncate expects int fd
		return nil, fmt.Errorf("failed to ftruncate memfd: %w", err)
	}
	// Use unix.Mmap for consistency, though syscall.Mmap might also work
	guestMem, err := unix.Mmap(int(memFD), 0, int(memorySize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap guest memory: %w", err)
	}

	// Set up guest memory region in KVM
	guestMemAddr := uintptr(unsafe.Pointer(&guestMem[0]))
	if err := hypervisor.KvmIoctlSetUserMemoryRegion(vmFD, 0, 0, memorySize, guestMemAddr); err != nil {
		return nil, fmt.Errorf("failed to set user memory region: %w", err)
	}

	// Load bootloader into guest memory at 0x7c00
	const bootloaderLoadAddress = 0x7C00
	if bootloader != nil && len(bootloader) > 0 {
		if len(bootloader) > 512 { // Basic check, MBR is typically 512 bytes
			// For now, log a warning. Could be an error in a stricter implementation.
			fmt.Printf("Warning: Bootloader size (%d bytes) exceeds 512 bytes.\n", len(bootloader))
		}
		if bootloaderLoadAddress+uint64(len(bootloader)) > memorySize {
			return nil, fmt.Errorf("bootloader (size %d) exceeds guest memory (size %d) when loaded at 0x%X",
				len(bootloader), memorySize, bootloaderLoadAddress)
		}
		copy(guestMem[bootloaderLoadAddress:], bootloader)
		fmt.Printf("Bootloader (size %d bytes) copied to guest memory at 0x%X\n", len(bootloader), bootloaderLoadAddress)
	} else {
		fmt.Println("No bootloader provided or bootloader is empty.")
		// Depending on requirements, this could be an error or the VM could proceed (e.g., to PXE boot or halt)
		// For now, we'll allow it and assume the guest might have its own way to start or will simply fail.
	}

	// Initialize devices
	pic := devices.NewPICController()
	// For SerialPortDevice, you used os.Stdout as the output writer
	// Now, devices will need a way to raise interrupts via the PIC.
	// The InterruptRaiser interface is implemented by PICController.
	serialPort := devices.NewSerialPortDevice(os.Stdout, pic) // Pass PIC as InterruptRaiser
	pitDevice := devices.NewPITDevice(pic)                     // Pass PIC as InterruptRaiser
	rtcDevice := devices.NewRTCDevice(pic)                     // Pass PIC as InterruptRaiser

	// Create and initialize the VCPU
	vcpu, err := NewVCpu(vmFD, guestMem) // Pass guestMem for VCPU to access it
	if err != nil {
		return nil, fmt.Errorf("failed to create VCPU: %w", err)
	}

	vm := &VirtualMachine{
		vmFD:       vmFD,
		vcpu:       vcpu,
		memorySize: memorySize,
		memFD:      memFD,
		serialPort: serialPort,
		pitDevice:  pitDevice,
		rtcDevice:  rtcDevice,
		pic:        pic,
	}

	// Configure VCPU registers (simplified for example)
	// You'll need more detailed setup here based on your boot sequence
	if err := vm.vcpu.SetupRegisters(); err != nil {
		return nil, fmt.Errorf("failed to setup VCPU registers: %w", err)
	}

	return vm, nil
}

// Run starts the execution of the virtual machine.
func (vm *VirtualMachine) Run() error {
	fmt.Println("VM starting...")
	// Pass all relevant devices to the VCPU's run loop.
	// The VCPU will need to query the PIC for pending interrupts.
	return vm.vcpu.Run(vm.serialPort, vm.pitDevice, vm.rtcDevice, vm.pic)
}

// Stop cleans up the virtual machine resources.
func (vm *VirtualMachine) Stop() {
	if vm.vcpu != nil {
		vm.vcpu.Stop()
	}
	if vm.memFD != 0 {
		syscall.Close(vm.memFD)
	}
	if vm.vmFD != 0 {
		syscall.Close(vm.vmFD)
	}
	fmt.Println("VM stopped.")
}
