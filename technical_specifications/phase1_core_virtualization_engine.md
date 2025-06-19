# Phase 1: Core Virtualization Engine - Detailed Technical Specifications

## Introduction

This document provides the detailed technical specifications for Phase 1: Core Virtualization Engine of the V-Architect project. It translates the conceptual designs outlined in `conceptual_designs/v_architect_conceptual_blueprint.md` (specifically "Phase 1: Core Virtualization Engine - Sculpting AI-Native Digital Hardware") into actionable technical details intended to guide software architects and development teams.

The goal of this specification is to define the precise APIs, data structures, module interactions, and key technological choices necessary to implement the foundational hypervisor and virtual hardware emulation layers of V-Architect. This includes the core engine, VM configuration, standard virtual hardware components (vCPU, vGPU, vRAM, virtual storage, virtual networking), AI-accelerated virtual hardware, and mechanisms for performance optimization and dynamic scaling.

## II. Hypervisor Architecture & Core VM Management - Technical Specifications

This section details the technical specifications for the hypervisor's architecture, focusing on the Core Engine, the Desktop Orchestration Layer, their APIs, and the management of Virtual Machine (VM) configurations.

### A. Hypervisor Core Engine APIs

The Core Engine will expose a gRPC API for robust, high-performance, and cross-language communication with management layers like the Desktop Orchestration Layer or other potential clients (e.g., CLI tools, web interfaces).

**Service Definition (Conceptual Protocol Buffer IDL):**

```protobuf
// Service for managing the Hypervisor Core Engine itself
service CoreHypervisorService {
  // Get version and capabilities of the Core Engine
  rpc GetEngineInfo(GetEngineInfoRequest) returns (GetEngineInfoResponse);
  // Perform health check
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  // Configure global Core Engine settings (e.g., logging, default paths)
  rpc ConfigureEngine(ConfigureEngineRequest) returns (ConfigureEngineResponse);
  // Get current host capabilities (CPUs, memory, supported features, detected AI accelerators)
  rpc GetHostCapabilities(GetHostCapabilitiesRequest) returns (GetHostCapabilitiesResponse);
}

// Service for managing Virtual Machines
service VMService {
  // Create a new Virtual Machine based on a VMConfig
  rpc CreateVM(CreateVMRequest) returns (CreateVMResponse);
  // Delete an existing Virtual Machine
  rpc DeleteVM(DeleteVMRequest) returns (DeleteVMResponse);
  // Start a Virtual Machine
  rpc StartVM(StartVMRequest) returns (StartVMResponse);
  // Stop a Virtual Machine (graceful shutdown or force power-off)
  rpc StopVM(StopVMRequest) returns (StopVMResponse);
  // Pause a Virtual Machine
  rpc PauseVM(PauseVMRequest) returns (PauseVMResponse);
  // Resume a paused Virtual Machine
  rpc ResumeVM(ResumeVMRequest) returns (ResumeVMResponse);
  // Get the current state/status of a Virtual Machine
  rpc GetVMStatus(GetVMStatusRequest) returns (GetVMStatusResponse);
  // Update a Virtual Machine's configuration (subset of VMConfig, e.g., for hot-plug)
  rpc UpdateVMConfiguration(UpdateVMConfigurationRequest) returns (UpdateVMConfigurationResponse);
  // List all Virtual Machines managed by this Core Engine
  rpc ListVMs(ListVMsRequest) returns (ListVMsResponse);
  // Attach or detach installation media (ISO)
  rpc ManageVMMedia(ManageVMMediaRequest) returns (ManageVMMediaResponse);
}
```

**Key Data Structures (Conceptual Protocol Buffer Messages):**

*   `VMConfig`: (To be fully defined by `vm_config_schema.json` as per step III of this plan, but referenced here). This will be a comprehensive structure including vCPU, vRAM, storage, network, graphics, and AI hardware configurations.
*   `GetEngineInfoResponse`: Contains fields like `engine_version` (string), `api_version` (string), `supported_vm_architectures` (list of strings like "x86-64", "arm64").
*   `HealthCheckResponse`: Contains `status` (enum: HEALTHY, DEGRADED, UNHEALTHY), `message` (string).
*   `GetHostCapabilitiesResponse`:
    *   `cpu_info`: (model string, core_count, thread_count, supported_virtualization_extensions list)
    *   `total_memory_gb`: (uint64)
    *   `available_memory_gb`: (uint64)
    *   `storage_pools`: (list of structures detailing available storage paths/pools)
    *   `detected_gpus`: (list of structures with GPU model, memory, passthrough_capable boolean)
    *   `detected_ai_accelerators`: (list of structures with accelerator type, model, capabilities)
*   `CreateVMRequest`: Contains `vm_id` (string, unique), `vm_config` (VMConfig message).
*   `CreateVMResponse`: Contains `vm_id` (string), `status` (enum: SUCCESS, FAILED), `message` (string).
*   (Similar request/response messages for other `VMService` RPCs, typically including `vm_id` for identification and status/message fields for results).

**Error Handling:**
Standard gRPC status codes will be used. Detailed error messages will be provided in the response body where applicable (e.g., `INVALID_ARGUMENT` if `VMConfig` is malformed, `NOT_FOUND` if `vm_id` does not exist, `RESOURCE_EXHAUSTED` if host lacks resources for VM creation/start).

### B. Desktop Orchestration Layer to Core Engine Communication

*   **Protocol:** gRPC over a local Unix domain socket (Linux/macOS) or named pipe (Windows) for secure and efficient local communication. Fallback to TCP/IP on a localhost port if local IPC is problematic.
*   **Message Formats:** As defined by the Protocol Buffer messages for the `CoreHypervisorService` and `VMService`.
*   **Service Discovery:** The Desktop Orchestration Layer will be responsible for locating and connecting to the Core Engine's gRPC server. This might involve a well-known socket path or a simple configuration file.

### C. Host Capabilities Detection (Core Engine Responsibility)

The `GetHostCapabilities` RPC in the `CoreHypervisorService` will be responsible for dynamically detecting and reporting the host system's capabilities.

*   **CPU Information:**
    *   Use libraries like `libcpuid` or platform-specific APIs (e.g., `/proc/cpuinfo` on Linux, `sysctl` on macOS, Windows Registry/WMI) to gather CPU model, core/thread counts, and supported virtualization extensions (VT-x/SVM flags, EPT/NPT, VT-d/IOMMU).
*   **Memory Information:**
    *   Use OS APIs to get total and available physical memory (e.g., `/proc/meminfo` on Linux, `sysctl` on macOS, `GlobalMemoryStatusEx` on Windows).
*   **Storage Pools:**
    *   Initially, this might be based on pre-configured paths in the Core Engine's settings. Future enhancements could involve discovering available storage devices and their capacities.
*   **GPU Detection:**
    *   Use tools like `lspci` (Linux), System Profiler (macOS), or Windows Device Manager APIs to list PCI devices and identify GPUs. Attempt to determine model, VRAM, and IOMMU group for passthrough capability.
*   **AI Accelerator Detection:**
    *   Similar to GPU detection, using PCI device listing. Identification will rely on known vendor/device IDs for common AI accelerators (TPUs, NPUs, specialized ASICs). Specific capabilities might require vendor-provided libraries or tools.

This section provides a high-level API definition. Detailed message structures for each RPC will be further elaborated as individual modules are specified.

### D. Virtual Machine (VM) Configuration Schema

This section defines the technical specification for the Virtual Machine (VM) configuration data structure, which was conceptualized in Phase 1B of the V-Architect Conceptual Blueprint. The definitive schema will be maintained as a separate JSON Schema file (e.g., `vm_config_schema.json`) or a Protocol Buffer definition, versioned alongside the V-Architect Core Engine.

**Schema Language:** JSON Schema (Draft 7 or later recommended for its maturity and feature set).

**Versioning Strategy:** Semantic versioning (e.g., `v1.0.0`, `v1.1.0`). The Core Engine API (`VMService.CreateVM`, `VMService.UpdateVMConfiguration`) will indicate the schema version it expects or supports. Mechanisms for backward compatibility (e.g., transforming older schema versions) should be considered for future updates.

**Root Object:** The root of the schema will define an object representing a single VM configuration.

**Key Fields and Data Types (Illustrative Examples):**

*(Note: This is not the exhaustive schema, but illustrative of the level of detail required. The full schema will be a separate artifact.)*

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "V-Architect VM Configuration",
  "description": "Schema for V-Architect Virtual Machine configurations",
  "type": "object",
  "version": "1.0.0",
  "required": [
    "vm_id",
    "vm_name",
    "architecture",
    "vcpu_config",
    "vram_config",
    "storage_devices",
    "network_interfaces"
  ],
  "properties": {
    "vm_id": {
      "description": "Unique identifier for the VM (e.g., UUID).",
      "type": "string",
      "pattern": "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
    },
    "vm_name": {
      "description": "User-defined name for the VM.",
      "type": "string",
      "minLength": 1,
      "maxLength": 128
    },
    "description": {
      "description": "Optional longer description for the VM.",
      "type": "string"
    },
    "os_type": {
      "description": "Operating system type identifier (e.g., 'windows_11_x64', 'ubuntu_22.04_arm64'). Helps V-Architect suggest defaults or apply OS-specific optimizations.",
      "type": "string",
      "examples": ["windows_11_x64", "ubuntu_server_22.04_x64", "macos_13_arm64", "chromeos_flex_x64"]
    },
    "architecture": {
      "description": "Target CPU architecture for the VM.",
      "type": "string",
      "enum": ["x86-64", "arm64"]
    },
    "vcpu_config": {
      "type": "object",
      "properties": {
        "count": { "type": "integer", "minimum": 1, "default": 1 },
        "topology": {
          "type": "object",
          "properties": {
            "sockets": { "type": "integer", "minimum": 1, "default": 1 },
            "cores_per_socket": { "type": "integer", "minimum": 1, "default": 1 },
            "threads_per_core": { "type": "integer", "minimum": 1, "default": 1 }
          },
          "required": ["sockets", "cores_per_socket", "threads_per_core"]
        },
        "cpu_features_passthrough": {
          "description": "List of specific CPU features to attempt to passthrough or enable. Exact feature strings are host and hypervisor dependent.",
          "type": "array",
          "items": { "type": "string" },
          "examples": [ "avx", "avx2", "vmx" ]
        }
      },
      "required": ["count", "topology"]
    },
    "vram_config": {
      "type": "object",
      "properties": {
        "size_mb": { "type": "integer", "minimum": 128, "description": "RAM size in Megabytes." },
        "memory_ballooning_enabled": { "type": "boolean", "default": true },
        "ai_optimized_flags": {
          "description": "Flags for AI RAM optimizations (conceptual).",
          "type": "object",
          "properties": {
            "prefer_contiguous": { "type": "boolean", "default": false },
            "numa_node_affinity": { "type": ["integer", "null"], "description": "Preferred host NUMA node if applicable." }
          }
        }
      },
      "required": ["size_mb"]
    },
    "storage_devices": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "disk_id": { "type": "string", "description": "Unique identifier for this disk within the VM." },
          "image_path": { "type": "string", "description": "Path to the virtual disk image file on the host or a URI for managed storage." },
          "controller_type": { "type": "string", "enum": ["virtio-blk", "virtio-scsi", "nvme", "sata", "ide"] },
          "size_gb": { "type": "integer", "minimum": 1, "description": "Size in Gigabytes (used if creating a new disk)." },
          "is_boot_disk": { "type": "boolean", "default": false },
          "read_only": { "type": "boolean", "default": false },
          "format": { "type": "string", "enum": ["qcow2", "raw", "vmdk"], "default": "qcow2" }
        },
        "required": ["disk_id", "image_path", "controller_type"]
      }
    },
    "network_interfaces": {
      "type": "array",
      "minItems": 1,
      "items": {
        // Refer to Phase 1E for vNIC, vSwitch, vRouter details
        // This section will include vNIC model, MAC address, network attachment type (bridged, NAT, specific vSwitch), etc.
      }
    },
    "graphics_config": {
      // Refer to Phase 1B for vGPU details
      // This will include type (VGA, virtio-gpu, passthrough, mediated), profile, physical device ID if passthrough, etc.
    },
    // ... Other sections for USB/PCIe passthrough ...
    "ai_accelerated_hardware": {
      "description": "Configuration for AI-specific virtual hardware.",
      "type": "object",
      "properties": {
        "ai_cpu_config": { /* Detailed spec for vNPU/vTPU */ },
        "ai_graphics_card_config": { /* Detailed spec for vAI-GPU */ }
        // AI RAM config is part of vram_config.ai_optimized_flags
        // AI Switches/Routers are part of network_interfaces attachment
      }
    },
    "boot_order": {
      "type": "array",
      "items": { "type": "string", "description": "Ordered list of device IDs to attempt booting from (e.g., disk_id, nic_id)." }
    }
    // ... other VM settings like BIOS/UEFI, secure boot toggles ...
  }
}
```

**Validation:**
The Core Engine API (`VMService.CreateVM` and `VMService.UpdateVMConfiguration`) MUST validate any incoming VM configuration against this schema before proceeding. Rejection with clear error messages for schema violations is required.

**Extensibility:**
The schema should allow for custom, non-standard fields using the `additionalProperties` keyword if necessary for specific hypervisor extensions, but core functionality should rely on the defined properties.

**Full Schema Document:**
The complete `vm_config_schema.json` (or its Protobuf equivalent) will be maintained as a separate artifact in the V-Architect repository under a `schemas/` directory and will be the single source of truth for VM configuration structure. It will include detailed descriptions and validation rules for all fields conceptualized in Phase 1 of the blueprint.

## III. Virtual Hardware Emulation Modules - Technical Specifications & Initial Implementation Planning

This major section translates the conceptual designs for each virtual hardware module from Phase 1 of the V-Architect Conceptual Blueprint into detailed technical specifications and initial implementation considerations.

### A. Virtual CPU (vCPU) Emulation - Technical Specifications

This subsection details the technical specifications for vCPU emulation within V-Architect, focusing on supported architectures, virtualization techniques, and state management.

**1. Supported CPU Architectures:**
    *   **Primary Target:** `x86-64` (Intel VT-x, AMD-V). This is the highest priority due to its widespread use in desktop and server environments.
    *   **Secondary Target:** `ARM64` (ARMv8-A and later with Virtualization Extensions). Growing importance, especially in mobile, embedded, and increasingly, server environments.
    *   **Cross-Architecture Emulation:** Not planned for initial V-Architect Core Engine implementation due to significant performance overhead and complexity. If considered in the far future, it would likely involve integrating a full system emulator like QEMU with TCG (Tiny Code Generator) in a specific operational mode.

**2. Hardware-Assisted Virtualization (HV) Integration:**
    *   **x86-64:**
        *   **Intel VT-x:**
            *   VMCS (Virtual Machine Control Structure) configuration and management for each vCPU.
            *   EPT (Extended Page Tables) for MMU virtualization (detailed in vRAM section).
            *   VPID (Virtual Processor Identifier) for reducing TLB flush overhead.
            *   VM-entry/VM-exit handling: Minimizing exit frequency by handling common instructions in guest mode where possible (e.g., through specific CPU feature flags or enlightened interfaces).
            *   Interception of privileged instructions (e.g., `HLT`, `IN`/`OUT`, `CPUID`, MSR reads/writes) and specific events (e.g., exceptions, interrupts).
        *   **AMD-V (SVM):**
            *   VMCB (Virtual Machine Control Block) configuration and management.
            *   NPT (Nested Page Tables) / RVI (Rapid Virtualization Indexing) for MMU virtualization.
            *   ASID (Address Space Identifier) for reducing TLB flush overhead.
            *   `#VMEXIT` handling, similar to Intel VT-x.
    *   **ARM64:**
        *   EL2 (Hypervisor Execution Level) utilization.
        *   Stage 2 MMU translation for guest physical addresses.
        *   Virtual GIC (Generic Interrupt Controller) configuration (e.g., GICv2, GICv3) for interrupt handling.
        *   Interception of privileged operations and system register accesses.
    *   **HV API Interaction:**
        *   **Linux Host:** Primarily via KVM ioctls (e.g., `KVM_CREATE_VM`, `KVM_CREATE_VCPU`, `KVM_RUN`, `KVM_SET_REGS`, `KVM_GET_SREGS`).
        *   **Windows Host (Conceptual for Desktop Orchestration Layer bridging):** Windows Hypervisor Platform (WHP) APIs (e.g., `WHvCreateVirtualProcessor`, `WHvRunVirtualProcessor`, `WHvGetVirtualProcessorRegisters`, `WHvSetVirtualProcessorRegisters`).

**3. Instruction Emulation (Minimal, for specific cases):**
    *   The primary approach is direct execution via HV.
    *   Emulation (e.g., by integrating components of QEMU's instruction set emulator or a custom solution) will be strictly limited to:
        *   Specific I/O instructions if not handled by device models directly.
        *   Certain MSRs or CPUID leaves that need specific values returned to the guest and are not fully handled by HV pass-through or underlying KVM/WHP.
        *   Handling specific unprivileged instructions known to cause VM exits if not handled by the guest or if specific behavior is required (rare).
    *   No full unprivileged instruction set emulation is planned for same-architecture virtualization in the Core Engine.

**4. vCPU State Management:**
    *   **Data Structure (`vCPU_State` - internal Core Engine representation):**
        *   General Purpose Registers (e.g., RAX, RBX, etc. for x86-64; X0-X30, SP, PC, PSTATE for ARM64).
        *   Segment Registers (x86-64): CS, DS, ES, FS, GS, SS, LDTR, TR, GDTR, IDTR.
        *   Control Registers (x86-64): CR0, CR2, CR3, CR4, CR8. Debug Registers (DR0-DR7).
        *   Model-Specific Registers (MSRs): A subset of critical MSRs will be managed/virtualized (e.g., `EFER`, `STAR`, `LSTAR`, `SYSCALL_MASK` for x86-64). Others might be passed through or denied.
        *   Floating Point Unit (FPU/SSE/AVX) state: (e.g., XMM registers, MXCSR for x86-64; FP/SIMD registers for ARM64).
        *   Interruptibility state.
        *   Activity State (e.g., running, HLT, shutdown).
        *   Pending exceptions and interrupts.
    *   **API for State Save/Restore:** KVM/WHP provide ioctls/APIs for saving and restoring much of this state (e.g., `KVM_SET_REGS`, `KVM_GET_SREGS`, `KVM_SET_FPU`, `KVM_GET_FPU`, `WHvSetVirtualProcessorRegisters`, `WHvGetVirtualProcessorRegisters`). V-Architect will wrap these for its internal needs (e.g., for snapshots, live migration).

**5. vCPU Scheduling Interface (Internal to Core Engine):**
    *   The Core Engine will implement a scheduler that maps vCPUs to physical host threads (pthreads or Windows threads).
    *   The scheduler will consider VM priorities, vCPU weights/shares (defined in `VMConfig`), and potentially host CPU load.
    *   APIs within the Core Engine will allow the VM management layer to set these priorities/weights.
    *   **Gemini Integration Point:** Gemini's dynamic vCPU scheduling optimization (conceptualized in Blueprint Phase 1) would provide hints or directives to this internal scheduler.

**Initial Implementation Considerations:**
*   Start with x86-64 support leveraging KVM on Linux as the primary development target for the Core Engine.
*   Focus on robust VMCS/VMCB setup for basic boot of a simple guest OS.
*   Implement minimal MSR and CPUID virtualization required for common OSs.
*   Gradually add support for more complex CPU features and ARM64.

### B. Virtual GPU (vGPU) & Graphics Acceleration - Technical Specifications

This subsection provides the technical specifications for virtual GPU (vGPU) and graphics acceleration capabilities within V-Architect, enabling VMs to handle tasks ranging from basic display output to intensive 3D rendering and GPU-accelerated computation.

**1. Basic Emulated Graphics (Compatibility Mode):**
    *   **Device Model:** Standard VGA compatible graphics card (e.g., `stdvga` model commonly found in QEMU, or Bochs VBE extensions).
    *   **Resolution Support:** Support for common VESA resolutions (e.g., 800x600, 1024x768, up to 1920x1200). Higher resolutions might depend on guest drivers for the emulated device.
    *   **Color Depth:** Support for 16-bit and 24/32-bit color depths.
    *   **Framebuffer Access:** The hypervisor will manage a virtual framebuffer for this device in host memory. The V-Architect display client (part of Desktop Orchestration Layer or remote access solution) will read from this framebuffer to render the VM's display.
    *   **Implementation:** Leverage existing VGA emulation code from QEMU or similar open-source projects.

**2. GPU Passthrough (Direct Device Assignment - DDA):**
    *   **Host Prerequisite:** Host hardware must support IOMMU (Intel VT-d or AMD-Vi) and it must be enabled in the BIOS/UEFI. The target GPU must be in a compatible IOMMU group.
    *   **Mechanism:**
        *   The Core Engine will identify passthrough-capable GPUs on the host (using `lspci` and IOMMU group information on Linux; platform-specific APIs on other hosts).
        *   The host GPU driver for the selected physical GPU must be unbound (e.g., bound to `vfio-pci` on Linux). V-Architect might provide tools or scripts to assist with this.
        *   The physical PCIe device corresponding to the GPU is assigned directly to the VM. The guest OS then loads its native drivers for that GPU.
    *   **ROM/Firmware Handling:** The GPU's option ROM (firmware) may need to be made available to the guest. V-Architect will need a mechanism to specify a ROM file or extract it from the host if necessary and permissible.
    *   **Supported GPUs:** Theoretically any PCIe GPU compatible with host IOMMU and for which the guest OS has drivers. Practical limitations may apply based on specific GPU models and motherboard compatibility.
    *   **Audio on GPU:** If the GPU has an integrated HDMI/DisplayPort audio device, this sub-device should also be passed through with the GPU.
    *   **API:** `VMService.UpdateVMConfiguration` or a dedicated RPC in `VMService` will be used to assign/unassign a host GPU to/from a VM (requires VM to be powered off for changes). `GetHostCapabilitiesResponse` will list GPUs available for passthrough.

**3. Mediated Passthrough (vGPU Profiles - Vendor Dependent):**
    *   **Rationale:** Allows sharing of a single physical GPU among multiple VMs, offering better density than DDA but with some performance overhead and reliance on vendor solutions.
    *   **Target Vendor Technologies (Initial Consideration):**
        *   **NVIDIA vGPU (GRID/Tesla/Quadro):** Requires NVIDIA GPUs that support vGPU, compatible NVIDIA host drivers (which include the vGPU manager), and licensed NVIDIA vGPU software. Guest VMs use specific NVIDIA vGPU guest drivers.
            *   **Integration:** V-Architect's Core Engine (or a management daemon on the host) would interact with the NVIDIA vGPU manager (e.g., `nvidia-vgpu-mgr` service or APIs if available) to create, assign, and manage vGPU instances (profiles like `A100-8Q`, `T4-4C`).
        *   **AMD MxGPU (Radeon Pro):** Requires AMD GPUs supporting MxGPU and AMD host drivers. Guest VMs use standard AMD Radeon Pro drivers.
            *   **Integration:** Interaction with AMD's host drivers/APIs for managing SR-IOV virtual functions (VFs) representing vGPU instances.
        *   **Intel GVT-g (Integrated Graphics):** Allows sharing of Intel integrated GPUs among VMs.
            *   **Integration:** Interaction with Linux KVM/VFIO mechanisms for Intel GVT-g (e.g., via sysfs).
    *   **vGPU Profile Management:** The `VMConfig` schema will include fields to specify the desired vGPU profile (e.g., `vendor: "nvidia"`, `profile_name: "GRID A40-8Q"`). `GetHostCapabilitiesResponse` will list available physical GPUs and supported vGPU profiles they can host.
    *   **API:** `VMService.UpdateVMConfiguration` for assigning vGPU profiles (VM power-off likely required).
    *   **Licensing:** V-Architect must clearly indicate to the user any licensing requirements imposed by GPU vendors for vGPU solutions. V-Architect itself will not bundle these licensed components.

**4. Paravirtualized Graphics (e.g., VirtIO-GPU):**
    *   **Device Model:** `virtio-gpu`.
    *   **Features:**
        *   2D acceleration.
        *   Cursor acceleration.
        *   Multi-head support (multiple virtual monitors).
        *   Conceptual support for 3D acceleration via Vulkan (leveraging the Venus project on Linux hosts) or DirectX (on Windows hosts with appropriate host-side support). This provides a more standardized way for 3D acceleration without full passthrough.
    *   **Guest Driver:** Requires VirtIO graphics drivers in the guest OS.
    *   **Backend Implementation:**
        *   **Linux Host:** The V-Architect Core Engine can leverage KVM's VirtIO-GPU backend, which can use host-side rendering (e.g., virglrenderer for OpenGL, or Venus for Vulkan pass-through to the host GPU).
        *   **Windows Host (Conceptual):** May require a custom VirtIO-GPU backend implementation or integration with existing Windows VirtIO driver frameworks if WHP is used.
    *   **API:** `VMConfig` will specify `virtio-gpu` as the graphics type.

**Initial Implementation Considerations:**
*   Start with robust basic emulated graphics (`stdvga`).
*   Implement GPU Passthrough (DDA) next, focusing on Linux hosts with `vfio-pci` initially. Test with common NVIDIA and AMD discrete GPUs.
*   Investigate VirtIO-GPU with virglrenderer for OpenGL on Linux hosts as a performant alternative to basic emulation.
*   Mediated passthrough (NVIDIA vGPU, AMD MxGPU, Intel GVT-g) will be highly dependent on vendor support and licensing; these are likely later-stage additions.
*   Venus project for Vulkan support via VirtIO-GPU is a promising future direction.

### C. Virtual Memory (vRAM) Management - Technical Specifications

This subsection specifies the technical details for virtual RAM (vRAM) management within V-Architect, covering memory mapping, optimization techniques, and considerations for AI workloads.

**1. Memory Mapping & Virtualization:**
    *   **Hardware Support:**
        *   **Intel EPT (Extended Page Tables) / AMD RVI/NPT (Rapid Virtualization Indexing / Nested Page Tables):** These are mandatory hardware features to be used for memory virtualization. The Core Engine will configure and manage these structures to map Guest Physical Addresses (GPAs) to Host Physical Addresses (HPAs).
        *   The hypervisor is responsible for managing the EPT/NPT structures, ensuring each VM has its own isolated physical address space from the host's perspective.
    *   **Page Sizes:** Support for standard 4KB host page sizes. Support for larger page sizes (e.g., 2MB, 1GB "huge pages") for guest RAM mapping should be investigated for performance benefits, especially for memory-intensive applications and AI workloads, though initial implementation may focus on 4KB.
    *   **Memory Allocation:** When a VM is started, the Core Engine will allocate a contiguous block of host physical memory (or virtual memory that is then backed by physical memory) corresponding to the VM's configured `vram_config.size_mb`.

**2. Memory Ballooning (Dynamic Adjustment):**
    *   **Device Model:** `virtio-balloon` (conforming to a recent VirtIO specification, e.g., v1.1 or later).
    *   **Guest Driver:** Requires `virtio-balloon` driver in the guest OS.
    *   **Backend Implementation (Core Engine):**
        *   The hypervisor backend will manage requests from the guest driver to "inflate" (return memory to host) or "deflate" (request memory from host, up to initial `size_mb`).
        *   **Inflate Operation:** When the guest inflates the balloon, it provides a list of Guest Page Numbers (GPNs). The hypervisor backend will mark these corresponding HPAs as free and available for other VMs or the host, effectively reducing the VM's memory footprint on the host.
        *   **Deflate Operation:** When the guest deflates the balloon, the hypervisor backend will allocate available HPAs to back the newly requested GPNs for the VM.
    *   **Statistics:** The backend will report balloon statistics (current size, target size) via the Core Engine API for monitoring by the Desktop Orchestration Layer or AI management services.
    *   **API Control:** `VMService.UpdateVMConfiguration` could potentially be used to suggest a new target balloon size to the VM via the backend, if the guest driver supports such hints (e.g., for AI-driven proactive memory reclaim).

**3. Page Sharing (KSM - Kernel Same-page Merging - Conceptual):**
    *   **Rationale:** To reduce overall host memory consumption by identifying and merging identical memory pages across different VMs (or even within a single VM).
    *   **Mechanism (If Implemented):**
        *   A background process within the Core Engine or host OS (if leveraging host KSM) would periodically scan VM memory pages.
        *   It would compute checksums/hashes of pages to find candidates for merging.
        *   Identical pages would be marked as copy-on-write (CoW), and only one physical copy would be retained. If a VM attempts to write to a shared page, a private copy is created for that VM.
    *   **Implementation Considerations:**
        *   This can be CPU intensive. The scanning rate and aggressiveness should be configurable.
        *   Initial V-Architect versions might not implement custom KSM due to complexity and potential performance side-effects, relying instead on host OS KSM if available and beneficial.
        *   If implemented, `ConfigureEngineRequest` could control KSM parameters.
    *   **Security Note:** While generally considered safe, KSM has been theoretically implicated in side-channel attacks in some research. Mitigation techniques (e.g., careful selection of scan targets, avoiding sharing across VMs of different security contexts if deemed too risky) would need to be considered.

**4. Memory Overcommitment:**
    *   **Strategy:** V-Architect will allow the sum of configured vRAM for all running VMs to exceed the available physical host RAM, relying on memory ballooning and potentially host swapping (if enabled and configured on the host OS).
    *   **Management:** The AI-driven resource management (Phase 2F and 4A) will be critical in managing overcommitted environments to prevent excessive swapping and performance degradation by proactively adjusting balloon sizes or migrating VMs.

**5. AI-Optimized Memory Regions (NUMA Awareness & Locality):**
    *   **Goal:** For VMs with assigned AI accelerators (vAI-GPU, vNPU via passthrough or SR-IOV), ensure their vRAM is allocated from host memory that is "local" to the physical AI accelerator's NUMA node to minimize memory access latency.
    *   **Implementation:**
        *   **Host NUMA Topology Detection:** The `GetHostCapabilitiesResponse` will include information about the host's NUMA topology (nodes, CPUs per node, memory per node, PCIe devices per node).
        *   **VM Configuration:** The `vram_config.ai_optimized_flags.numa_node_affinity` field in `VMConfig` can allow a user or AI (Gemini) to suggest a specific host NUMA node for memory allocation.
        *   **Hypervisor Memory Allocation Policy:** When a VM is started with AI accelerators assigned:
            *   The Core Engine identifies the NUMA node(s) of the assigned physical AI accelerator(s).
            *   It will attempt to allocate the VM's vRAM primarily from the memory attached to that same NUMA node(s).
            *   This might involve using platform-specific memory allocation APIs that allow specifying NUMA policy (e.g., `set_mempolicy` on Linux).
        *   **vCPU Affinity:** The vCPUs of such a VM should also preferentially be scheduled on physical CPU cores belonging to the same NUMA node as the VM's memory and its assigned AI accelerator to maintain locality.
    *   **Contiguous Memory (Conceptual):** The `vram_config.ai_optimized_flags.prefer_contiguous` flag is a hint. While the hypervisor will allocate the VM's initial RAM as a contiguous block in the *host virtual address space*, true host *physical* contiguity for very large allocations is difficult to guarantee long-term. The main benefit here comes from NUMA locality.

**Initial Implementation Considerations:**
*   Focus on robust EPT/NPT-based memory virtualization as the foundation.
*   Implement `virtio-balloon` for dynamic memory adjustments.
*   Develop NUMA-aware memory allocation for VMs with passthrough devices as a key feature for AI workloads.
*   Custom KSM and huge page support for guest RAM can be later optimizations if clear benefits are demonstrated.

### D. Virtual Storage (vHDD/vSSD) Controllers - Technical Specifications

This subsection provides the technical specifications for virtual storage controllers and virtual disk image management within V-Architect, aiming for a balance of performance, compatibility, and advanced features like snapshotting.

**1. Emulated Storage Controller Models:**

    *   **`virtio-blk` (Primary Performance Controller):**
        *   **Specification Version:** Target VirtIO v1.1 or later for modern features.
        *   **Features:**
            *   Support for multiqueue to improve parallelism and performance with multi-core guests.
            *   Support for indirect descriptors and packed virtqueues.
            *   Feature negotiation (e.g., `VIRTIO_BLK_F_RO` for read-only, `VIRTIO_BLK_F_FLUSH`, `VIRTIO_BLK_F_DISCARD` or `VIRTIO_BLK_F_WRITE_ZEROES` for TRIM/UNMAP support with SSDs).
        *   **Backend Implementation (Core Engine):** The hypervisor backend will translate `virtio-blk` requests into I/O operations on the host system, targeting the virtual disk image file. Efficient asynchronous I/O (e.g., Linux AIO, io_uring, or Windows Overlapped I/O) should be used.
    *   **`virtio-scsi` (Optional - For Broader SCSI Device Compatibility):**
        *   **Specification Version:** Target VirtIO v1.1 or later.
        *   **Rationale:** While `virtio-blk` is simpler and often faster for single block devices, `virtio-scsi` provides a full SCSI HBA emulation, allowing for more complex storage topologies, support for SCSI-specific commands, and easier attachment of virtual CD-ROMs or other SCSI-passthrough devices if needed.
        *   **Implementation:** If included, would also require a robust backend capable of handling SCSI commands. May be a lower priority for initial implementation compared to `virtio-blk` and NVMe.
    *   **NVMe (Non-Volatile Memory Express - High Performance):**
        *   **Specification Version:** Emulate a standard NVMe controller (e.g., NVMe 1.3 or 1.4).
        *   **Features:** Support for multiple queues, admin and I/O submission/completion queues, and basic NVMe command sets (Read, Write, Flush).
        *   **Backend Implementation:** Similar to `virtio-blk`, the backend translates NVMe commands to host I/O. This is typically more complex than `virtio-blk` due to the NVMe architecture.
        *   **Use Case:** Primarily for VMs requiring the highest possible storage I/O performance, especially when backed by host NVMe SSDs.
    *   **IDE/SATA (Legacy/Compatibility):**
        *   **Emulated Devices:** Emulate a standard IDE controller (e.g., PIIX3/4) and/or AHCI SATA controller.
        *   **Purpose:** Primarily for compatibility with older guest operating systems that may lack VirtIO or NVMe drivers. Also used for virtual CD-ROM/DVD-ROM drives.
        *   **Performance:** Expected to be lower than VirtIO or NVMe.
        *   **Virtual CD-ROM/DVD-ROM:** Typically attached as an ATAPI device on an emulated IDE/SATA controller. Supports mounting ISO images.

**2. Virtual Disk Image Formats:**

    *   **QCOW2 (QEMU Copy-On-Write version 2 - Preferred Format):**
        *   **Target Version:** Support for recent QCOW2 features (e.g., those in QEMU 4.x or later).
        *   **Key Features to Support:**
            *   **Snapshots:** Internal snapshots (storing deltas within the QCOW2 file). External snapshots (overlay files) for more complex scenarios.
            *   **Thin Provisioning (Dynamic Sizing):** Disk images only consume host storage as data is written by the guest.
            *   **Backing Files (Differencing Disks):** Essential for linked clones and snapshot chains.
            *   **Compression (Conceptual):** Read-only support for zlib-compressed QCOW2 images. Write support for compression is complex and lower priority.
            *   **Encryption (Conceptual):** Support for AES-encrypted QCOW2 images (LUKS format within QCOW2 or native QCOW2 encryption if available and mature). Key management becomes critical.
        *   **Implementation:** Utilize a robust library for QCOW2 manipulation (e.g., `libqcow` or directly using QEMU's block layer components if modular enough).
    *   **Raw:**
        *   **Format:** Direct bit-for-bit image of a disk. Can be a host file or a raw block device.
        *   **Features:** Simple, potentially slightly higher performance in some direct I/O scenarios due to no metadata overhead.
        *   **Limitations:** No native snapshot support (requires hypervisor/filesystem level snapshotting if used for base images needing snapshots), no thin provisioning.
    *   **VMDK (VMware Virtual Disk Format - For Compatibility):**
        *   **Rationale:** To allow import/export of VMs from/to VMware environments.
        *   **Feature Support:** Focus on commonly used VMDK types (e.g., monolithic sparse, twoGbMaxExtentSparse). Support for VMDK snapshots might be limited.
        *   **Implementation:** Leverage libraries like `libvmdk` or QEMU's VMDK support.

**3. I/O Path & Performance Optimization:**

    *   **Host Caching Strategy:**
        *   **Default:** `writeback` caching (host caches writes, guest is informed of completion, good performance but risk of data loss on host crash if not using `FUA` or flushes).
        *   **Optional:** `writethrough` (guest write completes only when data is on physical media, safer but slower), `none` (direct I/O, bypasses host cache, useful for specific database workloads). Configurable per virtual disk.
    *   **Asynchronous I/O:** Utilize host OS asynchronous I/O capabilities (e.g., Linux AIO, io_uring, Windows Overlapped I/O) in the hypervisor backends for `virtio-blk` and NVMe to avoid blocking hypervisor threads on guest I/O.
    *   **I/O Threads (Conceptual):** For very high I/O workloads, consider dedicated I/O threads per VM or per virtual disk to process I/O requests off the main vCPU emulation threads.
    *   **TRIM/Discard Passthrough:** When a guest OS issues TRIM/Discard commands (e.g., when deleting files on an SSD), these should be passed through to the host file system or physical SSD if the virtual disk image is thin-provisioned (e.g., QCOW2, raw on SSD with `discard` option) to reclaim space and maintain SSD performance. This is a feature of `virtio-blk` and NVMe.

**Initial Implementation Considerations:**
*   Prioritize `virtio-blk` with QCOW2 as the primary combination for performance and features.
*   Implement basic IDE/SATA for CD-ROM support and legacy OS compatibility.
*   Raw format support should be straightforward.
*   NVMe emulation is a higher complexity item for later, focusing on very high performance.
*   Focus on robust snapshotting with QCOW2.
*   Asynchronous I/O and efficient host caching are critical for good performance.

### E. Virtual Networking (vNICs, Virtual Switches, Virtual Routers) - Technical Specifications

This subsection provides the technical specifications for virtual networking components within V-Architect, enabling flexible and performant network connectivity for VMs.

**1. Virtual Network Interface Cards (vNICs):**

    *   **`virtio-net` (Primary Performance vNIC):**
        *   **Specification Version:** Target VirtIO v1.1 or later.
        *   **Features:**
            *   **Multiqueue `virtio-net`:** Essential for performance with multi-vCPU VMs, allowing parallel packet processing across multiple queues, each potentially handled by a different vCPU/host thread. Number of queues should be configurable (e.g., up to the number of vCPUs).
            *   **TSO (TCP Segmentation Offload), UFO (UDP Fragmentation Offload), CSO (Checksum Offload):** Guest offloads these tasks to the hypervisor backend, reducing guest CPU usage for networking.
            *   **Packed Virtqueues:** For improved performance and reduced overhead.
            *   **Control Virtqueue:** For advanced configuration and feature negotiation.
        *   **Backend Implementation (Core Engine):**
            *   The hypervisor backend connects the `virtio-net` device to a host-side networking entity (e.g., a tap device connected to a Linux bridge/Open vSwitch, or directly into a user-space vSwitch).
            *   Efficient packet forwarding between guest and host networking stack.
    *   **Emulated NICs (Legacy/Compatibility):**
        *   **Model Example:** `e1000` (Intel PRO/1000) or `rtl8139`.
        *   **Purpose:** For guest OSs lacking VirtIO drivers or for specific boot scenarios (e.g., PXE boot if the VirtIO driver isn't available in the PXE ROM).
        *   **Performance:** Expected to be significantly lower than `virtio-net`.

**2. Virtual Switches (vSwitches):**

    *   **Purpose:** Provide Layer 2 connectivity between VMs on the same host, and between VMs and the host network or other virtual networks.
    *   **Backend Implementation Options (Core Engine):**
        *   **Linux Host (Primary Target):**
            *   **Linux Bridge:** Utilize standard Linux bridging capabilities (`brctl` or `ip link` commands for configuration). Each vSwitch maps to a Linux bridge. Tap devices for each connected vNIC are added to the bridge.
            *   **Open vSwitch (OVS - Advanced Option):** For more complex scenarios requiring features like OpenFlow, detailed QoS, or integration with SDN controllers. V-Architect could manage OVS bridges via `ovs-vsctl` or OVSDB. Initial implementation may focus on Linux bridge for simplicity.
        *   **Windows Host (Conceptual for Desktop Orchestration Layer):**
            *   Hyper-V Virtual Switch (if WHP is used and allows such integration).
            *   Custom user-space vSwitch: More complex but offers more control. Could leverage libraries like DPDK for performance if targeting user-space networking.
    *   **Features:**
        *   **VLAN Tagging (802.1Q):** Allow vNICs and vSwitch ports to be configured with VLAN tags for network segmentation.
        *   **MAC Address Learning & Forwarding Table.**
        *   **Spanning Tree Protocol (STP):** Basic STP support to prevent loops if multiple vSwitches are interconnected (less common in typical VM setups but good for completeness).
    *   **API for Configuration (Core Engine `VMService` or dedicated `NetworkService`):**
        *   Create/delete vSwitch.
        *   Add/remove vNIC (tap device) to/from vSwitch.
        *   Configure VLANs on vSwitch ports.
        *   (If OVS) More advanced flow rule management.
    *   **Default vSwitch Modes (from Blueprint Phase 1E):**
        *   **Host-Only:** Bridge with no connection to physical NICs.
        *   **Internal:** Similar to host-only, potentially with stricter isolation from host stack if using custom switching.
        *   **Bridged/External:** Linux bridge including a physical host NIC.
        *   **NAT:** Implemented via vRouter connected to a vSwitch (see below).

**3. Virtual Routers (vRouters):**

    *   **Purpose:** Provide Layer 3 routing between different vSwitches (subnets), and between virtual networks and external networks (e.g., the internet via host's physical NIC). Implement firewalling and NAT.
    *   **Backend Implementation Options (Core Engine):**
        *   **Linux Host (Primary Target):**
            *   **Host-based Routing & Firewalling:** Utilize the host's IP forwarding capabilities (`net.ipv4.ip_forward`), iptables/nftables for firewall rules (ACLs, NAT/PAT), and DHCP server software (e.g., `dnsmasq`) running on the host and attached to relevant vSwitch bridges.
            *   **Dedicated Network VM (Router Appliance):** For more complex routing protocols or advanced firewall features, a specialized lightweight VM running a router OS (e.g., VyOS, OpenWrt, or a custom Linux build) could be deployed by V-Architect. This VM would have vNICs connected to multiple vSwitches.
            *   **User-space Router:** A custom user-space routing/firewall application, potentially using DPDK for high performance. This is the most complex option.
        *   **Windows Host (Conceptual):**
            *   Utilize Windows Internet Connection Sharing (ICS) or Routing and Remote Access Service (RRAS) if controllable via APIs.
            *   Dedicated Network VM approach is also viable.
    *   **Features:**
        *   **Static Routing:** User-defined static routes.
        *   **Dynamic Routing (Conceptual - for advanced scenarios/Network VMs):** OSPF, BGP if using a dedicated router VM.
        *   **Firewalling:** Stateful packet inspection, Access Control Lists (ACLs) based on source/destination IP/port, protocol.
        *   **NAT/PAT (Network Address Translation / Port Address Translation):** For allowing VMs on private vSwitches to access external networks using the host's IP or a dedicated vRouter IP.
        *   **DHCP Server:** Provide IP addresses, gateway, and DNS information to VMs on connected vSwitches.
    *   **API for Configuration (Core Engine `VMService` or dedicated `NetworkService`):**
        *   Create/delete vRouter.
        *   Attach/detach vRouter interface to/from vSwitch.
        *   Configure IP addresses on vRouter interfaces.
        *   Manage routing table entries (for static routes).
        *   Manage firewall rules.
        *   Configure NAT/PAT rules.
        *   Configure DHCP server scopes and options.

**Initial Implementation Considerations:**
*   Prioritize `virtio-net` for vNICs.
*   For vSwitches on Linux, start with Linux Bridge due to its ubiquity and simplicity. OVS can be a later enhancement.
*   For vRouters on Linux, initially focus on host-based routing with iptables/nftables and `dnsmasq` for NAT and DHCP, as this is a common and well-understood pattern. A dedicated router VM is a powerful but more complex option for later.
*   Ensure the Core Engine API allows for programmatic creation and configuration of these network topologies as defined by the user (conceptually via the Advanced Virtual Network Topology Management in Phase 2E).
