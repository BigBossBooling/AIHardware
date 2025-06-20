# Sub-Plan: Implement Full vCPU & Boot Minimal OS

**Objective:** To implement the necessary KVM vCPU setup, the vCPU run loop with basic exit handling, load a minimal bootloader/kernel into the VM's memory, and observe initial boot output on the virtual serial port.

**Prerequisites (Assumed conceptually complete from previous steps):**
1.  KVM Hypervisor context established (`KVMHypervisor` initialized).
2.  VM created with a valid KVM VM file descriptor (`VirtualMachine` instance, `vmFd`).
3.  Basic vRAM allocated and mapped to the VM (`VirtualMachine.setupMemory()` conceptually done).
4.  Basic Virtual Serial Port device model implemented and conceptually connected (`SerialPortDevice` instance, I/O port range known).
5.  `VCPU` struct exists with `vcpuFd` and `kvmRun` pointer (from `NewVCPU` initial stub).

**Tasks:**

**1. Implement Full vCPU Architectural State Setup (x86-64 Focus):**
    *   **Description:** Enhance `VCPU.setupInitialArchState()` in `core_engine/vcpu.go` to perform actual KVM ioctl calls for setting up critical CPU state for booting a 64-bit Linux kernel.
    *   **Sub-Tasks:**
        *   **1.1. CPUID Setup (`KVM_SET_CPUID2`):**
            *   Define a comprehensive set of CPUID entries. Start by getting host CPUID (e.g., using `libcpuid` or direct `CPUID` instruction in Cgo, then translating to `kvm_cpuid_entry2` format).
            *   Modify entries: Ensure essential features for a 64-bit kernel are present (Long Mode, PAE, etc.). Add KVM hypervisor signature.
            *   Call `ioctl(vcpuFd, KVM_SET_CPUID2, &cpuid_struct)`.
        *   **1.2. MSR Setup (`KVM_SET_MSRS`):**
            *   Identify critical MSRs: `EFER` (Long Mode Active LMA, NXE, SCE), `STAR`, `LSTAR`, `CSTAR`, `SFMASK`, `KERNEL_GS_BASE`, `MSR_KVM_SYSTEM_TIME` (for PV clock).
            *   Populate `kvm_msrs` struct and call `ioctl(vcpuFd, KVM_SET_MSRS, &msrs_struct)`.
        *   **1.3. Special Registers Setup (SREGS - `KVM_SET_SREGS`):**
            *   Fetch current SREGS: `ioctl(vcpuFd, KVM_GET_SREGS, &sregs_struct)`.
            *   **Segment Registers (CS, DS, ES, SS, etc.):** Configure flat 64-bit segments. CS must point to a 64-bit code segment.
            *   **Control Registers (CR0, CR3, CR4):**
                *   `CR0`: Set PE (Protected Mode), PG (Paging), WP (Write Protect). Other bits like AM, NE.
                *   `CR3`: Set to Guest Physical Address (GPA) of the PML4 page table (which needs to be created in Task 2).
                *   `CR4`: Set PAE (Physical Address Extension), PGE (Page Global Enable), OSXSAVE (if XMM/AVX features are enabled).
            *   **GDT & IDT:** Setup minimal Global Descriptor Table (GDT) and Interrupt Descriptor Table (IDT) in guest memory and load their base/limit into GDTR/IDTR within `sregs_struct`. For initial boot, a null IDT might suffice if interrupts are masked.
            *   Call `ioctl(vcpuFd, KVM_SET_SREGS, &sregs_struct)`.
        *   **1.4. General Purpose Registers Setup (REGS - `KVM_SET_REGS`):**
            *   `RIP`: Set to the entry point of the loaded bootloader/kernel (from Task 3).
            *   `RSP`: Set to an appropriate initial stack pointer within guest RAM.
            *   `RFLAGS`: Set initial flags (e.g., Interrupt Flag IF usually cleared initially).
            *   `RSI` (for Linux x86-64 boot protocol): Set to the GPA of the boot parameters (zero page).
            *   Call `ioctl(vcpuFd, KVM_SET_REGS, &regs_struct)`.
    *   **Success Metrics:** All ioctl calls return success. Registers can be read back (via `KVM_GET_SREGS`, `KVM_GET_REGS`) and match set values.
    *   **Unit Tests:** For each sub-task, mock KVM ioctls and verify the correct data structures are prepared and passed to the (mocked) ioctl.

**2. Implement Initial Guest Page Tables (x86-64 Long Mode):**
    *   **Description:** In `VirtualMachine.setupMemory()` or a dedicated `vm.setupInitialPaging()` method, create the initial page table hierarchy (PML4, PDPT, Page Directory, Page Table) in the `vm.guestMem` byte slice.
    *   **Sub-Tasks:**
        *   **2.1. Allocate Pages:** Reserve specific GPAs within `vm.guestMem` for PML4, PDPT, and at least one Page Directory and Page Table.
        *   **2.2. Populate Entries:**
            *   PML4 entry pointing to PDPT.
            *   PDPT entry pointing to Page Directory.
            *   Page Directory entry pointing to Page Table.
            *   Page Table entries identity mapping (GPA -> HPA, but KVM handles HPA, so GPA -> GPA for guest view) the first few MB of RAM (e.g., 0-2MB or 0-4MB) to allow kernel decompression and early boot code to run. Ensure Present (P) and Read/Write (RW) bits are set.
        *   **2.3. Kernel/Bootloader Region Mapping:** Ensure the GPA range where the kernel/bootloader will be loaded (Task 3) is also mapped by these initial page tables.
    *   **Success Metrics:** Page table structures in `vm.guestMem` are correctly formatted according to x86-64 long mode paging. `CR3` in vCPU SREGS points to the GPA of PML4.
    *   **Unit Tests:** Functions to create/populate page table entries can be unit tested. Verification of the full structure in `vm.guestMem`.

**3. Load Minimal Bootloader/Kernel into Guest Memory:**
    *   **Description:** Implement a function (e.g., `VirtualMachine.loadBootImage(imagePath string, loadAddressGPA uint64)`) to read a small bootloader (e.g., `pvpanic.bin` for testing, or a minimal GRUB stage) or a tiny Linux kernel (`bzImage`) from a host file directly into `vm.guestMem` at a specified GPA.
    *   **Sub-Tasks:**
        *   **3.1. Read Image File:** Open and read the binary file from the host filesystem.
        *   **3.2. Copy to Guest Memory:** Copy the file contents into `vm.guestMem` at `loadAddressGPA`. Ensure `loadAddressGPA` and image size are within mapped guest RAM.
        *   **3.3. (Linux specific) Zero Page / Boot Parameters:** If booting a Linux kernel directly, create the `boot_params` struct (zero page) in guest memory at a known GPA (e.g., 0x10000) and set `RSI` to this address (as per Task 1.4). This struct includes command line, memory map, etc.
    *   **Success Metrics:** File is correctly copied into `vm.guestMem`. `RIP` (Task 1.4) is set to `loadAddressGPA`.
    *   **Unit Tests:** Test file reading and memory copy logic. Test `boot_params` struct creation.

**4. Implement vCPU Run Loop with Basic I/O Exit Handling for Serial Port:**
    *   **Description:** Enhance `VCPU.Run()` in `core_engine/vcpu.go` to correctly execute `KVM_RUN` and handle `KVM_EXIT_IO` for the serial port.
    *   **Sub-Tasks:**
        *   **4.1. `KVM_RUN` Loop:** Implement the blocking `ioctl(vcpuFd, KVM_RUN, 0)` call.
        *   **4.2. Exit Reason Parsing:** After `KVM_RUN` returns, parse `kvm_run->exit_reason`.
        *   **4.3. `KVM_EXIT_IO` Handler:**
            *   Extract port, size, direction, and data offset from `kvm_run->io`.
            *   Iterate through the VM's registered I/O devices (initially just the serial port(s) from `vm.serialPorts`).
            *   If port matches a serial port's range:
                *   If `direction == KVM_EXIT_IO_OUT`: Call `serialDevice.HandlePIOWrite()`.
                *   If `direction == KVM_EXIT_IO_IN`: Call `serialDevice.HandlePIORead()` and write the result back to the `kvm_run->io` data area.
            *   If no device handles the port, log an "unhandled PIO" warning.
        *   **4.4. `KVM_EXIT_HLT` Handler:** Print a message. For now, the loop can continue (guest will re-execute HLT unless an interrupt arrives). In a more advanced system, this might pause the vCPU thread.
        *   **4.5. `KVM_EXIT_SHUTDOWN` / `KVM_EXIT_INTERNAL_ERROR`:** Log and terminate the run loop for this vCPU, potentially signaling the VM for shutdown.
    *   **Success Metrics:** `KVM_RUN` executes. Serial port I/O writes from a guest (e.g., early printk from a tiny kernel) appear on host stdout/PTY. VM can be shut down by guest action if `KVM_EXIT_SHUTDOWN` is handled.
    *   **Integration Test:** The ultimate goal: load a tiny kernel, set RIP to its entry, set up serial port, run vCPU, and see kernel boot messages on the host console/PTY.

This sub-plan provides a more detailed roadmap for the next critical implementation steps.
