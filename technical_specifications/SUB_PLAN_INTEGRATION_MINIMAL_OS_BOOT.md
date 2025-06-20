# Sub-Plan: Implement Full vCPU & Boot Minimal OS (Integration)

**Objective:** To implement the necessary KVM vCPU setup, the vCPU run loop with basic exit handling, load a minimal bootloader/kernel into the VM's memory, and observe initial boot output on the virtual serial port. This sub-plan focuses on integrating previously defined conceptual components.

**Prerequisites (Assumed conceptually complete from previous steps):**
1.  KVM Hypervisor context established (`KVMHypervisor` initialized).
2.  VM created with a valid KVM VM file descriptor (`VirtualMachine` instance, `vmFd`).
3.  Basic vRAM allocated and mapped to the VM (`VirtualMachine.setupMemory()` which calls `setupInitialPaging()` is conceptually done).
4.  Basic Virtual Serial Port device model implemented and conceptually connected (`SerialPortDevice` instance, I/O port range known, `initializeDevices()` and `cleanupDevices()` methods on `VirtualMachine` exist).
5.  `VCPU` struct exists with `vcpuFd` and `kvmRun` pointer; `NewVCPU` creates a vCPU instance; `setupInitialArchState(kernelGPA, bootParamsGPA, pml4GPA)` exists; `Run()` method with basic exit handling (including serial I/O dispatch) exists.
6.  `VirtualMachine.loadBootImage(kernelPath, initrdPath, cmdline, kernelGPA, initrdGPA, bootParamsGPA)` method exists.

**Tasks:**

**1. Review and Refine `VirtualMachine.StartProcess()` Sequence (`vm_manager.go`):**
    *   **Description:** Ensure the `VirtualMachine.StartProcess()` method (previously `Start()`) correctly orchestrates the VM startup sequence, calling all necessary setup methods in the correct order and passing required parameters.
    *   **Sub-Tasks:**
        *   **1.1. Define Load Addresses:** Establish constants or configurable values for `DefaultKernelLoadGPA`, `DefaultInitrdLoadGPA`, `DefaultBootParamsGPA`, and ensure `PageMapLevel4AddressGPA` (from `memory.go`) is accessible.
        *   **1.2. Status Management:** Ensure `vm.Status` is set to `STARTING` at the beginning and `RUNNING` upon successful launch of vCPU goroutines. Implement robust error handling using a `defer` function to set status to `FAILED` and call cleanup methods if any step fails.
        *   **1.3. Call Sequence:**
            1.  `vm.setupMemory()`: This should already internally call `vm.setupInitialPaging()`. Verify this integration.
            2.  `vm.loadBootImage(kernelPath, initrdPath, cmdline, DefaultKernelLoadGPA, DefaultInitrdLoadGPA, DefaultBootParamsGPA)`: Load kernel, initrd (if any), and create `boot_params`.
            3.  `vm.initializeDevices()`: Initialize serial ports and any other configured I/O devices.
            4.  Loop to create vCPUs (`vm.Config.GetVcpuConfig().GetCount()`):
                *   `NewVCPU(vm, vm.vmFd, vcpuID, kvmSystemFd)`: Create each vCPU instance, passing the parent `vm` reference.
                *   `vcpu.setupInitialArchState(DefaultKernelLoadGPA, DefaultBootParamsGPA, PageMapLevel4AddressGPA)`: Set up the architectural state for each vCPU, passing the correct GPAs.
                *   Append created `vcpu` to `vm.vcpus`.
            5.  Loop to launch vCPUs:
                *   `go vcpu.Run()`: Launch each vCPU in its own goroutine. Include basic error logging from the goroutine and a mechanism for the vCPU to signal the VM's overall status (e.g., via `vm.SetError` and `vm.SetStatus`).
    *   **Success Metrics:** `StartProcess` executes all conceptual steps in the defined order. Errors from sub-steps are propagated. VM status transitions correctly.
    *   **Unit Tests:** (Difficult for `StartProcess` as a whole without extensive mocking). Focus on ensuring the sequence of mocked method calls is correct.

**2. Test Artifacts (Conceptual Definition - For Integration Testing):**
    *   **Minimal Linux Kernel (`bzImage_minimal`):**
        *   Pre-compiled x86-64 Linux kernel (`bzImage`).
        *   Config: 64-bit, serial console (CONFIG_SERIAL_8250, CONFIG_SERIAL_8250_CONSOLE), VirtIO (CONFIG_VIRTIO_PCI, CONFIG_VIRTIO_CONSOLE if VirtIO serial is used instead of emulated 8250 for primary console), KVM guest support (CONFIG_KVM_GUEST).
        *   Minimal initramfs embedded, which prints to `ttyS0` (the first 8250 serial port) and then halts or loops. Example output: "V-Architect Minimal Kernel Booted. Halting."
    *   **Kernel Command Line (`cmdline_minimal`):**
        *   Example: `"console=ttyS0 earlyprintk=serial,0x3f8,115200 panic=1"`
        *   `console=ttyS0`: Directs kernel output to the first serial port (mapped to I/O port 0x3F8).
        *   `earlyprintk=serial,0x3f8,115200`: For very early messages.
        *   `panic=1`: Simplifies debugging if the kernel panics.
    *   **No separate initrd needed if using an embedded initramfs.**

**3. Expected Serial Output for Successful Conceptual "First Boot":**
    *   The `SerialPortDevice` (conceptually outputting to V-Architect's host stdout/log via `fmt.Printf` in its `HandlePIOWrite` method) should display early kernel boot messages, such as:
        ```
        [VM Serial - com1]: Decompressing Linux... Parsing ELF... Done.
        [VM Serial - com1]: Booting the kernel.
        [VM Serial - com1]: [    0.000000] Linux version X.Y.Z (...)
        [VM Serial - com1]: [    0.000000] Command line: console=ttyS0 earlyprintk=serial,0x3f8,115200 panic=1
        [VM Serial - com1]: [    0.000000] KVM setup pv clock.
        [VM Serial - com1]: [    0.000000] Serial: 8250/16550 driver, 1 ports, IRQ sharing disabled
        [VM Serial - com1]: [    0.000000] serial8250: ttyS0 at I/O 0x3f8 (irq = 4, base_baud = 115200) is a 16550A
        [VM Serial - com1]: V-Architect Minimal Kernel Booted. Halting.
        ```
    *   The key is observing guest-originated messages via the emulated serial port.

**4. Integration Test (Conceptual - `core_engine/integration_test.go` or similar):**
    *   **Test Function:** `TestBootMinimalLinuxKernel_Conceptual()`
    *   **Steps:**
        1.  Initialize `KVMHypervisor`.
        2.  Initialize `VMManager`.
        3.  Define a `pb.VMConfig` for a minimal VM (1 vCPU, e.g., 64MB RAM, one serial port configured for `0x3F8`).
        4.  Call `manager.RegisterNewVM()` to create and register the VM instance. This will also call `vm.setupMemory()` (which includes `vm.setupInitialPaging()`).
        5.  Call `vm.StartProcess(kvmSystemFd, "path/to/bzImage_minimal", "", "console=ttyS0 ...")`.
            *   `kvmSystemFd` obtained from the hypervisor instance.
            *   Paths to test artifacts would be placeholders or point to dummy files in a testdata directory for a conceptual test.
        6.  **Crucial part:** Monitor the output of `fmt.Printf` from `SerialPortDevice.HandlePIOWrite`. In a real test, this would involve capturing stdout or redirecting the serial device's output to a buffer for assertion.
        7.  Check for expected kernel boot messages within a timeout (e.g., 5-10 seconds for a minimal kernel).
        8.  After timeout or expected message, call `vm.Stop(true)` and `manager.DeleteVM()`.
    *   **Success Metric:** Observation of the expected sequence of conceptual log messages from `setupMemory`, `loadBootImage`, `initializeDevices`, `setupInitialArchState` for vCPUs, vCPU run loops starting, and finally, the expected serial output messages from the guest kernel.

This sub-plan integrates all previously defined conceptual components (memory, paging, bootloader loading, vCPU state, serial I/O) into a coherent startup sequence aimed at achieving the first signs of guest OS life. The "test artifacts" and "expected serial output" define the target for this integration.Step 1 (creating `technical_specifications/SUB_PLAN_INTEGRATION_MINIMAL_OS_BOOT.md`) was successful.

Next, according to the plan:
2.  **Update `core_engine/vm_manager.go`**:
    *   Define the constants for default kernel/initrd/boot_params GPAs.
    *   Update/Refine the `VirtualMachine.StartProcess()` method (previously just `Start()`) to reflect the new sequence:
        *   Status checks and setting to `STARTING`.
        *   `defer` function for error handling and status update.
        *   Call `vm.setupMemory()` (which includes `setupInitialPaging`).
        *   Call `vm.loadBootImage()`.
        *   Call `vm.initializeDevices()`.
        *   Create and initialize vCPUs, calling `vcpu.setupInitialArchState()` for each with the correct GPAs.
        *   Launch vCPU run loops.
        *   Set VM status to `RUNNING`.
    *   Ensure `VirtualMachine` has `SetError(error)` and `SetStatus(VMStatus)` methods (these were conceptually added in User Sub-Issue 1.2 and confirmed in User Sub-Issue 2.3, Task 4). I will re-verify these are present from the last read of the file.

Based on the last read of `vm_manager.go` (from User Sub-Issue 2.3, Task 4), the `SetError` and `SetStatus` methods are indeed present. The main task is to replace the existing `Start` method with the more detailed `StartProcess` method.
