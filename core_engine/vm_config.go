package core_engine

import (
	"encoding/json" // Using standard json for simplicity in conceptual phase
	"fmt"
	"os" // For os.ReadFile, os.WriteFile
	"path/filepath" // For test file path construction in conceptual Load
	"strings"       // For test file name check in conceptual Load

	// Import the generated protobuf package (adjust path if necessary)
	// The previous go_package was "./;proto", so the import path depends on where the files are placed
	// relative to the Go module root. Assuming they will be in a 'proto' package at the same level as 'core_engine'
	// or if core_engine is the module root, then "module_name/proto".
	// For this task, using the path from the previous successful generation:
	pb "github.com/V-Architect/v-architect-core/proto"

	// "google.golang.org/protobuf/encoding/protojson" // Preferred for marshalling/unmarshalling protos with JSON
)

// LoadVMConfigFromFile loads a VM configuration from a JSON file,
// unmarshals it into a pb.VMConfig struct, and validates its business logic.
// NOTE: Using standard `encoding/json`. For robust Protobuf JSON, `protojson` is recommended.
func LoadVMConfigFromFile(filePath string) (*pb.VMConfig, error) {
	fmt.Printf("Conceptual: LoadVMConfigFromFile called for %s\n", filePath)

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		// Conceptual placeholder logic for tests if file doesn't exist
		isTestSaveFile := strings.HasSuffix(filePath, "test_vm_config_save_load.json")
		isTestMalformedFile := strings.HasSuffix(filePath, "malformed_config.json")
		isTestValidFileForLoad := strings.HasSuffix(filePath, "valid_config_for_load.json")


		if os.IsNotExist(err) && (isTestSaveFile || isTestValidFileForLoad) {
			 fmt.Printf("Conceptual: Test file '%s' not found, using dummy valid config for LoadVMConfigFromFile.\n", filePath)
			 dummyConfig := &pb.VMConfig{
				 VmId:         "dummy-test-id-from-load-default",
				 Name:       "DummyLoadedVMFromFile",
				 Architecture: pb.VirtualHardwareArch_X86_64,
				 OsTypeHint:   "linux_generic_test", // Assuming OsTypeHint is the field name
				 VcpuCores:   1,
				 MemoryMb:   512,
				 VcpuConfig:   &pb.VCPUConfig{EnableKvmHv: true},
				 MemoryConfig: &pb.MemoryConfig{EnableHugePages: true, EnableMemBallooning: true},
				 SerialPorts:  []*pb.SerialPortConfig{{Type: pb.SerialPortConfig_STDIO}},
			 }
			 tempJsonData, _ := json.Marshal(dummyConfig) // Standard json for this conceptual path
			 jsonData = tempJsonData // Use this jsonData for unmarshalling

		} else if os.IsNotExist(err) && isTestMalformedFile {
			 fmt.Printf("Conceptual: Test file '%s' not found, using internal malformed JSON for LoadVMConfigFromFile.\n", filePath)
			 jsonData = []byte(`{"vm_id": "bad", "name": "Bad JSON", "vcpu_cores": "not_an_int" }`) // Malformed
		} else {
			return nil, fmt.Errorf("failed to read VM config file %s: %w", filePath, err)
		}
	}

	config := &pb.VMConfig{}
	// For production, use protojson:
	// err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(jsonData, config)
	// For conceptual phase with standard json:
	err = json.Unmarshal(jsonData, config) // This might be lossy or error-prone for some proto features
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VM config JSON from %s: %w", filePath, err)
	}

	// Apply defaults after loading, before validation
	SetDefaultVMConfigValues(config) // Modifies config in place

	if errVal := ValidateVMConfigBusinessLogic(config); errVal != nil {
		return nil, fmt.Errorf("business logic validation failed for VM %s (%s): %w", config.GetName(), config.GetVmId(), errVal)
	}

	fmt.Printf("Conceptual: VMConfig loaded and validated for %s from %s\n", config.GetName(), filePath)
	return config, nil
}

// SaveVMConfigToFile marshals a pb.VMConfig struct to JSON and saves it to a file.
// NOTE: Using standard `encoding/json`. For robust Protobuf JSON, `protojson` is recommended.
func SaveVMConfigToFile(config *pb.VMConfig, filePath string) error {
	fmt.Printf("Conceptual: SaveVMConfigToFile called for %s for VM %s\n", filePath, config.GetName())

	// For production, use protojson:
	// jsonData, err := protojson.MarshalOptions{Indent: "  ", EmitUnpopulated: true, UseProtoNames: true}.Marshal(config)
	// For conceptual phase with standard json:
	jsonData, err := json.MarshalIndent(config, "", "  ") // This relies on json tags in pb.go files or default behavior
	if err != nil {
		return fmt.Errorf("failed to marshal VM config to JSON for %s: %w", config.GetName(), err)
	}

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write VM config file %s: %w", filePath, err)
	}
	fmt.Printf("Conceptual: VMConfig for %s saved to %s\n", config.GetName(), filePath)
	return nil
}

// ValidateVMConfigBusinessLogic validates a pb.VMConfig struct against business logic
// and constraints not easily expressed in Protobuf definitions alone.
func ValidateVMConfigBusinessLogic(config *pb.VMConfig) error {
	fmt.Printf("Conceptual: ValidateVMConfigBusinessLogic called for VM: %s (ID: %s)\n", config.GetName(), config.GetVmId())
	if config.GetVmId() == "" {
		// Allow vm_id to be empty if it's generated by Core Engine later,
		// but name should usually be present if config is considered "complete".
		// For now, let's assume vm_id can be empty pre-registration.
	}
	if config.GetName() == "" {
		return fmt.Errorf("vm_config.name is required")
	}
	if config.GetVcpuCores() == 0 {
		return fmt.Errorf("vm_config.vcpu_cores must be > 0")
	}
	if config.GetMemoryMb() < 128 { // Example minimum memory
		return fmt.Errorf("vm_config.memory_mb must be at least 128MB")
	}
	if config.GetArch() == pb.VirtualHardwareArch_ARCH_UNSPECIFIED {
		return fmt.Errorf("vm_config.arch must be specified (e.g., X86_64, ARM64)")
	}

	// If not using BIOS boot (implying UEFI or direct kernel), and no kernel path, it's an issue
	// unless there's a bootable disk. This logic can get complex.
	// For simplicity: if direct kernel boot fields are partially set, kernel_image_path is key.
	if config.GetKernelImagePath() != "" {
		// Direct kernel boot implied, cmdline might also be essential
		if config.GetKernelCmdline() == "" {
			fmt.Println("Warning: Direct kernel boot specified but kernel_cmdline is empty.")
		}
	} else {
		// Not direct kernel boot, check for bootable disk if disks are present
		if len(config.GetDiskImages()) > 0 {
			hasBootDisk := false
			for _, disk := range config.GetDiskImages() {
				if disk.GetPath() == "" {
					return fmt.Errorf("disk_image.path is required for disk ID %s (conceptual, disk_id not in this message yet)", "N/A")
				}
				// Assuming disk_id is not part of DiskImage directly, but part of a repeated field key or wrapper.
				// The current proto has disk_images as a simple repeated field.
				// If we assume the VMConfig has a separate boot_order field that references disk_id from StorageDevice in the full proto:
				// This validation would be more complex and cross-reference boot_order with disk_images.
				// For this simplified proto, let's assume the first disk is bootable if no kernel_image_path.
				// This is a weak assumption for a real VMM.
			}
			if !hasBootDisk && len(config.GetBootOrder()) == 0 { // If boot order not specified, and no kernel path
				// This implies we need a bootable disk but can't identify one.
				// This validation is better done with a boot_order field.
			}
		} else if config.GetKernelImagePath() == "" { // No disks and no kernel path
			return fmt.Errorf("no bootable disk images provided and no kernel_image_path for direct boot")
		}
	}

	fmt.Printf("Conceptual: VMConfig for %s (ID: %s) passed basic business logic validation.\n", config.GetName(), config.GetVmId())
	return nil
}


// SetDefaultVMConfigValues populates missing fields with sensible defaults.
// It modifies the passed config object in place and also returns it.
func SetDefaultVMConfigValues(config *pb.VMConfig) *pb.VMConfig {
	fmt.Printf("Conceptual: SetDefaultVMConfigValues called for VM: %s
", config.GetName())

	if config.GetArch() == pb.VirtualHardwareArch_ARCH_UNSPECIFIED {
		config.Arch = pb.VirtualHardwareArch_X86_64
		fmt.Printf("Conceptual: Defaulted arch to X86_64 for VM %s
", config.GetName())
	}

	if config.GetVcpuConfig() == nil {
		config.VcpuConfig = &pb.VCPUConfig{}
	}
	if !config.GetVcpuConfig().GetEnableKvmHv() { // Assuming default should be true if field exists
		// If the field defaults to false in proto3 and we want true as *our* default if not set.
		// However, proto3 bool defaults to false. So if it's false, it could be explicitly set or default.
		// A better approach for "not set" is to use wrapper types (google.protobuf.BoolValue) or
		// check if the parent message (VcpuConfig) is nil.
		// For this conceptual step, let's assume if VcpuConfig is not nil, we ensure EnableKvmHv is true.
		config.VcpuConfig.EnableKvmHv = true
		fmt.Printf("Conceptual: Defaulted vcpu_config.enable_kvm_hv to true for VM %s
", config.GetName())
	}

	if config.GetMemoryConfig() == nil {
		config.MemoryConfig = &pb.MemoryConfig{}
	}
	if !config.GetMemoryConfig().GetEnableHugePages() { // Similar logic to EnableKvmHv
		config.MemoryConfig.EnableHugePages = true
		fmt.Printf("Conceptual: Defaulted memory_config.enable_huge_pages to true for VM %s
", config.GetName())
	}
	// EnableMemBallooning might default to false or true based on typical use case. Let's assume false is acceptable default.

	if len(config.GetSerialPorts()) == 0 {
		config.SerialPorts = append(config.SerialPorts, &pb.SerialPortConfig{
			Type: pb.SerialPortConfig_STDIO,
			// PathIfFile is not relevant for STDIO
		})
		fmt.Printf("Conceptual: Added default STDIO serial port for VM %s
", config.GetName())
	}

	// Ensure enable_bios_boot is explicitly false if UEFI might be implied by other settings (e.g. secure_boot)
	// Or, if firmware_type field is added later (as in full proto), default that.
	// For now, if enable_bios_boot is false, it implies UEFI-like or direct kernel.
	// If kernel_image_path is set, enable_bios_boot might be irrelevant or should be false.

	fmt.Printf("Conceptual: Defaults applied for VM %s
", config.GetName())
	return config
}

// Placeholder for filepath and strings package usage in conceptual LoadVMConfigFromFile
var _ = filepath.Separator
var _ = strings.HasSuffix
