package hypervisor

import (
	"log"
	"os"
	// "sync" // No longer needed
	"testing"
	"time"

	"golang.org/x/sys/unix" // Added import for unix package
)

// checkKVMSupport checks if /dev/kvm exists and is accessible.
// It returns true if KVM seems usable, false otherwise.
// This is a helper for skipping tests gracefully if KVM is not available.
func checkKVMSupport(t *testing.T) bool {
	if _, err := os.Stat(kvmDevicePath); os.IsNotExist(err) {
		t.Logf("KVM device %s not found, skipping test.", kvmDevicePath)
		return false
	}
	// Try to open /dev/kvm to check permissions etc.
	// We don't need to get the API version here, NewKVMHypervisor will do that.
	f, err := os.OpenFile(kvmDevicePath, os.O_RDWR, 0)
	if err != nil {
		t.Logf("KVM device %s not accessible (err: %v), skipping test.", kvmDevicePath, err)
		return false
	}
	f.Close()
	return true
}

func TestNewKVMHypervisor(t *testing.T) {
	if !checkKVMSupport(t) {
		t.SkipNow()
	}

	log.Println("TestNewKVMHypervisor: Starting test")
	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("NewKVMHypervisor() failed: %v", err)
	}
	if hypervisor == nil {
		t.Fatal("NewKVMHypervisor() returned nil hypervisor without error")
	}
	if hypervisor.kvmFd == nil {
		t.Fatal("NewKVMHypervisor() returned hypervisor with nil kvmFd")
	}
	log.Printf("TestNewKVMHypervisor: KVM fd: %d", hypervisor.kvmFd.Fd())

	// Check if the fd is valid by trying a simple operation, e.g. getting API version again
	// (although NewKVMHypervisor already does this). This is more of a sanity check.
	version, err := unix.IoctlRetInt(int(hypervisor.kvmFd.Fd()), ioctl_KVM_GET_API_VERSION)
	if err != nil {
		t.Errorf("ioctl KVM_GET_API_VERSION on hypervisor.kvmFd failed: %v", err)
	}
	if version != KVM_API_VERSION {
		t.Errorf("ioctl KVM_GET_API_VERSION on hypervisor.kvmFd returned %d, want %d", version, KVM_API_VERSION)
	}

	log.Println("TestNewKVMHypervisor: About to close hypervisor")
	err = hypervisor.Close()
	if err != nil {
		t.Errorf("hypervisor.Close() failed: %v", err)
	}
	log.Println("TestNewKVMHypervisor: Hypervisor closed")

	// Test closing already closed hypervisor
	err = hypervisor.Close()
	if err != nil {
		t.Errorf("hypervisor.Close() on already closed hypervisor failed: %v", err)
	}
	log.Println("TestNewKVMHypervisor: Finished test")
}

func TestKVMHypervisor_CreateVM(t *testing.T) {
	if !checkKVMSupport(t) {
		t.SkipNow()
	}

	log.Println("TestKVMHypervisor_CreateVM: Starting test")
	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("Failed to create KVMHypervisor for CreateVM test: %v", err)
	}
	defer hypervisor.Close()

	log.Println("TestKVMHypervisor_CreateVM: Attempting to create VM")
	vm, err := hypervisor.CreateVM()
	if err != nil {
		t.Fatalf("hypervisor.CreateVM() failed: %v", err)
	}
	if vm == nil {
		t.Fatal("hypervisor.CreateVM() returned nil vm without error")
	}
	if vm.vmFd <= 0 { // FD should be > 0
		t.Fatalf("hypervisor.CreateVM() returned vm with invalid vmFd: %d", vm.vmFd)
	}
	log.Printf("TestKVMHypervisor_CreateVM: VM created with fd: %d", vm.vmFd)

	// Check if the VM fd is valid by trying to close it.
	// A more robust check would be another KVM ioctl on the VM fd if one exists that doesn't require more setup.
	log.Println("TestKVMHypervisor_CreateVM: Attempting to close VM")
	err = vm.Close()
	if err != nil {
		t.Errorf("vm.Close() failed: %v", err)
	}
	log.Println("TestKVMHypervisor_CreateVM: VM closed")

	// Test closing already closed VM
	err = vm.Close()
	if err != nil {
		t.Errorf("vm.Close() on already closed VM failed: %v", err)
	}
	log.Println("TestKVMHypervisor_CreateVM: Finished test")
}

func TestCreateVMOnClosedHypervisor(t *testing.T) {
	if !checkKVMSupport(t) {
		t.SkipNow()
	}

	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("NewKVMHypervisor() failed: %v", err)
	}
	err = hypervisor.Close()
	if err != nil {
		t.Fatalf("hypervisor.Close() failed: %v", err)
	}

	// Attempt to create VM on closed hypervisor
	_, err = hypervisor.CreateVM()
	if err == nil {
		t.Error("hypervisor.CreateVM() on closed hypervisor should have failed, but didn't")
	} else {
		t.Logf("hypervisor.CreateVM() on closed hypervisor failed as expected: %v", err)
	}
}

func TestVM_StartStop(t *testing.T) {
	if !checkKVMSupport(t) {
		t.SkipNow()
	}

	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("Failed to create KVMHypervisor: %v", err)
	}
	defer hypervisor.Close()

	vm, err := hypervisor.CreateVM()
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}
	// vm.Close will be called by the test's cleanup, or explicitly if Start fails.
	// Using a t.Cleanup to ensure vm.Close is called.
	t.Cleanup(func() {
		log.Println("TestVM_StartStop: Cleanup - closing VM.")
		if err := vm.Close(); err != nil {
			// Log error during cleanup, but don't fail the test here as it might already be failing.
			t.Logf("TestVM_StartStop: Cleanup - error closing VM: %v", err)
		}
		log.Println("TestVM_StartStop: Cleanup - VM closed.")
	})


	if vm.state != StateStopped {
		t.Fatalf("VM initial state should be StateStopped, got %s", vm.state)
	}

	// Start the VM
	log.Println("TestVM_StartStop: Attempting to start VM.")
	err = vm.Start()
	if err != nil {
		// If KVM_RUN fails immediately (e.g. no memory, no registers set), this is expected for now.
		// Our current runLoop exits on the first KVM_RUN success because no guest code is running to HLT/Shutdown.
		// So, Start() might return nil, but runLoop exits quickly.
		// Let's check the state after a brief moment.
		t.Logf("vm.Start() returned: %v. This might be okay if runLoop exits quickly due to no guest.", err)
		// We expect Start to succeed in launching the runLoop, even if KVM_RUN then exits.
		// The error from Start() itself should ideally be nil if the setup up to launching runLoop is okay.
		// Let's assume for now that Start() failing is a test failure unless it's a very specific "no guest" error.
		// For this phase, any error from Start() is unexpected if KVM setup (VCPU, mmap) is okay.
		// The current runLoop will exit immediately as KVM_RUN will likely fail_entry or internal_error.
		// This means the VM might transition to Stopped or Error very quickly.
		// So, an error from Start() IS a problem.
		// Actually, Start() itself will error out if KVM_CREATE_VCPU or KVM_GET_VCPU_MMAP_SIZE fails.
		// If those succeed, and runLoop is launched, Start() returns nil.
		// The runLoop then calls KVM_RUN.
		// Let's refine this: If Start() returns an error, it's a failure.
		// if err != nil {
		// 	t.Fatalf("vm.Start() failed: %v", err)
		// }
		// Re-evaluating: Start() itself should return nil if setup is okay. The runLoop handles KVM_RUN issues.
		// If KVM_RUN fails immediately in the runLoop, the VM state will become StateError or StateStopped.
		// This is the expected behavior without a guest.
		// So, vm.Start() should succeed.
		if err != nil {
			t.Fatalf("vm.Start() failed unexpectedly: %v", err)
		}
	}
	log.Println("TestVM_StartStop: vm.Start() called.")


	// Wait a very short moment to allow the runLoop to potentially execute KVM_RUN once and exit.
	// Our current runLoop exits on the first successful KVM_RUN (as it's not a real guest).
	// Or if KVM_RUN itself errors.
	time.Sleep(50 * time.Millisecond) // Give runLoop a chance to run and exit

	vm.mu.Lock()
	currentState := vm.state
	vm.mu.Unlock()
	log.Printf("TestVM_StartStop: VM state after Start and short delay: %s", currentState)

	// Given the current runLoop behavior (exits after first KVM_RUN "success" or error),
	// the state should be StateStopped or StateError.
	if currentState != StateStopped && currentState != StateError {
		t.Errorf("VM state after Start() and short delay should be StateStopped or StateError due to immediate KVM_RUN exit, got %s", currentState)
	}


	// Stop the VM
	// Since the runLoop likely exited quickly, Stop() should reflect this.
	log.Println("TestVM_StartStop: Attempting to stop VM.")
	err = vm.Stop()
	if err != nil {
		t.Errorf("vm.Stop() failed: %v", err)
	}
	log.Println("TestVM_StartStop: vm.Stop() called.")

	vm.mu.Lock()
	finalState := vm.state
	vm.mu.Unlock()
	log.Printf("TestVM_StartStop: VM state after Stop: %s", finalState)

	if finalState != StateStopped && finalState != StateError { // Could be Error if KVM_RUN failed badly
		t.Errorf("VM state after Stop() should be StateStopped or StateError, got %s", finalState)
	}

	// Try starting again (should fail if not StateStopped, or succeed if it is StateStopped)
	// This depends on whether the previous stop resulted in Stopped or Error.
	// If it was Error, it can't be started. If Stopped, it can.
	if finalState == StateStopped {
		log.Println("TestVM_StartStop: Attempting to start VM again after stop.")
		err = vm.Start()
		if err != nil {
			t.Errorf("vm.Start() again failed: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // allow runloop to exit
		err = vm.Stop() // Stop it again
		if err != nil {
			t.Errorf("vm.Stop() again failed: %v", err)
		}
		vm.mu.Lock()
		if vm.state != StateStopped && vm.state != StateError {
			t.Errorf("VM state after second Stop() should be StateStopped or StateError, got %s", vm.state)
		}
		vm.mu.Unlock()
	} else {
		log.Printf("TestVM_StartStop: VM was in state %s, not attempting second start.", finalState)
	}


	log.Println("TestVM_StartStop: Test finished.")
}


func TestVM_PauseResume_StateOnly(t *testing.T) {
	// This test does not require KVM as it only tests state transitions for now.
	// However, it needs a VM object.
	// If we want to test this independently of KVM, we'd need to mock NewKVMHypervisor and CreateVM,
	// or make VirtualMachine constructible without them for testing states.
	// For now, let's make it depend on KVM availability for VM creation.
	if !checkKVMSupport(t) {
		t.Skip("KVM not available, skipping Pause/Resume state test that needs a VM object.")
	}

	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("Failed to create KVMHypervisor: %v", err)
	}
	defer hypervisor.Close()

	vm, err := hypervisor.CreateVM()
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}
	t.Cleanup(func() { vm.Close() })


	// Should not be able to pause a stopped VM
	err = vm.Pause()
	if err == nil {
		t.Error("Pausing a stopped VM should fail, but it succeeded.")
	}
	if vm.state != StateStopped {
		t.Errorf("VM state should remain StateStopped after failed Pause, got %s", vm.state)
	}

	// Start the VM
	err = vm.Start()
	if err != nil {
		// As in TestVM_StartStop, runLoop will exit quickly.
		// This test focuses on Pause/Resume state changes assuming Start "worked" enough to change state.
		t.Logf("vm.Start() in PauseResume test returned: %v. Proceeding to check states.", err)
		// If Start itself errors, it's a problem for this test flow.
		// Let's assume the test environment for KVM allows Start() to not error out during setup.
		if err != nil { // Re-check after log
			t.Fatalf("vm.Start() failed unexpectedly in PauseResume test: %v", err)
		}
	}

	// Wait for runloop to start and potentially exit quickly
	time.Sleep(50 * time.Millisecond)

	// Current runLoop exits immediately. So state will be Stopped or Error.
	// To test Pause/Resume properly, we need a runLoop that stays running.
	// For now, we can "force" the state to Running for the purpose of testing Pause/Resume logic.
	// This is a temporary hack for this phase.
	vm.mu.Lock()
	originalStateAfterStart := vm.state
	vm.state = StateRunning // Force state for testing Pause
	log.Printf("TestVM_PauseResume_StateOnly: Forcing VM state to Running (was %s) to test Pause.", originalStateAfterStart)
	vm.mu.Unlock()


	// Pause the running VM
	err = vm.Pause()
	if err != nil {
		t.Errorf("Pausing a running VM failed: %v", err)
	}
	if vm.state != StatePaused {
		t.Errorf("VM state should be StatePaused, got %s", vm.state)
	}

	// Try pausing an already paused VM
	err = vm.Pause()
	if err == nil {
		t.Error("Pausing an already paused VM should fail, but it succeeded.")
	}

	// Resume the paused VM
	err = vm.Resume()
	if err != nil {
		t.Errorf("Resuming a paused VM failed: %v", err)
	}
	if vm.state != StateRunning { // Conceptually back to running
		t.Errorf("VM state should be StateRunning after Resume, got %s", vm.state)
	}

	// Try resuming an already running VM
	err = vm.Resume()
	if err == nil {
		t.Error("Resuming an already running VM should fail, but it succeeded.")
	}

	// Clean up: Stop the VM (it's conceptually running, or was forced to running)
	// Restore original state if it was Error, otherwise set to Stopped for Stop() to work.
	vm.mu.Lock()
	if originalStateAfterStart == StateError {
		vm.state = StateError // Restore error if that's what it was
	} else {
		// If it was Stopped, and we forced it to Running, Stop needs it to be Running or Paused.
		// We set it to Running after Resume, so Stop should work.
	}
	vm.mu.Unlock()

	err = vm.Stop()
	if err != nil {
		t.Errorf("Stopping VM at end of PauseResume test failed: %v", err)
	}
	if vm.state != StateStopped && vm.state != StateError {
		t.Errorf("VM state should be StateStopped or StateError after final Stop, got %s", vm.state)
	}
}

func TestVM_AddMemoryRegion(t *testing.T) {
	if !checkKVMSupport(t) {
		t.SkipNow()
	}

	hypervisor, err := NewKVMHypervisor()
	if err != nil {
		t.Fatalf("Failed to create KVMHypervisor: %v", err)
	}
	defer hypervisor.Close()

	vm, err := hypervisor.CreateVM()
	if err != nil {
		t.Fatalf("Failed to create VM: %v", err)
	}
	// Ensure VM is closed to unmap memory regions
	t.Cleanup(func() {
		log.Println("TestVM_AddMemoryRegion: Cleanup - closing VM.")
		if err := vm.Close(); err != nil {
			t.Logf("TestVM_AddMemoryRegion: Cleanup - error closing VM: %v", err)
		}
		log.Println("TestVM_AddMemoryRegion: Cleanup - VM closed.")
	})


	pageSize := uint64(os.Getpagesize())
	memSize := pageSize * 16 // 16 pages, e.g., 64KB if page is 4KB

	// 1. Add a valid memory region
	log.Println("TestVM_AddMemoryRegion: Attempting to add valid memory region.")
	region1, err := vm.AddMemoryRegion(0, 0x0, memSize, 0, false)
	if err != nil {
		t.Fatalf("AddMemoryRegion(slot 0) failed: %v", err)
	}
	if region1 == nil {
		t.Fatal("AddMemoryRegion(slot 0) returned nil region without error.")
	}
	if len(vm.memoryRegions) != 1 {
		t.Fatalf("Expected 1 memory region, got %d", len(vm.memoryRegions))
	}
	if vm.memoryRegions[0] != region1 {
		t.Fatal("VM memoryRegions[0] is not the returned region1.")
	}
	if region1.backingStore == nil {
		t.Fatal("region1.backingStore is nil after successful AddMemoryRegion.")
	}
	if len(region1.backingStore) != int(memSize) {
		t.Fatalf("region1.backingStore length is %d, want %d", len(region1.backingStore), int(memSize))
	}
	if region1.GuestPhysAddr != 0x0 {
		t.Errorf("region1.GuestPhysAddr is 0x%x, want 0x0", region1.GuestPhysAddr)
	}
	log.Printf("TestVM_AddMemoryRegion: Added region1: Slot=%d, GPA=0x%x, Size=0x%x, HUA=0x%x",
		region1.Slot, region1.GuestPhysAddr, region1.MemorySize, region1.HostUserAddr)


	// 2. Add another valid memory region
	log.Println("TestVM_AddMemoryRegion: Attempting to add second valid memory region.")
	region2GPA := memSize // Place it right after the first region
	region2, err := vm.AddMemoryRegion(1, region2GPA, memSize/2, KVM_MEM_READONLY, true) // Read-only
	if err != nil {
		t.Fatalf("AddMemoryRegion(slot 1) failed: %v", err)
	}
	if region2 == nil {
		t.Fatal("AddMemoryRegion(slot 1) returned nil region without error.")
	}
	if len(vm.memoryRegions) != 2 {
		t.Fatalf("Expected 2 memory regions, got %d", len(vm.memoryRegions))
	}
	if !region2.readOnly || (region2.Flags&KVM_MEM_READONLY == 0) {
		t.Error("region2 should be read-only but flags do not reflect it.")
	}
	log.Printf("TestVM_AddMemoryRegion: Added region2: Slot=%d, GPA=0x%x, Size=0x%x, HUA=0x%x, ReadOnly=%v",
		region2.Slot, region2.GuestPhysAddr, region2.MemorySize, region2.HostUserAddr, region2.readOnly)


	// 3. Test adding with zero size (should fail)
	log.Println("TestVM_AddMemoryRegion: Attempting to add zero-size memory region.")
	_, err = vm.AddMemoryRegion(2, region2GPA+memSize/2, 0, 0, false)
	if err == nil {
		t.Error("AddMemoryRegion with zero size should have failed, but didn't.")
	} else {
		t.Logf("AddMemoryRegion with zero size failed as expected: %v", err)
	}


	// 4. Test adding memory while VM is "running" (should fail)
	// Force state to running for this test part.
	// Note: The VM is not actually running KVM_RUN here, just state manipulation.
	vm.mu.Lock()
	originalState := vm.state
	vm.state = StateRunning
	vm.mu.Unlock()
	log.Println("TestVM_AddMemoryRegion: Attempting to add memory region while VM state is Running.")
	_, err = vm.AddMemoryRegion(3, region2GPA+memSize, pageSize, 0, false)
	if err == nil {
		t.Error("AddMemoryRegion while VM is Running should have failed, but didn't.")
	} else {
		t.Logf("AddMemoryRegion while VM is Running failed as expected: %v", err)
	}
	vm.mu.Lock()
	vm.state = originalState // Restore original state
	vm.mu.Unlock()


	// 5. Test Close unmaps memory (implicitly tested by t.Cleanup and no panic/error from Close)
	// To be more explicit, we could check if region.backingStore becomes nil after Close,
	// but Close() itself nils out vm.memoryRegions.
	// The main check is that Close() runs without erroring on Munmap.
	// If we wanted to verify, we'd have to Close here and then inspect.
	// But Cleanup handles the Close.
	log.Println("TestVM_AddMemoryRegion: Test finished. VM Close in Cleanup will test Munmap.")
}
