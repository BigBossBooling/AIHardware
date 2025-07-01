package core_engine

import (
	// "fmt" // No longer needed
	"os"
	"testing"
	// "v-architect/core_engine/devices" // Not directly needed if using existing CreateVM
)

// checkKVM checks if /dev/kvm is accessible.
// Returns true if KVM is likely available, false otherwise.
func checkKVM() bool {
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		return false
	}
	// Further checks could involve trying to open /dev/kvm,
	// but for a simple skip, stat is often enough.
	// f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	// if err != nil {
	// 	return false
	// }
	// f.Close()
	return true
}

func TestMinimalBootAndHalt(t *testing.T) {
	if !checkKVM() {
		t.Skip("Skipping KVM integration test: /dev/kvm not available or accessible.")
		return
	}

	t.Log("KVM device found, proceeding with MinimalBootAndHalt test.")

	bootloaderBytes := JustHaltBootloader // Defined in bootloader_placeholder.go
	memorySize := uint64(1 * 1024 * 1024) // 1MB

	t.Logf("Attempting to create VM with bootloader (size %d bytes).", len(bootloaderBytes))

	vm, err := CreateVM(memorySize, bootloaderBytes)
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}
	t.Log("VM Created successfully.")

	defer func() {
		t.Log("Stopping VM in test cleanup...")
		vm.Stop()
		t.Log("VM stopped in test cleanup.")
	}()

	t.Log("Running VM...")
	runErr := vm.Run()

	// In our current setup, vm.Run() is expected to return an error when the guest halts
	// because KVM_EXIT_HLT causes the KVM_RUN ioctl to exit, and this is propagated.
	if runErr != nil {
		// This is the expected path for a HLT exit.
		// We could try to make the error more specific if KvmIoctlRun or vm.Run could return typed errors.
		// For now, any error after HLT is "success" for this test meaning it didn't loop forever.
		t.Logf("VM Run exited as expected (due to HLT): %v", runErr)
	} else {
		// This would be unexpected if the bootloader only contains HLT.
		t.Errorf("VM Run completed without error, but an exit (e.g., HLT) was expected.")
	}
	t.Log("MinimalBootAndHalt test finished.")
}

// TestBootPrintAndHalt would be a more advanced test:
// - Uses MinimalBootloader (prints 'H' then HLTs)
// - Redirects os.Stdout (or uses a configurable writer for SerialPortDevice)
// - Checks if 'H' was printed to the captured output.
// - Checks for HLT exit.
// This is more involved due to output capture.
/*
func TestBootPrintAndHalt(t *testing.T) {
	if !checkKVM() {
		t.Skip("Skipping KVM integration test: /dev/kvm not available.")
		return
	}

	// 1. Redirect stdout to capture serial output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout // Restore stdout
		w.Close()
	}()

	bootloaderBytes := MinimalBootloader
	memorySize := uint64(1 * 1024 * 1024) // 1MB

	vm, err := CreateVM(memorySize, bootloaderBytes)
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}
	defer vm.Stop()

	runErr := vm.Run()
	w.Close() // Close writer to allow reader to finish

	if runErr == nil {
		t.Errorf("VM Run completed without error, but an exit (HLT) was expected.")
	} else {
		t.Logf("VM Run exited as expected (due to HLT): %v", runErr)
	}

	capturedOutputBytes, _ := io.ReadAll(r)
	capturedOutput := string(capturedOutputBytes)

	expectedSerialOutput := "H"
	if !strings.Contains(capturedOutput, expectedSerialOutput) {
		t.Errorf("Expected serial output to contain '%s', got '%s'", expectedSerialOutput, capturedOutput)
	} else {
		t.Logf("Serial output successfully captured and contains '%s'. Full output: %s", expectedSerialOutput, capturedOutput)
	}
}
*/
