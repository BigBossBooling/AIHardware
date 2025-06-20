package core_engine

import (
	"encoding/binary" // For conceptual data packing in KVM_EXIT_IO
	"fmt"
	"syscall" // For EINTR, conceptually for actual ioctl
	"time"    // For conceptual sleep in HLT
	"unsafe"  // For accessing kvm_run fields and pointer casting
)

// KVM ioctl constants (from previous conceptual steps)
const (
	KVM_SET_CPUID2_CONCEPTUAL = 0xAE90
	KVM_SET_MSRS_CONCEPTUAL   = 0xAE89
	KVM_GET_SREGS_CONCEPTUAL  = 0xAE83
	KVM_SET_SREGS_CONCEPTUAL  = 0xAE84
	KVM_SET_REGS_CONCEPTUAL   = 0xAE82
	KVM_GET_REGS_CONCEPTUAL   = 0xAE81
	KVM_RUN_CONCEPTUAL        = 0xAE80
)

// Conceptual KVM structures (from previous conceptual steps)
type kvm_cpuid_entry2 struct {
	Function uint32; Index uint32; Flags uint32; Eax uint32; Ebx uint32; Ecx uint32; Edx uint32;
}
type kvm_cpuid2_header struct { Nent, Padding uint32; }
type kvm_msr_entry struct { Index, Reserved uint32; Data uint64; }
type kvm_msrs_header struct { Nmsrs, Padding uint32; }
type kvm_segment struct { Base uint64; Limit uint32; Selector uint16; Type, Present, DPL, DB, S, L, G, Unusable, Padding uint8; }
type kvm_dtable struct { Base uint64; Limit uint16; Padding [3]uint16; }
type kvm_sregs struct {
	CS, DS, ES, FS, GS, SS kvm_segment; TR, LDT kvm_segment; GDT, IDT kvm_dtable;
	CR0, CR2, CR3, CR4, CR8, EFER, ApicBase uint64;
}
type kvm_regs struct {
	RAX, RBX, RCX, RDX, RSI, RDI, RSP, RBP uint64; R8, R9, R10, R11, R12, R13, R14, R15 uint64;
	RIP, RFLAGS uint64;
}

// VCPU struct
type VCPU struct {
	id      int
	vm      *VirtualMachine // Reference to parent VM
	vcpuFd  int             // KVM vCPU file descriptor (conceptual)
	kvmRun  *KvmRun         // Pointer to the mmap'd KVM run structure (from kvm_structs.go)
}

// NewVCPU (modified to use KvmRun from kvm_structs.go and not call setupInitialArchState)
func NewVCPU(vm *VirtualMachine, id int, kvmSystemFd int) (*VCPU, error) {
	fmt.Printf("Conceptual VCPU: NewVCPU called for VM ID: %s, vCPU ID: %d, KVM System FD: %d\n", vm.ID, id, kvmSystemFd)
	vcpu_fd_placeholder := 2000 + id
	fmt.Printf("Conceptual VCPU: KVM_CREATE_VCPU ioctl called for VM ID %s, vCPU ID %d. Got vcpu_fd: %d (placeholder)\n", vm.ID, id, vcpu_fd_placeholder)

	mmap_size := int(unsafe.Sizeof(KvmRun{})) // Use size of KvmRun from kvm_structs.go
	fmt.Printf("Conceptual VCPU: KVM_GET_VCPU_MMAP_SIZE conceptually returned %d bytes\n", mmap_size)

	kvmRunStruct_placeholder := KvmRun{}
	kvmRunPtr_placeholder := &kvmRunStruct_placeholder
	fmt.Printf("Conceptual VCPU: Mapped KVM_RUN structure for vCPU %d (using placeholder Go struct address: %p)\n", id, kvmRunPtr_placeholder)

	vcpu := &VCPU{
		id:      id,
		vm:      vm,
		vmFd:    vm.vmFd, // From parent VM
		vcpuFd:  vcpu_fd_placeholder,
		kvmRun:  kvmRunPtr_placeholder,
	}

	fmt.Printf("Conceptual VCPU: vCPU %d for VM %s created. setupInitialArchState will be called by VM's StartProcess.\n", id, vm.ID)
	return vcpu, nil
}

// setupInitialArchState (as defined in previous subtask - Task 1 of SUB_PLAN_VCPU_BOOT.md)
func (v *VCPU) setupInitialArchState(kernelLoadAddressGPA uint64, bootParamsAddressGPA uint64, pml4AddressGPA uint64) error {
	fmt.Printf("Conceptual VCPU: setupInitialArchState for vCPU ID %d (vcpu_fd %d) of VM %s\n", v.id, v.vcpuFd, v.vm.ID)
	fmt.Printf("Conceptual VCPU: Using KernelEntryGPA=0x%X, BootParamsGPA=0x%X, PML4_GPA=0x%X\n",
		kernelLoadAddressGPA, bootParamsAddressGPA, pml4AddressGPA)
	fmt.Println("Conceptual VCPU: Setting up CPUID...")
	fmt.Println("Conceptual VCPU: KVM_SET_CPUID2 ioctl called with populated CPUID entries.")
	fmt.Println("Conceptual VCPU: Setting up MSRs...")
	fmt.Println("Conceptual VCPU: KVM_SET_MSRS ioctl called with critical MSRs (EFER, STAR, LSTAR, etc.).")
	fmt.Println("Conceptual VCPU: Setting up SREGS (Segments, CR0, CR3, CR4, EFER)...")
	var sregs kvm_sregs
	sregs.CR0 = 0x80050031
	sregs.CR3 = pml4AddressGPA
	sregs.CR4 = 0x000006A0
	sregs.EFER = 0x00000D01
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

// Run starts the vCPU execution loop. (Enhanced as per current subtask description)
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
	for iter := 0; iter < 10; iter++ {
		fmt.Printf("Conceptual VCPU.Run: vCPU %d KVM_RUN iteration %d...\n", v.id, iter+1)
		// Actual KVM_RUN call:
		// _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(v.vcpuFd), KVM_RUN_CONCEPTUAL, 0)
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
			v.kvmRun.Io.Port = DEFAULT_SERIAL_IO_BASE_COM1
			v.kvmRun.Io.Size = 1
			v.kvmRun.Io.Count = 1
			v.kvmRun.Io.DataOffset = uint64(unsafe.Offsetof(v.kvmRun.RawDataPayload))
			v.kvmRun.RawDataPayload[0] = 'V'
		} else if iter == 1 && v.id == 0 && v.vm.serialPorts != nil && len(v.vm.serialPorts) > 0 {
			v.kvmRun.ExitReason = KVM_EXIT_IO
			v.kvmRun.Io.Direction = KVM_EXIT_IO_OUT
			v.kvmRun.Io.Port = DEFAULT_SERIAL_IO_BASE_COM1
			v.kvmRun.Io.Size = 1
			v.kvmRun.Io.Count = 1
			v.kvmRun.Io.DataOffset = uint64(unsafe.Offsetof(v.kvmRun.RawDataPayload))
			v.kvmRun.RawDataPayload[0] = 'M'
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
			dataPayloadPtr := unsafe.Pointer(&v.kvmRun.RawDataPayload[0])

			fmt.Printf("Conceptual VCPU.Run: vCPU %d KVM_EXIT_IO: Port=0x%X, Size=%d, Dir=%d\n",
				v.id, ioPort, ioSize, ioDirection)

			handled := false
			if v.vm != nil && v.vm.serialPorts != nil {
				for _, sp := range v.vm.serialPorts {
					if sp != nil && ioPort >= sp.ioBaseAddr && ioPort < (sp.ioBaseAddr+8) {
						if ioDirection == KVM_EXIT_IO_OUT {
							var writeData uint64
							switch ioSize {
							case 1: writeData = uint64(*(*uint8)(dataPayloadPtr))
							case 2: writeData = uint64(binary.LittleEndian.Uint16((*[2]byte)(dataPayloadPtr)[:]))
							case 4: writeData = uint64(binary.LittleEndian.Uint32((*[4]byte)(dataPayloadPtr)[:]))
							default:
								fmt.Printf("Warning: vCPU %d KVM_EXIT_IO_OUT unhandled size %d for port 0x%X\n", v.id, ioSize, ioPort)
								continue
							}
							if err := sp.HandlePIOWrite(ioPort, writeData, ioSize); err != nil {
								fmt.Printf("Error handling PIO write for vCPU %d on port 0x%X: %v\n", v.id, ioPort, err)
							}
						} else { // KVM_EXIT_IO_IN
							readData, err := sp.HandlePIORead(ioPort, ioSize)
							if err != nil {
								fmt.Printf("Error handling PIO read for vCPU %d on port 0x%X: %v\n", v.id, ioPort, err)
							} else {
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

		case KVM_EXIT_HLT:
			fmt.Printf("Conceptual VCPU.Run: vCPU %d received KVM_EXIT_HLT. Guest is idle. Pausing for 10ms.\n", v.id)
			time.Sleep(10 * time.Millisecond)

		case KVM_EXIT_SHUTDOWN:
			fmt.Printf("Info: vCPU %d received KVM_EXIT_SHUTDOWN. Exiting run loop.\n", v.id)
			if v.vm != nil { v.vm.SetStatus(STOPPING) }
			return nil

		case KVM_EXIT_INTERNAL_ERROR:
			suberror := uint32(0) // Conceptual: v.kvmRun.InternalError.Suberror
			errMsg := fmt.Sprintf("vCPU %d KVM_EXIT_INTERNAL_ERROR. Suberror: 0x%X. VM will be terminated.", v.id, suberror)
			fmt.Println(errMsg)
			if v.vm != nil {
				v.vm.SetError(fmt.Errorf(errMsg))
				v.vm.SetStatus(FAILED)
			}
			return fmt.Errorf(errMsg)

		default:
			errMsg := fmt.Sprintf("vCPU %d unhandled KVM exit reason: %d. Terminating.", v.id, v.kvmRun.ExitReason)
			fmt.Println(errMsg)
			if v.vm != nil {
				v.vm.SetError(fmt.Errorf(errMsg))
				v.vm.SetStatus(FAILED)
			}
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
	fmt.Printf("Conceptual VCPU %d: KVM run structure unmapped.\n", v.id)
	fmt.Printf("Conceptual VCPU %d: vCPU fd %d closed.\n", v.id, v.vcpuFd)
	return nil
}
