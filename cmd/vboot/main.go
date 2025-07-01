package main

import (
	"fmt"
	"log"
	// "os" // No longer needed after removing os.ReadFile example
	"v-architect/core_engine" // Adjusted import path
)

func main() {
	fmt.Println("V-Architect Boot Test Utility")

	// Use the placeholder bootloader from core_engine
	// For a real scenario, you might read this from a file:
	// bootloaderBytes, err := os.ReadFile("path/to/your/bootloader.bin")
	// if err != nil {
	//	 log.Fatalf("Failed to read bootloader file: %v", err)
	// }
	bootloaderBytes := core_engine.MinimalBootloader

	// Define VM parameters
	// memorySize := uint64(64 * 1024 * 1024) // 64MB
	memorySize := uint64(1 * 1024 * 1024) // 1MB is sufficient for this minimal test

	fmt.Printf("Attempting to create VM with %dMB memory and bootloader (size %d bytes)...\n", memorySize/(1024*1024), len(bootloaderBytes))

	// Create the Virtual Machine
	vm, err := core_engine.CreateVM(memorySize, bootloaderBytes)
	if err != nil {
		log.Fatalf("Failed to create VM: %v", err)
	}
	fmt.Println("VM Created successfully.")

	// Defer stopping the VM to ensure cleanup
	defer func() {
		fmt.Println("Stopping VM...")
		vm.Stop()
		fmt.Println("VM stopped.")
	}()

	// Run the Virtual Machine
	fmt.Println("Running VM...")
	if err := vm.Run(); err != nil {
		// Check if the error is due to a clean halt or shutdown, which might not be a "failure" for this test.
		// However, vm.Run() currently returns error on KVM_EXIT_HLT.
		// For this test, an error from Run() after KVM_EXIT_HLT is expected.
		// The underlying KVM_RUN ioctl wrapper (KvmIoctlRun) also propagates errors.
		// We expect an error here if the HLT instruction in the bootloader was reached.
		fmt.Printf("VM Run exited. Last KVM exit reason might indicate success (e.g., HLT): %v\n", err)
		// Specific error checking could be added here if vm.Run() returned more structured exit info.
	} else {
		fmt.Println("VM Run completed without error (this might be unexpected if HLT was supposed to cause an exit error).")
	}

	fmt.Println("Boot test finished. Check console for serial output from bootloader (expected 'H').")
}
