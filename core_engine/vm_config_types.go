package main

// This file contains Go struct definitions that would conceptually mirror
// the `vm_config_schema.json` and other necessary type definitions
// for managing VM configurations and state.

// VMConfig represents the configuration for a single virtual machine.
// This is a simplified version. A full version would be derived from
// the JSON schema and include many more fields for storage, networking,
// AI hardware, passthrough devices, etc.
type VMConfig struct {
	VMID              string            `json:"vm_id"`
	Name              string            `json:"name"`
	OSProfileID       string            `json:"os_profile_id"` // e.g., "ubuntu_22.04_x64"
	CPUCount          int               `json:"cpu_count"`
	VCPUConfig        VCPUConfig        `json:"vcpu_config"`
	MemoryMB          uint64            `json:"memory_mb"`
	RAMConfig         RAMConfig         `json:"ram_config"`
	StorageDevices    []StorageDevice   `json:"storage_devices"`
	NetworkInterfaces []NetworkInterface `json:"network_interfaces"`
	GraphicsConfig    GraphicsConfig    `json:"graphics_config"`
	BootOrder         []string          `json:"boot_order"` // e.g., ["disk0", "cdrom0"]

	// AI Related configurations (conceptual placeholders)
	AIWorkloadProfileTag string             `json:"ai_workload_profile_tag,omitempty"`
	AICPUConfig          *AICPUConfig       `json:"ai_cpu_config,omitempty"`
	AIGraphicsCardConfig *AIGraphicsCardConfig `json:"ai_graphics_card_config,omitempty"`
	// Add other AI hardware configs (AI RAM flags in RAMConfig, AI Switch/Router in NetworkInterface)

	// Security & Policy related (conceptual)
	SecurityPolicyID string `json:"security_policy_id,omitempty"`
	EnableVTPM       bool   `json:"enable_vtpm,omitempty"`

	// Other conceptual fields
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VCPUConfig defines vCPU specific settings.
type VCPUConfig struct {
	CoresPerSocket   int      `json:"cores_per_socket,omitempty"`
	ThreadsPerCore   int      `json:"threads_per_core,omitempty"`
	CPUFeaturesToAdd []string `json:"cpu_features_to_add,omitempty"`
	// Potentially CPU pinning info, NUMA config
}

// RAMConfig defines RAM specific settings.
type RAMConfig struct {
	EnableMemoryBallooning bool   `json:"enable_memory_ballooning,omitempty"`
	AIOptimizedFlags       AIOptimizedRAMFlags `json:"ai_optimized_flags,omitempty"`
	// NUMA node preferences could go here or be inferred
}

// AIOptimizedRAMFlags for AI RAM specific optimizations.
type AIOptimizedRAMFlags struct {
	PreferContiguous bool `json:"prefer_contiguous,omitempty"`
	NUMANodeAffinity *int `json:"numa_node_affinity,omitempty"` // Pointer to allow nil if not set
}

// StorageDevice defines a virtual storage device.
type StorageDevice struct {
	ID            string `json:"id"`             // e.g., "disk0", "cdrom0"
	ImagePath     string `json:"image_path,omitempty"` // Path to .qcow2, .vmdk, .iso
	Controller    string `json:"controller"`     // e.g., "virtio-blk", "nvme", "sata", "ide"
	SizeGB        uint64 `json:"size_gb,omitempty"`  // For new images
	IsBootDisk    bool   `json:"is_boot_disk,omitempty"`
	IsCDROM       bool   `json:"is_cdrom,omitempty"`
	ReadOnly      bool   `json:"read_only,omitempty"`
	Format        string `json:"format,omitempty"` // e.g., "qcow2", "raw"
	SnapshotBasedOn string `json:"snapshot_based_on,omitempty"` // ID of parent disk/snapshot
}

// NetworkInterface defines a virtual network interface.
type NetworkInterface struct {
	ID               string `json:"id"`                 // e.g., "net0"
	MACAddress       string `json:"mac_address,omitempty"` // Auto-generated if empty
	Model            string `json:"model"`              // e.g., "virtio-net", "e1000"
	NetworkName      string `json:"network_name"`       // Name of V-Architect network (vSwitch) to connect to
	IsAISwitchLinked bool   `json:"is_ai_switch_linked,omitempty"` // If connected to an AI Switch
	// IPConfiguration (static/dhcp) could go here
}

// GraphicsConfig defines virtual graphics adapter settings.
type GraphicsConfig struct {
	Type             string  `json:"type"`               // e.g., "vga", "virtio-gpu", "vgpu_passthrough"
	VRAMSizeMB       *uint32 `json:"vram_size_mb,omitempty"` // Optional, might be part of profile
	Resolution       string  `json:"resolution,omitempty"`   // e.g., "1920x1080"
	Headless         bool    `json:"headless,omitempty"`
	PassthroughGPUID string  `json:"passthrough_gpu_id,omitempty"` // PCI ID for passthrough
	VGPUMediatedProfile string `json:"vgpu_mediated_profile,omitempty"` // Vendor-specific profile
}

// AICPUConfig for virtual AI CPUs (vNPU/vTPU).
type AICPUConfig struct {
	Type                    string `json:"type"` // e.g., "passthrough", "mediated", "software_emulated"
	NumVirtualDevices       int    `json:"num_virtual_devices"`
	PassthroughDeviceID     string `json:"passthrough_device_id,omitempty"`
	MediatedProfileName     string `json:"mediated_profile_name,omitempty"`
	EmulatedPerformanceTier string `json:"emulated_performance_tier,omitempty"`
}

// AIGraphicsCardConfig for virtual AI-optimized GPUs/NPUs.
type AIGraphicsCardConfig struct {
	Type                  string `json:"type"` // e.g., "passthrough", "mediated_vgpu"
	PassthroughDeviceID   string `json:"passthrough_device_id,omitempty"`
	MediatedProfileID     string `json:"mediated_profile_id,omitempty"` // Vendor-specific profile for AI
	DedicatedMemoryMB     uint64 `json:"dedicated_memory_mb,omitempty"`
}

// MemoryRegionInfo could be used by VMManager to track memory mappings.
// type MemoryRegionInfo struct {
// 	Slot          uint32
// 	GuestPhysAddr uint64
// 	MemorySize    uint64
// 	HostUserAddr  uintptr // Mapped address in Core Engine process
// 	Flags         uint32  // e.g., ReadOnly
// }

// VCPUState might be used later for more detailed vCPU management.
// type VCPUState struct {
//  ID     int
//  Thread *os.Thread // Or goroutine ID
//  KVMRunPtr uintptr // Pointer to KVM_RUN structure
//  Status string    // e.g., "RUNNING", "WAITING_IO"
// }

// This file provides the basic Go structs. In a real system, these might be
// auto-generated from Protobuf definitions or a more formal schema language
// to ensure consistency between gRPC services and internal representations.
