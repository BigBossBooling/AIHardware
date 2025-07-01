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
	"v-architect/core_engine/network" // For TapDevice
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
	ataDevice  *devices.ATADevice        // Primary ATA Controller
	ne2000     *devices.NE2000Device     // NE2000 Network Interface Card
	tapDevice  *network.TapDevice      // TAP device for NE2000 (owned by VM for cleanup)
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

	// Initialize ATA device with an in-memory disk
	diskSize := uint64(16 * 1024 * 1024) // 16MB disk
	memDisk, err := devices.NewMemoryDiskImage(diskSize, devices.ATA_SECTOR_SIZE)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory disk image: %w", err)
	}
	ataDevice, err := devices.NewATADevice(memDisk, pic, devices.IRQ_PRIMARY_ATA) // Assuming IRQ_PRIMARY_ATA = 14
	if err != nil {
		return nil, fmt.Errorf("failed to create ATA device: %w", err)
	}

	// Initialize TAP device and NE2000 NIC
	tapDev, err := network.NewTapDevice("tap-varch%d") // Kernel will pick a number
	if err != nil {
		// Consider if VM creation should fail or proceed without network
		return nil, fmt.Errorf("failed to create TAP device: %w", err)
	}
	// MAC address for the virtual NIC
	macAddress := "DE:AD:BE:EF:00:01" // Example MAC
	ne2000Dev, err := devices.NewNE2000Device(devices.NE2000_IO_BASE, tapDev, macAddress, pic, devices.IRQ_NE2000)
	if err != nil {
		tapDev.Close() // Clean up TAP device if NE2000 init fails
		return nil, fmt.Errorf("failed to create NE2000 device: %w", err)
	}


	// Create and initialize the VCPU
	vcpu, err := NewVCpu(vmFD, guestMem) // Pass guestMem for VCPU to access it
	if err != nil {
		tapDev.Close() // Clean up TAP device
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
		ataDevice:  ataDevice,
		ne2000:     ne2000Dev,
		tapDevice:  tapDev,
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
	return vm.vcpu.Run(vm.serialPort, vm.pitDevice, vm.rtcDevice, vm.pic, vm.ataDevice, vm.ne2000)
}

// Stop cleans up the virtual machine resources.
func (vm *VirtualMachine) Stop() {
	if vm.vcpu != nil {
		vm.vcpu.Stop()
	}
	if vm.memFD != 0 {
		syscall.Close(vm.memFD) // Consider using unix.Close for consistency if syscall.Close causes issues on some Go versions for memfd.
	}
	if vm.vmFD != 0 {
		syscall.Close(vm.vmFD)
	}
	if vm.tapDevice != nil {
		// log.Printf("Closing TAP device: %s", vm.tapDevice.IfName()) // Requires log import
		fmt.Printf("Closing TAP device: %s\n", vm.tapDevice.IfName())
		if err := vm.tapDevice.Close(); err != nil {
			// log.Printf("Error closing TAP device %s: %v", vm.tapDevice.IfName(), err)
			fmt.Printf("Error closing TAP device %s: %v\n", vm.tapDevice.IfName(), err)
		}
	}
	fmt.Println("VM stopped.")
}
