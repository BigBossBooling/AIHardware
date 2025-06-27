package core_engine

import (
	"fmt"
	"io"
	"os" // For os.Stdout, if used directly in device setup
	"unsafe" // Needed if using KVM's mmap-related operations or unsafe casts
	"syscall" // For syscall.SYS_IOCTL and other low-level operations

	// Import hypervisor and devices packages once they are created
	"core_engine/hypervisor"
	"core_engine/devices"
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
	// Add other devices here as they are implemented (e.g., PIC)
}

// CreateVM initializes a new KVM virtual machine.
func CreateVM(memorySize uint64) (*VirtualMachine, error) {
	// Initialize KVM system-wide
	kvmFD, err := syscall.Open("/dev/kvm", syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/kvm: %w", err)
	}

	// Create a new VM instance
	vmFD, err := hypervisor.KVM_CREATE_VM(kvmFD)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVM VM: %w", err)
	}

	// Allocate guest memory (simplified)
	memFD, err := syscall.MemfdCreate("guest_mem", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create memfd for guest memory: %w", err)
	}
	if err := syscall.Ftruncate(memFD, int64(memorySize)); err != nil {
		return nil, fmt.Errorf("failed to ftruncate memfd: %w", err)
	}
	guestMem, err := syscall.Mmap(memFD, 0, int(memorySize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap guest memory: %w", err)
	}

	// Set up guest memory region in KVM
	guestMemAddr := uintptr(unsafe.Pointer(&guestMem[0]))
	if err := hypervisor.KVM_SET_USER_MEMORY_REGION(vmFD, 0, 0, memorySize, guestMemAddr); err != nil {
		return nil, fmt.Errorf("failed to set user memory region: %w", err)
	}

	// Initialize devices
	// For SerialPortDevice, you used os.Stdout as the output writer
	serialPort := devices.NewSerialPortDevice(os.Stdout) // Assuming NewSerialPortDevice takes an io.Writer
	pitDevice := devices.NewPITDevice()
	rtcDevice := devices.NewRTCDevice()

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
	return vm.vcpu.Run(vm.serialPort, vm.pitDevice, vm.rtcDevice) // Pass devices for handling I/O exits
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
