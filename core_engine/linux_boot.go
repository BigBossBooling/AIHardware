package core_engine

import (
	"encoding/binary" // For writing boot_params fields conceptually
	"fmt"
	"os" // For os.ReadFile (conceptually)
	// "io/ioutil" // For ReadFile in older Go versions
)

// Constants for boot_params structure offsets (simplified, from <asm/bootparam.h> or kernel docs)
// These are byte offsets within the boot_params struct, which itself is at the start of the zero page.
// For a more complete implementation, a full Go struct mirroring C's boot_params would be used.
const (
	// Offsets within struct boot_params (which starts at bootParamsGPA)
	// These are illustrative and simplified. Real offsets are from kernel headers.
	// See linux/arch/x86/include/uapi/asm/bootparam.h

	// From 'struct setup_header' (itself at an offset within boot_params, typically 0x1f1 + setup_sects * 512)
	// For simplicity, these are treated as direct offsets from bootParamsGPA for this conceptual model,
	// assuming a very basic boot_params layout or that these are well-known fields.
	// A VMM would typically parse the bzImage header to find the real setup_header first.

	OFFSET_SETUP_HEADER_START = 0x01F1 // Approximate start of setup_header relative to boot_params start
	// Fields relative to setup_header start:
	OFFSET_HDR_TYPE_OF_LOADER  = 0x0E // (setup_header.type_of_loader, offset 0x21E from boot_params start in some layouts)
	OFFSET_HDR_LOADER_FLAGS    = 0x0F // (setup_header.loadflags, offset 0x21F)
	OFFSET_HDR_CMD_LINE_PTR    = 0x18 // (setup_header.cmd_line_ptr, offset 0x228 from boot_params start)
	OFFSET_HDR_INITRD_ADDR_MAX = 0x2C // (setup_header.initrd_addr_max, offset 0x23C)
	OFFSET_HDR_KERNEL_ALIGNMENT= 0x30 // (setup_header.kernel_alignment, offset 0x240)
	OFFSET_HDR_RAMDISK_IMAGE_64= 0x28 // (setup_header.ext_ramdisk_image, offset 0x28 within setup_header if version >= 0x0206)
	OFFSET_HDR_RAMDISK_SIZE_64 = 0x30 // (setup_header.ext_ramdisk_size, offset 0x30 within setup_header if version >= 0x0206)
	// This simplified model will use direct conceptual offsets for key fields for now.
	// A more accurate model would use the proper setup_header parsing.

	// Direct conceptual offsets from bootParamsGPA for this simplified model:
	OFFSET_BP_SETUP_SECTS      = 0x01F1 // (boot_params.hdr.setup_sects)
	OFFSET_BP_TYPE_OF_LOADER   = 0x0210 // (boot_params.hdr.type_of_loader in some layouts)
	OFFSET_BP_LOADFLAGS        = 0x0211 // (boot_params.hdr.loadflags in some layouts)
	OFFSET_BP_CMD_LINE_PTR     = 0x0228 // (boot_params.hdr.cmd_line_ptr)
	OFFSET_BP_RAMDISK_IMAGE    = 0x0218 // (boot_params.hdr.ramdisk_image - 32-bit GPA)
	OFFSET_BP_RAMDISK_SIZE     = 0x021C // (boot_params.hdr.ramdisk_size - 32-bit size)
	OFFSET_BP_E820_MAP_ENTRIES = 0x01E8 // (boot_params.e820_entries - u8)
	OFFSET_BP_E820_MAP         = 0x02D0 // (boot_params.e820_table - array of struct e820entry)

	BOOT_PARAMS_ZERO_PAGE_SIZE = 0x1000 // Typically a 4KB page for zero page + boot_params
	E820_ENTRY_SIZE            = 20     // Size of one struct e820entry
	E820_MAX_ENTRIES           = 128    // Max entries in e820 map
	E820_TYPE_RAM              = 1      // Usable RAM
	E820_TYPE_RESERVED         = 2      // Reserved memory
)

// loadBootImage loads a kernel, optional initrd, and sets up boot_params.
// This method is part of the VirtualMachine struct (defined in vm_manager.go).
// kernelLoadAddressGPA: GPA where the kernel image itself is loaded.
// initrdLoadAddressGPA: GPA where initrd is loaded (if initrdPath is not empty).
// bootParamsGPA: GPA where the boot_params (zero page) struct will be placed.
func (vm *VirtualMachine) loadBootImage(kernelImagePath string, initrdPath string, cmdline string,
	kernelLoadAddressGPA uint64, initrdLoadAddressGPA uint64, bootParamsGPA uint64) (kernelEntryPointActualGPA uint64, bootParamsActualGPA uint64, err error) {

	// resourceLock is assumed to be held by the caller (e.g., vm.StartProcess)
	// if direct vm.guestMem manipulation happens. If this function mmaps, it should lock.
	// For conceptual, assume caller handles lock for vm.guestMem writes.

	if vm.guestMem == nil {
		return 0, 0, fmt.Errorf("guestMem not allocated for VM %s, cannot load boot image", vm.ID)
	}
	fmt.Printf("Conceptual Bootloader: VM %s - Loading boot image. Kernel: '%s', Initrd: '%s', Cmdline: '%s'\n",
		vm.ID, kernelImagePath, initrdPath, cmdline)

	// --- 1. Load Kernel Image ---
	if kernelImagePath == "" {
		return 0, 0, fmt.Errorf("kernel image path is required for VM %s", vm.ID)
	}
	fmt.Printf("Conceptual Bootloader: Reading kernel image from host path: %s\n", kernelImagePath)
	kernelData, errKernelRead := os.ReadFile(kernelImagePath)
	if errKernelRead != nil {
		return 0, 0, fmt.Errorf("failed to read kernel image %s: %w", kernelImagePath, errKernelRead)
	}

	// Basic bzImage header parsing to find actual start of kernel code (conceptual)
	// A real bzImage has a setup_header at offset 0x1F1.
	// setup_header.code32_start (or similar fields like pref_address) indicates load address.
	// For this conceptual step, we assume kernelLoadAddressGPA is the final entry point,
	// or that the bzImage is simple enough to be loaded and jumped to directly at this address.
	// A real VMM would parse setup_header.setup_sects to find the end of setup code.
	// kernelEntryPointActualGPA = kernelLoadAddressGPA + uint64(setup_sects_val+1)*512 (if setup is part of bzImage)
	kernelEntryPointActualGPA = kernelLoadAddressGPA // Simplification: assume kernelLoadAddressGPA is the entry point

	if kernelLoadAddressGPA+uint64(len(kernelData)) > vm.ramSizeBytes {
		return 0, 0, fmt.Errorf("kernel image (%d bytes) at 0x%X exceeds guest RAM size (0x%X)",
			len(kernelData), kernelLoadAddressGPA, vm.ramSizeBytes)
	}
	if kernelLoadAddressGPA < bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE &&
		kernelLoadAddressGPA+uint64(len(kernelData)) > bootParamsGPA {
		return 0, 0, fmt.Errorf("kernel load address 0x%X overlaps with boot_params page 0x%X - 0x%X",
			kernelLoadAddressGPA, bootParamsGPA, bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE-1)
	}

	copy(vm.guestMem[kernelLoadAddressGPA:], kernelData)
	fmt.Printf("Conceptual Bootloader: Copied %d bytes of kernel data to guest physical address 0x%X. Entry point: 0x%X\n",
		len(kernelData), kernelLoadAddressGPA, kernelEntryPointActualGPA)

	// --- 2. Load Initrd Image (Optional) ---
	var initrdSize uint32 = 0
	if initrdPath != "" {
		fmt.Printf("Conceptual Bootloader: Reading initrd image from host path: %s\n", initrdPath)
		initrdData, errInitrdRead := os.ReadFile(initrdPath)
		if errInitrdRead != nil {
			return 0, 0, fmt.Errorf("failed to read initrd image %s: %w", initrdPath, errInitrdRead)
		}
		initrdSize = uint32(len(initrdData))

		if initrdLoadAddressGPA+uint64(initrdSize) > vm.ramSizeBytes {
			return 0, 0, fmt.Errorf("initrd image (%d bytes) at 0x%X exceeds guest RAM size (0x%X)",
				initrdSize, initrdLoadAddressGPA, vm.ramSizeBytes)
		}
		if (initrdLoadAddressGPA < kernelLoadAddressGPA+uint64(len(kernelData)) && initrdLoadAddressGPA+uint64(initrdSize) > kernelLoadAddressGPA) ||
			(initrdLoadAddressGPA < bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE && initrdLoadAddressGPA+uint64(initrdSize) > bootParamsGPA) {
			return 0, 0, fmt.Errorf("initrd load address 0x%X overlaps with kernel (0x%X) or boot_params page (0x%X)",
				initrdLoadAddressGPA, kernelLoadAddressGPA, bootParamsGPA)
		}

		copy(vm.guestMem[initrdLoadAddressGPA:], initrdData)
		fmt.Printf("Conceptual Bootloader: Copied %d bytes of initrd data to guest physical address 0x%X\n",
			initrdSize, initrdLoadAddressGPA)
	}

	// --- 3. Setup boot_params (Zero Page) ---
	if bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE > vm.ramSizeBytes {
		return 0, 0, fmt.Errorf("boot_params page (0x%X - 0x%X) exceeds guest RAM size (0x%X)",
			bootParamsGPA, bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE-1, vm.ramSizeBytes)
	}

	bootParamsSlice := vm.guestMem[bootParamsGPA : bootParamsGPA+BOOT_PARAMS_ZERO_PAGE_SIZE]
	for i := range bootParamsSlice { bootParamsSlice[i] = 0 } // Zero out the page

	// Set type_of_loader (e.g., 0xFF for VMM)
	// This is typically in struct setup_header, which is part of boot_params.
	// Offset 0x210 in boot_params (Linux v5.x) for type_of_loader.
	// Or use the previously defined conceptual OFFSET_BP_TYPE_OF_LOADER.
	if bootParamsGPA+OFFSET_BP_TYPE_OF_LOADER < uint64(len(vm.guestMem)) {
		vm.guestMem[bootParamsGPA+OFFSET_BP_TYPE_OF_LOADER] = 0xFF
		fmt.Printf("Conceptual Bootloader: Set boot_params.type_of_loader = 0xFF at GPA 0x%X\n", bootParamsGPA+OFFSET_BP_TYPE_OF_LOADER)
	}

	// Place command line string in guest memory (e.g., after boot_params struct in the zero page)
	// and set cmd_line_ptr.
	cmdLineDestOffsetInZeroPage := uint64(BOOT_PARAMS_STRUCT_SIZE) // Place cmdline right after the struct
	cmdLineGPA := bootParamsGPA + cmdLineDestOffsetInZeroPage
	cmdLineMaxLen := BOOT_PARAMS_ZERO_PAGE_SIZE - cmdLineDestOffsetInZeroPage

	if uint64(len(cmdline))+1 > cmdLineMaxLen { // +1 for null terminator
		return 0, 0, fmt.Errorf("kernel command line too long (%d chars, max %d at GPA 0x%X)", len(cmdline), cmdLineMaxLen-1, cmdLineGPA)
	}
	copy(vm.guestMem[cmdLineGPA:], []byte(cmdline))
	vm.guestMem[cmdLineGPA+uint64(len(cmdline))] = 0 // Null terminate

	// Set cmd_line_ptr in boot_params (e.g., boot_params.hdr.cmd_line_ptr at offset 0x228)
	if bootParamsGPA+OFFSET_BP_CMD_LINE_PTR+4 <= uint64(len(vm.guestMem)) { // Ensure space for u32 pointer
		binary.LittleEndian.PutUint32(vm.guestMem[bootParamsGPA+OFFSET_BP_CMD_LINE_PTR:], uint32(cmdLineGPA))
		fmt.Printf("Conceptual Bootloader: Kernel command line '%s' placed at GPA 0x%X. Pointer set in boot_params at GPA 0x%X.\n",
			cmdline, cmdLineGPA, bootParamsGPA+OFFSET_BP_CMD_LINE_PTR)
	}

	if initrdSize > 0 {
		// Set ramdisk_image (GPA) and ramdisk_size in boot_params
		// These are often 64-bit fields in modern setup_header for 64-bit kernels.
		// Using conceptual 32-bit offsets for simplicity here, but a real VMM needs to be precise.
		if bootParamsGPA+OFFSET_BP_RAMDISK_IMAGE+4 <= uint64(len(vm.guestMem)) &&
		   bootParamsGPA+OFFSET_BP_RAMDISK_SIZE+4 <= uint64(len(vm.guestMem)) {
			binary.LittleEndian.PutUint32(vm.guestMem[bootParamsGPA+OFFSET_BP_RAMDISK_IMAGE:], uint32(initrdLoadAddressGPA))
			binary.LittleEndian.PutUint32(vm.guestMem[bootParamsGPA+OFFSET_BP_RAMDISK_SIZE:], initrdSize)
			fmt.Printf("Conceptual Bootloader: Ramdisk image GPA 0x%X and size %d set in boot_params (32-bit fields).\n",
				initrdLoadAddressGPA, initrdSize)
		}
	}

	// Populate E820 memory map (critical for kernel to know available RAM)
	// Example: one entry for all available RAM. A real VMM might add more entries for reserved areas.
	// struct e820entry { u64 addr; u64 size; u32 type; }
	e820MapOffset := bootParamsGPA + OFFSET_BP_E820_MAP
	if e820MapOffset+uint64(E820_ENTRY_SIZE) <= uint64(len(vm.guestMem)) { // Space for at least one entry
		// Entry 1: Main RAM (up to vm.ramSizeBytes or a bit less if low mem is reserved)
		// binary.LittleEndian.PutUint64(vm.guestMem[e820MapOffset+0:], 0x0)                     // addr
		// binary.LittleEndian.PutUint64(vm.guestMem[e820MapOffset+8:], vm.ramSizeBytes)        // size
		// binary.LittleEndian.PutUint32(vm.guestMem[e820MapOffset+16:], E820_TYPE_RAM) // type
		fmt.Printf("Conceptual Bootloader: E820 map: Entry 0: Addr=0x0, Size=0x%X, Type=RAM at GPA 0x%X\n", vm.ramSizeBytes, e820MapOffset)

		// Set number of E820 entries (boot_params.e820_entries at offset 0x1e8)
		if bootParamsGPA+OFFSET_BP_E820_MAP_ENTRIES < uint64(len(vm.guestMem)) {
			vm.guestMem[bootParamsGPA+OFFSET_BP_E820_MAP_ENTRIES] = 1 // Number of entries
		}
	}

	// Other fields like setup_header fields (vidmem_mode, etc.) would also be set from kernel image header.
	fmt.Printf("Conceptual Bootloader: boot_params (zero page) configured at GPA 0x%X.\n", bootParamsGPA)

	return kernelEntryPointActualGPA, bootParamsGPA, nil
}
