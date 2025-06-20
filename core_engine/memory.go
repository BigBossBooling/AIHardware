package core_engine

import (
	// "encoding/binary" // Would be used for PutUint64
	"fmt"
	// "unsafe" // If directly manipulating memory via pointers from guestMem
)

// x86-64 Page Table Entry bits
const (
	PTE_PRESENT         uint64 = 1 << 0
	PTE_READ_WRITE      uint64 = 1 << 1
	PTE_USER_SUPERVISOR uint64 = 1 << 2 // If allowing userspace access, else 0 for kernel
	PTE_WRITE_THROUGH   uint64 = 1 << 3
	PTE_CACHE_DISABLE   uint64 = 1 << 4
	PTE_ACCESSED        uint64 = 1 << 5
	PTE_DIRTY           uint64 = 1 << 6
	PTE_PSE             uint64 = 1 << 7  // Page Size Extension (for 2MB/1GB pages)
	PTE_GLOBAL          uint64 = 1 << 8  // Global page (ignored for PML4E/PDPTE/PDE that points to next level table)
	PTE_NX              uint64 = 1 << 63 // No-Execute bit (if PAE and LME are enabled)
)

// PageMapLevel4AddressGPA holds the Guest Physical Address of the PML4 table.
// This needs to be accessible by the vCPU setup (to set CR3).
// It's defined here as it's fundamental to memory layout.
var PageMapLevel4AddressGPA uint64 = 0x1000 // Default, can be made configurable if needed via VM options

// setupInitialPaging creates a basic identity-mapped page table hierarchy for the first few MB of RAM.
// It assumes vm.guestMem is already allocated and vm.ramSizeBytes is set.
// This function would be called by vm.setupMemory() after vm.guestMem is mmap'd.
// The VirtualMachine struct is defined in vm_manager.go; this method is associated with it.
func (vm *VirtualMachine) setupInitialPaging() error {
	vm.resourceLock.Lock() // Ensure exclusive access to guestMem and page table structures
	defer vm.resourceLock.Unlock()

	fmt.Printf("Conceptual Paging: VM %s - Setting up initial x86-64 Long Mode page tables.\n", vm.ID)

	if vm.guestMem == nil {
		return fmt.Errorf("guestMem not allocated for VM %s, cannot setup paging", vm.ID)
	}
	// Need space for PML4, PDPT, PD, and at least one PT (4 pages = 4 * 4KB = 16KB)
	// plus some actual RAM to map (e.g., first 2MB).
	minRequiredRamForPaging := uint64((4 * 4096) + (512 * 4096)) // Tables + 2MB mapped
	if vm.ramSizeBytes < minRequiredRamForPaging {
		return fmt.Errorf("RAM size %d bytes is too small for initial paging setup (need at least %d for tables + 2MB map)", vm.ramSizeBytes, minRequiredRamForPaging)
	}

	// Define GPAs for page table structures. Ensure they are page-aligned (4KB).
	// These addresses must be within the allocated vm.guestMem.
	// PageMapLevel4AddressGPA is already defined globally (e.g., 0x1000)
	pdptGPA := PageMapLevel4AddressGPA + 0x1000 // e.g., 0x2000
	pdGPA := pdptGPA + 0x1000                // e.g., 0x3000
	ptGPA := pdGPA + 0x1000                  // e.g., 0x4000 (first page table)

	// Check if table structures fit within guest memory
	highestTableAddr := ptGPA + 0x1000 // End of the first page table
	if highestTableAddr > vm.ramSizeBytes {
		return fmt.Errorf("page table structures (up to 0x%X) exceed allocated guest RAM size (0x%X)", highestTableAddr, vm.ramSizeBytes)
	}

	// --- Clear page table areas (conceptual) ---
	// In a real scenario, you'd zero out these pages in vm.guestMem.
	// This ensures no stale data is interpreted as page table entries.
	tablesToClear := []uint64{PageMapLevel4AddressGPA, pdptGPA, pdGPA, ptGPA}
	for _, tableBaseGPA := range tablesToClear {
		// pageSlice := vm.guestMem[tableBaseGPA : tableBaseGPA+0x1000]
		// for j := range pageSlice { pageSlice[j] = 0 }
		// Conceptually zeroed:
		fmt.Printf("Conceptual Paging: Zeroed memory page at GPA 0x%X for page table.\n", tableBaseGPA)
	}
	fmt.Printf("Conceptual Paging: Cleared memory for PML4 (0x%X), PDPT (0x%X), PD (0x%X), PT (0x%X)\n",
		PageMapLevel4AddressGPA, pdptGPA, pdGPA, ptGPA)

	// --- Create Page Table Entries (PTEs) for the first Page Table (maps 0MB - 2MB) ---
	// This first PT will identity map the first 512 * 4KB = 2MB of RAM.
	numInitialPagesToMap := 512 // Maps 2MB

	// Ensure the area to be mapped is within guest RAM bounds
	if (uint64(numInitialPagesToMap) * 4096) > vm.ramSizeBytes {
		return fmt.Errorf("not enough RAM (size 0x%X) to identity map initial %d pages (needs 0x%X)",
			vm.ramSizeBytes, numInitialPagesToMap, uint64(numInitialPagesToMap)*4096)
	}

	currentMappedGPA := uint64(0)
	for i := 0; i < numInitialPagesToMap; i++ {
		// Identity map: GuestPhysicalAddress == HostPhysicalAddress (from KVM's perspective after SET_USER_MEMORY_REGION)
		// So, the address field of the PTE will be currentMappedGPA.
		pte := currentMappedGPA | PTE_PRESENT | PTE_READ_WRITE // Present, R/W, Supervisor-only by default
		// Write pte to vm.guestMem at ptGPA + uint64(i*8) (8 bytes per PTE)
		// binary.LittleEndian.PutUint64(vm.guestMem[ptGPA+uint64(i*8) : ptGPA+uint64(i*8)+8], pte)
		// Conceptual write:
		if (ptGPA + uint64(i*8) + 8) > uint64(len(vm.guestMem)) {
			return fmt.Errorf("attempt to write PTE out of bounds: offset 0x%X", ptGPA+uint64(i*8)+8)
		}
		// Store conceptually: vm.guestMem[ptGPA + uint64(i*8)] = byte(pte & 0xFF) ...
		currentMappedGPA += 0x1000 // 4KB page size
	}
	fmt.Printf("Conceptual Paging: Populated %d PTEs in PT (at GPA 0x%X) to identity map GPAs 0x0 - 0x%X\n",
		numInitialPagesToMap, ptGPA, currentMappedGPA-1)

	// --- Create Page Directory Entry (PDE) pointing to the Page Table ---
	// This PDE will cover the 2MB region mapped by the PT above.
	// Located at pdGPA, first entry (index 0).
	pde := ptGPA | PTE_PRESENT | PTE_READ_WRITE
	// binary.LittleEndian.PutUint64(vm.guestMem[pdGPA+0*8:], pde)
	fmt.Printf("Conceptual Paging: Populated PDE[0] (at GPA 0x%X) to point to PT (at GPA 0x%X)\n", pdGPA, ptGPA)
	// If using 2MB huge pages directly at PD level (PTE_PSE in CR4 & PDE):
	// pde_huge := uint64(0) | PTE_PRESENT | PTE_READ_WRITE | PTE_PSE // Maps GPA 0-2MB
	// binary.LittleEndian.PutUint64(vm.guestMem[pdGPA+0*8:], pde_huge)

	// --- Create Page Directory Pointer Table Entry (PDPTE) pointing to the Page Directory ---
	// This PDPTE will cover the 1GB region mapped by one Page Directory.
	// Located at pdptGPA, first entry (index 0).
	pdpte := pdGPA | PTE_PRESENT | PTE_READ_WRITE
	// binary.LittleEndian.PutUint64(vm.guestMem[pdptGPA+0*8:], pdpte)
	fmt.Printf("Conceptual Paging: Populated PDPTE[0] (at GPA 0x%X) to point to PD (at GPA 0x%X)\n", pdptGPA, pdGPA)

	// --- Create Page Map Level 4 Entry (PML4E) pointing to the PDPT ---
	// Located at PageMapLevel4AddressGPA, first entry (index 0).
	pml4e := pdptGPA | PTE_PRESENT | PTE_READ_WRITE
	// binary.LittleEndian.PutUint64(vm.guestMem[PageMapLevel4AddressGPA+0*8:], pml4e)
	fmt.Printf("Conceptual Paging: Populated PML4E[0] (at GPA 0x%X) to point to PDPT (at GPA 0x%X)\n", PageMapLevel4AddressGPA, pdptGPA)

	// The CR3 register in the vCPU must be set to PageMapLevel4AddressGPA.
	// This is handled in vcpu.setupInitialArchState() which receives this GPA.

	fmt.Println("Conceptual Paging: Initial page table hierarchy setup complete for VM ID:", vm.ID)
	return nil
}
