package core_engine

import (
	"encoding/json" // Used by SaveVMConfigToFile conceptually, and for test setup
	"fmt"
	"os"
	"path/filepath"
	"testing"

	// Assuming pb types are generated and accessible via this import path
	pb "github.com/V-Architect/v-architect-core/core_engine/pb"
	// "github.com/stretchr/testify/assert" // Popular assertion library
	// "github.com/stretchr/testify/require"
	// "google.golang.org/protobuf/proto" // For proto.Equal
)

// Helper to create a valid pb.VMConfig for tests
func newValidTestVMConfig() *pb.VMConfig {
	// Ensure all required fields from the proto definition are included for successful validation
	return &pb.VMConfig{
		VmId:         "test-vm-uuid-1234",
		VmName:       "TestVM1",
		Architecture: "x86-64",
		OsType:       "ubuntu_server_22.04_x64", // Matches conceptual os_type
		VcpuConfig:   &pb.VCPUConfig{Count: 2, Topology: &pb.VCPUConfig_Topology{Sockets: 1, CoresPerSocket: 2, ThreadsPerCore: 1}},
		VramConfig:   &pb.VRAMConfig{SizeMb: 2048, MemoryBallooningEnabled: true, AiOptimizedFlags: &pb.VRAMConfig_AIOptimizedFlags{PreferContiguous: false, NumaNodeAffinity: 0}},
		StorageDevices: []*pb.StorageDevice{
			{DiskId: "disk0", ImagePath: "/images/disk0.qcow2", ControllerType: pb.StorageDevice_VIRTIO_BLK, SizeGb: 20, IsBootDisk: true, Format: pb.StorageDevice_QCOW2, SerialNumber: "disk0-serial"},
		},
		NetworkInterfaces: []*pb.NetworkInterface{
			{NicId: "net0", VnicModel: pb.NetworkInterface_VIRTIO_NET, NetworkAttachmentId: "default_nat_bridge", MacAddress: "DE:AD:BE:EF:00:01"},
		},
		GraphicsConfig: &pb.GraphicsConfig{Type: pb.GraphicsConfig_VIRTIO_GPU, DisplayResolutionWidth: 1920, DisplayResolutionHeight: 1080},
		PciPassthroughDevices: []*pb.PCIPassthroughDevice{}, // Initialize as empty slice
		AiAcceleratedHardware: &pb.AIAcceleratedHardware{   // Initialize, even if sub-fields are nil
			AiCpuConfig:          nil,
			AiGraphicsCardConfig: nil,
		},
		BootOrder:          []string{"disk0"},
		FirmwareType:       "uefi",
		SecureBootEnabled:  true,
		VirtualTpmEnabled:  true,
		SerialPorts:        []*pb.SerialPortConfig{},
	}
}

func TestSaveAndLoadVMConfig(t *testing.T) {
	// require := require.New(t) // testify helper
	// assert := assert.New(t)   // testify helper

	config := newValidTestVMConfig()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_vm_config.json")

	fmt.Printf("Conceptual Test: TestSaveAndLoadVMConfig - Saving config to %s\n", filePath)
	err := SaveVMConfigToFile(config, filePath)
	// require.NoError(err, "Saving VM config should not produce an error")
	if err != nil {
		t.Fatalf("SaveVMConfigToFile failed: %v", err)
	}

	// Verify file exists
	if _, errStat := os.Stat(filePath); os.IsNotExist(errStat) {
		t.Fatalf("SaveVMConfigToFile did not create the file %s", filePath)
	}

	fmt.Printf("Conceptual Test: TestSaveAndLoadVMConfig - Loading config from %s\n", filePath)
	loadedConfig, err := LoadVMConfigFromFile(filePath)
	// require.NoError(err, "Loading VM config should not produce an error")
	if err != nil {
		t.Fatalf("LoadVMConfigFromFile failed: %v", err)
	}
	// require.NotNil(loadedConfig, "Loaded config should not be nil")
	if loadedConfig == nil {
		t.Fatalf("Loaded config is nil")
	}


	// Basic comparison. For protobuf, using proto.Equal is best for deep comparison.
	// For conceptual, comparing a few key fields.
	// success := proto.Equal(config, loadedConfig) // This would be ideal
	// if !success {
	//     t.Errorf("Loaded config does not match saved config.\nExpected: %v\nGot: %v", config, loadedConfig)
	// }
	// Manual comparison for conceptual stage:
	if config.GetVmId() != loadedConfig.GetVmId() ||
	   config.GetVmName() != loadedConfig.GetVmName() ||
	   config.GetArchitecture() != loadedConfig.GetArchitecture() {
		t.Errorf("Loaded config basic fields do not match saved config. Expected ID %s, Name %s, Arch %s. Got ID %s, Name %s, Arch %s",
			config.GetVmId(), config.GetVmName(), config.GetArchitecture(), loadedConfig.GetVmId(), loadedConfig.GetVmName(), loadedConfig.GetArchitecture())
	}
	if loadedConfig.GetVcpuConfig().GetCount() != config.GetVcpuConfig().GetCount() {
		t.Errorf("VCPU count mismatch: expected %d, got %d", config.GetVcpuConfig().GetCount(), loadedConfig.GetVcpuConfig().GetCount())
	}
	if loadedConfig.GetVramConfig().GetSizeMb() != config.GetVramConfig().GetSizeMb() {
		t.Errorf("VRAM size mismatch: expected %d, got %d", config.GetVramConfig().GetSizeMb(), loadedConfig.GetVramConfig().GetSizeMb())
	}


	fmt.Println("Conceptual Test: TestSaveAndLoadVMConfig PASSED (placeholder assertions used)")
}

func TestValidateVMConfigBusinessLogic(t *testing.T) {
	// require := require.New(t) // testify helper
	// assert := assert.New(t)   // testify helper

	// Test case 1: Valid config
	validConfig := newValidTestVMConfig()
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating a valid config.")
	err := ValidateVMConfigBusinessLogic(validConfig)
	// require.NoError(err, "Valid config should pass business logic validation")
	if err != nil {
		t.Errorf("ValidateVMConfigBusinessLogic failed for valid config: %v", err)
	}

	// Test case 2: Missing vm_id
	invalidConfig1 := newValidTestVMConfig()
	invalidConfig1.VmId = ""
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with missing vm_id.")
	err = ValidateVMConfigBusinessLogic(invalidConfig1)
	// require.Error(err, "Config with missing vm_id should fail validation")
	// assert.Contains(err.Error(), "vm_id is required")
	if err == nil {
		t.Errorf("Expected error for missing vm_id, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for missing vm_id: %v\n", err)
		if err.Error() != "vm_id is required" { // Example of more specific check
			// t.Errorf("Error message mismatch for missing vm_id. Got: %s", err.Error())
		}
	}

	// Test case 3: Missing vm_name
	invalidConfig2 := newValidTestVMConfig()
	invalidConfig2.VmName = ""
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with missing vm_name.")
	err = ValidateVMConfigBusinessLogic(invalidConfig2)
	if err == nil {
		t.Errorf("Expected error for missing vm_name, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for missing vm_name: %v\n", err)
	}

	// Test case 4: Missing architecture
	invalidConfig3 := newValidTestVMConfig()
	invalidConfig3.Architecture = ""
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with missing architecture.")
	err = ValidateVMConfigBusinessLogic(invalidConfig3)
	if err == nil {
		t.Errorf("Expected error for missing architecture, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for missing architecture: %v\n", err)
	}

	// Test case 5: Missing vCPU config
	invalidConfig4 := newValidTestVMConfig()
	invalidConfig4.VcpuConfig = nil
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with nil vcpu_config.")
	err = ValidateVMConfigBusinessLogic(invalidConfig4)
	if err == nil {
		t.Errorf("Expected error for nil vcpu_config, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for nil vcpu_config: %v\n", err)
	}

	// Test case 6: vCPU count zero
	invalidConfig5 := newValidTestVMConfig()
	invalidConfig5.VcpuConfig.Count = 0
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with vcpu_config.count = 0.")
	err = ValidateVMConfigBusinessLogic(invalidConfig5)
	if err == nil {
		t.Errorf("Expected error for vcpu_config.count = 0, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for vcpu_config.count = 0: %v\n", err)
	}

	// Test case 7: vRAM too small
	invalidConfig6 := newValidTestVMConfig()
	invalidConfig6.VramConfig.SizeMb = 64 // Assuming 128MB is min from ValidateVMConfigBusinessLogic
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with vram_config.size_mb too small.")
	err = ValidateVMConfigBusinessLogic(invalidConfig6)
	if err == nil {
		t.Errorf("Expected error for vram_config.size_mb too small, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for vram_config.size_mb too small: %v\n", err)
	}

	// Test case 8: Boot disk in boot_order not found
	invalidConfig7 := newValidTestVMConfig()
	invalidConfig7.BootOrder = []string{"non_existent_disk"}
	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic - Validating config with boot_order disk not in storage_devices.")
	err = ValidateVMConfigBusinessLogic(invalidConfig7)
	if err == nil {
		t.Errorf("Expected error for boot_order disk not found, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed for boot_order disk not found: %v\n", err)
	}

	fmt.Println("Conceptual Test: TestValidateVMConfigBusinessLogic checks completed.")
}

func TestLoadMalformedJSON(t *testing.T) {
	// require := require.New(t) // testify helper
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "malformed.json")

	malformedJSON := []byte(`{"vm_id": "test", "vm_name": "Bad JSON", "vcpu_config": { "count": "not_an_int" }`) // count is string, not uint32
	errWrite := os.WriteFile(filePath, malformedJSON, 0644)
	// require.NoError(errWrite)
	if errWrite != nil {
		t.Fatalf("Failed to write malformed JSON test file: %v", errWrite)
	}

	fmt.Printf("Conceptual Test: TestLoadMalformedJSON - Attempting to load malformed JSON from %s\n", filePath)
	_, errLoad := LoadVMConfigFromFile(filePath)
	// require.Error(errLoad, "Loading malformed JSON should produce an error")
	if errLoad == nil {
		t.Errorf("Expected error when loading malformed JSON, got nil")
	} else {
		fmt.Printf("Conceptual Test: Correctly failed to load malformed JSON: %v\n", errLoad)
		// Add specific error message check if using protojson, e.g.:
		// assert.Contains(t, errLoad.Error(), "cannot unmarshal string into Go value of type uint32")
	}

	fmt.Println("Conceptual Test: TestLoadMalformedJSON PASSED.")
}
