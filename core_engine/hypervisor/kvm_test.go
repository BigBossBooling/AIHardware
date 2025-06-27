package hypervisor

import (
	"log"
	"os"
	"testing"

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
