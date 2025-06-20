package core_engine

import (
	"encoding/binary" // For PIO data handling if not using direct unsafe pointer writes for multi-byte
	"fmt"
	"syscall" // For EINTR, and conceptual syscall.Syscall
	"time"    // For conceptual sleep in HLT
	"unsafe"  // For unsafe.Pointer and unsafe.Offsetof
)

// KVM_RUN_CONCEPTUAL is a placeholder for the actual KVM_RUN ioctl number.
// In a real implementation, this would come from kvm_constants_linux.go or similar.
const KVM_RUN_CONCEPTUAL = 0xAE80


// VCPU struct now uses KvmRun from kvm_x86_arch.go (same package)
type VCPU struct {
	id     int
	vm     *VirtualMachine // Reference to parent VM
	vcpuFd int             // KVM vCPU file descriptor (conceptual)
	kvmRun *KvmRun         // Pointer to the mmap'd KVM run structure
}

// NewVCPU - updated to use KvmRun type correctly
func NewVCPU(vm *VirtualMachine, id int, kvmSystemFd int) (*VCPU, error) {
	fmt.Printf("Conceptual VCPU: NewVCPU for VM ID: %s, vCPU ID: %d\n", vm.ID, id)

	vcpuFd_placeholder := vm.vmFd + id + 101 // Make it more distinct, ensure it's just a placeholder
	fmt.Printf("Conceptual VCPU: KVM_CREATE_VCPU ioctl called for vCPU ID %d. Got vcpuFd: %d (placeholder)\n", id, vcpuFd_placeholder)

	// In a real scenario, mmapSize comes from KVM_GET_VCPU_MMAP_SIZE
	mmapSize := int(unsafe.Sizeof(KvmRun{}))
	fmt.Printf("Conceptual VCPU: KVM_GET_VCPU_MMAP_SIZE conceptually returned %d bytes (using Go KvmRun struct size).\n", mmapSize)

	// Conceptual mmap: Allocate a KvmRun struct and point to it.
	// Real mmap would return a byte slice, then cast to (*KvmRun)(unsafe.Pointer(&slice[0])).
	conceptualKvmRunInstance := &KvmRun{}
	fmt.Printf("Conceptual VCPU: KVM run structure conceptually mmap'd for vCPU %d at %p.\n", id, conceptualKvmRunInstance)

	vcpu := &VCPU{
		id:     id,
		vm:     vm,
		vcpuFd: vcpuFd_placeholder,
		kvmRun: conceptualKvmRunInstance,
	}

	fmt.Printf("Conceptual VCPU: vCPU %d for VM %s created. setupInitialArchState to be called by VM's StartProcess.\n", id, vm.ID)
	return vcpu, nil
}

// setupInitialArchState remains as defined previously (Task 1 of SUB_PLAN_VCPU_BOOT.md)
func (v *VCPU) setupInitialArchState(kernelLoadAddressGPA uint64, bootParamsAddressGPA uint64, pml4BaseGPA uint64) error {
	fmt.Printf("Conceptual VCPU: setupInitialArchState for vCPU ID %d (vcpu_fd %d) of VM %s\n", v.id, v.vcpuFd, v.vm.ID)
	fmt.Printf("Conceptual VCPU: Using KernelEntryGPA=0x%X, BootParamsGPA=0x%X, PML4_GPA=0x%X\n",
		kernelLoadAddressGPA, bootParamsAddressGPA, pml4BaseGPA)
	fmt.Println("Conceptual VCPU: Setting up CPUID...")
	fmt.Println("Conceptual VCPU: KVM_SET_CPUID2 ioctl called with populated CPUID entries.")
	fmt.Println("Conceptual VCPU: Setting up MSRs...")
	fmt.Println("Conceptual VCPU: KVM_SET_MSRS ioctl called with critical MSRs (EFER, STAR, LSTAR, etc.).")
	fmt.Println("Conceptual VCPU: Setting up SREGS (Segments, CR0, CR3, CR4, EFER)...")
	var sregs kvm_sregs
	sregs.CR0 = CR0_PE | CR0_MP | CR0_ET | CR0_NE | CR0_WP | CR0_AM | CR0_PG
	sregs.CR3 = pml4BaseGPA
	sregs.CR4 = CR4_PAE | CR4_MCE | CR4_PGE
	sregs.EFER = EFER_LME | EFER_LMA | EFER_SCE | EFER_NXE
	fmt.Printf("Conceptual VCPU: KVM_SET_SREGS ioctl called. CR0=0x%X, CR3=0x%X, CR4=0x%X, EFER=0x%X\n", sregs.CR0, sregs.CR3, sregs.CR4, sregs.EFER)
	fmt.Println("Conceptual VCPU: Setting up REGS (RIP, RSP, RFLAGS, RSI)...")
	var regs kvm_regs
	regs.RIP = kernelLoadAddressGPA
	regs.RSP = bootParamsAddressGPA - 16
	regs.RFLAGS = 0x2
	regs.RSI = bootParamsAddressGPA
	fmt.Printf("Conceptual VCPU: KVM_SET_REGS ioctl called. RIP=0x%X, RSP=0x%X, RFLAGS=0x%X, RSI=0x%X\n", regs.RIP, regs.RSP, regs.RFLAGS, regs.RSI)
	fmt.Printf("Conceptual VCPU: Initial MSR/CPUID/SREGS/REGS setup complete for vCPU %d.\n", v.id)
	return nil
}

// Run starts the vCPU execution loop.
func (v *VCPU) Run() error {
	fmt.Printf("Conceptual VCPU.Run: vCPU %d (FD conceptual: %d) of VM %s starting KVM_RUN loop.\n", v.id, v.vcpuFd, v.vm.ID)

	if v.kvmRun == nil {
		err := fmt.Errorf("kvmRun structure not initialized for vCPU %d", v.id)
		if v.vm != nil { v.vm.SetError(err); v.vm.SetStatus(FAILED) }
		return err
	}
	if v.vm == nil {
		 return fmt.Errorf("vCPU %d has no parent VM assigned", v.id)
	}

	// Conceptual loop for KVM_RUN, limited iterations for this conceptual implementation
	for iter := 0; iter < 10; iter++ { // Limit iterations for conceptual testing
		fmt.Printf("Conceptual VCPU.Run: vCPU %d KVM_RUN iteration %d...\n", v.id, iter+1)
		// Actual KVM_RUN call:
		// _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(v.vcpuFd), uintptr(KVM_RUN), 0)
		// if errno != 0 && errno != syscall.EINTR {
		//     runErr := fmt.Errorf("KVM_RUN failed for vCPU %d: %w", v.id, errno)
		//     if v.vm != nil { v.vm.SetError(runErr); v.vm.SetStatus(FAILED) }
		//     return runErr
		// }
		// if errno == syscall.EINTR {
		//     fmt.Printf("Conceptual VCPU.Run: KVM_RUN for vCPU %d interrupted, retrying.\n", v.id)
		//     continue
		// }
		// KVM updates v.kvmRun upon exit.

		// Simulate KVM setting exit reason and data for conceptual flow:
		if iter == 0 && v.id == 0 && v.vm.serialPorts != nil && len(v.vm.serialPorts) > 0 {
			v.kvmRun.ExitReason = KVM_EXIT_IO
			v.kvmRun.Io.Direction = KVM_EXIT_IO_OUT
			v.kvmRun.Io.Port = DEFAULT_SERIAL_IO_BASE_COM1 // Ensure this constant is accessible
			v.kvmRun.Io.Size = 1
			v.kvmRun.Io.Count = 1
			// DataOffset should point within the KvmRun.RawDataPayload for this conceptual model
			v.kvmRun.Io.DataOffset = uint64(unsafe.Offsetof(v.kvmRun.RawDataPayload))
			v.kvmRun.RawDataPayload[0] = 'V' // Character 'V'
		} else if iter == 1 && v.id == 0 && v.vm.serialPorts != nil && len(v.vm.serialPorts) > 0 {
			v.kvmRun.ExitReason = KVM_EXIT_IO
			v.kvmRun.Io.Direction = KVM_EXIT_IO_OUT
			v.kvmRun.Io.Port = DEFAULT_SERIAL_IO_BASE_COM1
			v.kvmRun.Io.Size = 1
			v.kvmRun.Io.Count = 1
			v.kvmRun.Io.DataOffset = uint64(unsafe.Offsetof(v.kvmRun.RawDataPayload))
			v.kvmRun.RawDataPayload[0] = 'M' // Character 'M'
		} else if iter == 2 {
			v.kvmRun.ExitReason = KVM_EXIT_HLT
		} else {
			v.kvmRun.ExitReason = KVM_EXIT_SHUTDOWN
		}
		// End simulation block

		fmt.Printf("Conceptual VCPU.Run: vCPU %d KVM_RUN exited with reason: %d\n", v.id, v.kvmRun.ExitReason)

		switch v.kvmRun.ExitReason {
		case KVM_EXIT_IO:
			ioPort := v.kvmRun.Io.Port
			ioSize := int(v.kvmRun.Io.Size)
			ioDirection := v.kvmRun.Io.Direction
			// dataPayloadPtr points to the conceptual data area within KvmRun struct
			dataPayloadPtr := unsafe.Pointer(&v.kvmRun.RawDataPayload[0])

			fmt.Printf("Conceptual VCPU.Run: vCPU %d KVM_EXIT_IO: Port=0x%X, Size=%d, Dir=%d\n",
				v.id, ioPort, ioSize, ioDirection)

			handled := false
			if v.vm != nil && v.vm.serialPorts != nil {
				for _, sp := range v.vm.serialPorts {
					if sp != nil && ioPort >= sp.ioBaseAddr && ioPort < (sp.ioBaseAddr+8) {
						if ioDirection == KVM_EXIT_IO_OUT {
							var writeData uint64 // Data from guest for device
							switch ioSize {
							case 1: writeData = uint64(*(*uint8)(dataPayloadPtr))
							case 2: writeData = uint64(binary.LittleEndian.Uint16((*[2]byte)(dataPayloadPtr)[:]))
							case 4: writeData = uint64(binary.LittleEndian.Uint32((*[4]byte)(dataPayloadPtr)[:]))
							default:
								fmt.Printf("Warning: vCPU %d KVM_EXIT_IO_OUT unhandled size %d for port 0x%X\n", v.id, ioSize, ioPort)
								continue
							}
							if err := sp.HandlePIOWrite(ioPort, uint8(writeData), ioSize); err != nil { // HandlePIOWrite expects uint8 data for now
								fmt.Printf("Error handling PIO write for vCPU %d on port 0x%X: %v\n", v.id, ioPort, err)
							}
						} else { // KVM_EXIT_IO_IN
							readData, err := sp.HandlePIORead(ioPort, ioSize)
							if err != nil {
								fmt.Printf("Error handling PIO read for vCPU %d on port 0x%X: %v\n", v.id, ioPort, err)
							} else {
								// Write readData back to where KVM expects it (dataPayloadPtr)
								switch ioSize {
								case 1: *(*uint8)(dataPayloadPtr) = uint8(readData)
								case 2: binary.LittleEndian.PutUint16((*[2]byte)(dataPayloadPtr)[:], uint16(readData))
								case 4: binary.LittleEndian.PutUint32((*[4]byte)(dataPayloadPtr)[:], uint32(readData))
								default:
									fmt.Printf("Warning: vCPU %d KVM_EXIT_IO_IN unhandled size %d for port 0x%X\n", v.id, ioSize, ioPort)
								}
							}
						}
						handled = true
						break
					}
				}
			}
			if !handled {
				fmt.Printf("Warning: vCPU %d Unhandled KVM_EXIT_IO: Port=0x%X, Size=%d, Dir=%d\n", v.id, ioPort, ioSize, ioDirection)
				if ioDirection == KVM_EXIT_IO_IN {
					 dataWritePtr := unsafe.Pointer(&v.kvmRun.RawDataPayload[0])
					 switch ioSize {
						case 1: *(*uint8)(dataWritePtr) = 0xFF
						case 2: binary.LittleEndian.PutUint16((*[2]byte)(dataWritePtr)[:], 0xFFFF)
						case 4: binary.LittleEndian.PutUint32((*[4]byte)(dataWritePtr)[:], 0xFFFFFFFF)
					 }
				}
			}

		case KVM_EXIT_MMIO:
			fmt.Printf("Conceptual VCPU %d: KVM_EXIT_MMIO (phys_addr: 0x%X, len: %d, is_write: %t) - Not Implemented.\n",
				v.id, v.kvmRun.RawDataPayload[0], v.kvmRun.RawDataPayload[8], v.kvmRun.RawDataPayload[12]) // Example placeholder access, real MMIO struct needed in KvmRun

		case KVM_EXIT_HLT:
			fmt.Printf("Conceptual VCPU.Run: vCPU %d received KVM_EXIT_HLT. Guest is idle. Pausing for 10ms.\n", v.id)
			time.Sleep(10 * time.Millisecond)

		case KVM_EXIT_SHUTDOWN:
			fmt.Printf("Info: vCPU %d received KVM_EXIT_SHUTDOWN. Exiting run loop.\n", v.id)
			if v.vm != nil { v.vm.SetStatus(STOPPING) }
			return nil

		case KVM_EXIT_FAIL_ENTRY:
			// hardware_entry_failure_reason := v.kvmRun.FailEntry.HardwareEntryFailureReason // Conceptual
			hardware_entry_failure_reason := uint64(0) // Placeholder
			errMsg := fmt.Sprintf("vCPU %d KVM_EXIT_FAIL_ENTRY. Reason: 0x%X. VM will be terminated.", v.id, hardware_entry_failure_reason)
			fmt.Println(errMsg)
			if v.vm != nil { v.vm.SetError(fmt.Errorf(errMsg)); v.vm.SetStatus(FAILED) }
			return fmt.Errorf(errMsg)

		case KVM_EXIT_INTERNAL_ERROR:
			suberror := v.kvmRun.Internal.Suberror // Accessing conceptual Internal struct within KvmRun
			errMsg := fmt.Sprintf("vCPU %d KVM_EXIT_INTERNAL_ERROR. Suberror: 0x%X. VM will be terminated.", v.id, suberror)
			fmt.Println(errMsg)
			if v.vm != nil { v.vm.SetError(fmt.Errorf(errMsg)); v.vm.SetStatus(FAILED) }
			return fmt.Errorf(errMsg)

		default:
			errMsg := fmt.Sprintf("vCPU %d unhandled KVM exit reason: %d. Terminating.", v.id, v.kvmRun.ExitReason)
			fmt.Println(errMsg)
			if v.vm != nil { v.vm.SetError(fmt.Errorf(errMsg)); v.vm.SetStatus(FAILED) }
			return fmt.Errorf(errMsg)
		}
	}

	fmt.Printf("Conceptual VCPU.Run: vCPU %d run loop finished conceptual iterations (should have exited via SHUTDOWN, HLT->SHUTDOWN, or ERROR).\n", v.id)
	if v.vm != nil { v.vm.SetStatus(STOPPING) }
	return nil
}

// Close cleans up resources associated with the vCPU.
func (v *VCPU) Close() error {
	fmt.Printf("Conceptual VCPU: vCPU.Close() called for vCPU ID %d (vcpu_fd %d).\n", v.id, v.vcpuFd)
	// Conceptual: munmap v.kvmRun (if it was a direct mmap slice)
	// Or if v.kvmRun points to a part of v.kvmRunData, then v.kvmRunData is munmap'd.
	// For this conceptual model, we just log.
	fmt.Printf("Conceptual VCPU %d: KVM run structure unmapped.\n", v.id)
	// Conceptual: syscall.Close(v.vcpuFd)
	fmt.Printf("Conceptual VCPU %d: vCPU fd %d closed.\n", v.id, v.vcpuFd)
	return nil
}
