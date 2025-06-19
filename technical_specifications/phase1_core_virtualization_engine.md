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

### F. AI CPU (vNPU/vTPU - Virtual Neural Processing Unit / Virtual Tensor Processing Unit) - Technical Specifications

This subsection details the technical specifications for the virtual AI CPU (referred to broadly as vNPU/vTPU), designed to provide VMs with accelerated execution for AI/ML workloads by leveraging physical AI accelerators or specialized CPU instructions.

**1. Hardware Interface Emulation:**

    *   **Appearance to Guest OS:**
        *   The vNPU/vTPU will typically appear as a **PCIe device** to the guest OS.
        *   **Vendor/Device IDs:** A V-Architect specific Vendor ID and a set of Device IDs will be used to represent different types or capabilities of vNPU/vTPUs (e.g., one Device ID for a generic software-emulated vNPU, others for specific mediated passthrough profiles of physical hardware).
        *   **Memory-Mapped I/O (MMIO) Regions:** The device will expose MMIO regions for:
            *   **Control Registers:** For device discovery, capability querying, context setup, task submission, and interrupt management.
            *   **Doorbell Registers:** For notifying the device that new tasks are available on a submission queue.
            *   **Shared Memory Regions (Conceptual):** For command queues (submission queues - SQs, completion queues - CQs) and data exchange between the guest driver and the hypervisor backend, similar to NVMe or modern VirtIO devices.
    *   **Interrupts:** The device will use standard PCIe interrupts (MSI or MSI-X) to notify the guest driver of task completion, errors, or other events.

**2. API for Guest Interaction / Paravirtualized Interface (if not pure passthrough):**

    *   **Rationale:** A standardized paravirtualized interface is preferred for mediated passthrough or software-emulated vNPUs to provide a stable API for guest drivers across different underlying hardware or emulation strategies. Pure passthrough would rely on vendor drivers in the guest.
    *   **Interface Type (Conceptual - "VirtIO-AI-Accelerator"):** A new VirtIO-based device specification could be conceptualized for this, or a custom PCIe device interface defined. Key aspects:
        *   **Device Discovery & Capability Negotiation:** Guest driver queries device capabilities (e.g., supported operations, model formats, memory capacity, available compute units).
        *   **Context Management:** APIs to create, manage, and destroy execution contexts for different AI models or processes.
        *   **Task Submission:**
            *   Guest driver places AI tasks (e.g., pointers to model graphs, input/output tensor descriptors, execution parameters) into Submission Queues in shared memory.
            *   Rings a doorbell register to notify the hypervisor backend.
        *   **Task Completion:**
            *   Hypervisor backend processes tasks and places completion events (status, output tensor locations, performance metrics) into Completion Queues in shared memory.
            *   Raises an interrupt to notify the guest driver.
        *   **Data Transfer:** Efficient mechanisms for transferring input/output tensor data between guest RAM and the accelerator (either physical device memory via DMA managed by hypervisor, or host RAM for software emulation). This might involve registering guest memory regions with the hypervisor.
    *   **Data Structures for API (Conceptual):**
        *   `AI_Task_Descriptor`: Model identifier, input tensor(s) (address, size, type), output tensor(s) buffer info, execution parameters (e.g., batch size, precision).
        *   `AI_Completion_Event`: Task ID, status code, performance metrics, output tensor(s) metadata.

**3. Backend Logic (Core Engine):**

    *   **Passthrough/SR-IOV Mode:**
        *   The Core Engine uses VFIO to assign the physical NPU/TPU PCIe device (or a Virtual Function if SR-IOV is used) directly to the VM.
        *   The hypervisor's role is minimal, primarily managing IOMMU isolation and PCIe configuration space virtualization.
        *   Guest OS uses vendor-specific drivers for the physical device.
    *   **Mediated Passthrough Mode:**
        *   **Vendor API Integration:** The Core Engine's backend interfaces with the host-level drivers and APIs of the physical AI accelerator (e.g., NVIDIA MIG manager, AMD ROCm/VITIS AI NPU scheduler, Google Cloud TPU APIs if running in GCP).
        *   **Task Scheduling & Multiplexing:** The backend receives AI tasks from multiple VMs via the paravirtualized interface. It schedules these tasks onto the shared physical accelerator(s) based on VM priorities, requested resources, and accelerator availability. This may involve context switching on the physical accelerator.
        *   **Resource Management:** Manages allocation of accelerator compute units, memory, and bandwidth among contending VMs.
    *   **Optimized Software Emulation Mode (If No Physical Accelerator):**
        *   The backend receives AI tasks and executes them on the host CPU using optimized AI libraries (e.g., Intel oneDNN, ARM Compute Library, Google XNNPACK, TensorFlow Lite runtime).
        *   Focus on common inference operations and potentially limited training for smaller models.
        *   Leverage host CPU vector extensions (AVX, NEON) extensively.
    *   **DMA Management:** For modes involving physical accelerators, the backend is responsible for securely managing DMA transfers between guest RAM (via GPA to HPA translation) and the accelerator's memory or MMIO space.

**4. Performance Counter Definitions (Exposed via API & Guest Interface):**

    *   **Accelerator Utilization:** Percentage of time the vNPU/vTPU (or underlying physical resource) was actively processing.
    *   **Operations Per Second (OPS/TOPS):** Relevant to the type of accelerator (e.g., INT8 TOPS, FP16 TFLOPS).
    *   **Memory Bandwidth Usage:** Data transfer rate between guest/host and the accelerator.
    *   **Task Latency:** Average time taken to complete an AI task.
    *   **Context Switch Overhead (for mediated mode):** Time spent switching active models/contexts on the physical accelerator.
    *   **Queue Lengths (SQ/CQ):** To monitor for bottlenecks in task submission/completion.
    *   These metrics will be exposed via `GetVMStatusResponse` or a dedicated metrics RPC in `VMService`, and potentially to the guest via the paravirtualized interface for self-monitoring.

**5. Configuration in `vm_config_schema.json`:**

    *   The `ai_accelerated_hardware.ai_cpu_config` object will contain fields like:
        *   `type`: (enum: "passthrough", "mediated", "software_emulated")
        *   `physical_device_id`: (string, PCIe address for passthrough)
        *   `mediated_profile_name`: (string, e.g., "nvidia_a100_mig_1g.10gb_profile", "google_tpu_v4_slice_small")
        *   `emulated_performance_tier`: (enum: "low", "medium", "high" for software emulation, influencing CPU resource allocation)
        *   `num_virtual_devices`: (integer, for scenarios where a single physical device is split into multiple virtual ones visible to the guest).

**Initial Implementation Considerations:**
*   Start with PCIe passthrough for a specific, well-supported physical AI accelerator (e.g., a common GPU in AI mode or a development board NPU). This validates the basic assignment and guest driver functionality.
*   Develop the paravirtualized interface ("VirtIO-AI-Accelerator") incrementally, starting with basic task submission and completion for a software-emulated backend.
*   Mediated passthrough is highly vendor-specific and will require deep integration with vendor SDKs; likely a later step.

### G. AI RAM - Technical Specifications (Consolidation)

This subsection consolidates the technical specifications for how V-Architect handles memory optimizations specifically beneficial for AI workloads, largely building upon the general vRAM management capabilities detailed in Section III.C. AI RAM is not a distinct type of emulated hardware but rather a set of configurations and hypervisor policies applied to standard vRAM to enhance performance for AI models.

**1. Core Principles (Cross-reference Section III.C - vRAM Management):**

    *   **NUMA Awareness & Locality:**
        *   **Technical Detail:** As specified in Section III.C.5, the Core Engine MUST query host NUMA topology (nodes, CPU-memory-PCIe device locality) via `GetHostCapabilitiesResponse`.
        *   When a VM is configured with `vram_config.ai_optimized_flags.numa_node_affinity` set to a specific host NUMA node (or when AI (Gemini) infers optimal affinity based on assigned AI accelerators like vAI-GPUs/vNPUs), the Core Engine's memory allocator MUST prioritize allocating the VM's physical RAM from that specified NUMA node.
        *   Platform-specific APIs (e.g., `set_mempolicy` with `MPOL_BIND` on Linux) shall be used to enforce this memory placement policy for the VM's process.
        *   Corresponding vCPUs for such VMs should also be affinitized to pCPUs within the same NUMA node by the Core Engine's scheduler to prevent remote memory access penalties.
    *   **Large Contiguous Memory Blocks (Conceptual Hint):**
        *   **Technical Detail:** The `vram_config.ai_optimized_flags.prefer_contiguous` flag in `VMConfig` serves as a hint to the Core Engine.
        *   While the hypervisor typically allocates a VM's initial memory as a contiguous block in the *host's virtual address space*, true *physical* contiguity for large multi-gigabyte AI models is difficult to guarantee over extended periods due to host memory fragmentation.
        *   The primary benefit of "AI RAM" in current implementation will stem from NUMA locality. Future research could explore advanced host physical memory defragmentation or reservation techniques if this flag is set, but this is not part of the initial specification beyond best-effort contiguous allocation at VM start.
    *   **Host Page Size Considerations:**
        *   While Section III.C.1 mentions investigating larger host page sizes (2MB/1GB huge pages) for guest RAM mapping, this is particularly relevant for AI RAM. If the host supports transparent huge pages (THP) or can be configured for explicit huge page allocation, and the performance benefits for AI workloads are validated, the Core Engine should attempt to use huge pages for VMs with significant AI RAM allocations to reduce TLB pressure and improve memory access speeds. This requires careful host configuration.

**2. Configuration in `vm_config_schema.json` (Cross-reference):**

    *   The relevant fields are within the `vram_config` object, under `ai_optimized_flags`:
        *   `prefer_contiguous`: (boolean, default: `false`) - Hint for contiguous physical allocation.
        *   `numa_node_affinity`: (integer or null, default: `null`) - Specifies preferred host NUMA node ID.
    *   The AI (Gemini) through the **Virtual Hardware Recommendations (Phase 4A2)** feature is expected to set these flags appropriately when configuring a VM for AI workloads based on detected host capabilities and assigned AI accelerators.

**3. Performance Monitoring for AI RAM:**

    *   In addition to standard memory performance counters (page faults, bandwidth from host perspective), specific monitoring for NUMA effects could be beneficial:
        *   **Remote Node Memory Accesses (Conceptual):** If feasible through hypervisor or host PMU counters, track the proportion of memory accesses by a VM that go to remote NUMA nodes versus its local assigned node. A high remote access rate for an AI-optimized VM would indicate sub-optimal placement or configuration.
        *   This data would be invaluable for Gemini to refine its NUMA affinity recommendations or to alert users to potential performance issues.

**Initial Implementation Considerations:**
*   Robust NUMA topology detection on the host is the immediate priority.
*   Implementation of NUMA-aware memory allocation policies for VMs in the Core Engine is critical.
*   vCPU-to-pCPU affinity that respects NUMA node choices for memory is also essential.
*   Huge page support for guest RAM is an optimization to be evaluated based on complexity and performance impact.
*   Advanced monitoring for NUMA effects is a research/later enhancement item.

### H. AI Graphics Card (vAI-GPU/NPU) - Technical Specifications

This subsection details the technical specifications for the virtual AI Graphics Card (vAI-GPU), which may also represent other Neural Processing Units (NPUs) suited for AI/ML workloads. It focuses on providing VMs with high-performance access to GPU/NPU compute capabilities for AI frameworks.

**1. Hardware Interface Emulation:**

    *   **Appearance to Guest OS:**
        *   Typically as a **PCIe device**.
        *   **Vendor/Device IDs:**
            *   For **passthrough mode**, the original physical device's Vendor/Device ID will be exposed to the guest.
            *   For **mediated passthrough mode (vGPU profiles)**, a V-Architect specific Vendor ID or a vendor-defined vGPU Device ID (e.g., NVIDIA vGPU device IDs) will be used. This identifies the device as a virtualized AI accelerator.
        *   **MMIO Regions & BARs:** Exposes Base Address Registers (BARs) for control registers, device memory (VRAM), and doorbells, consistent with the physical device (for passthrough) or the vGPU specification (for mediated modes).
    *   **Interrupts:** Standard PCIe interrupts (MSI/MSI-X).

**2. API for Guest Interaction / Paravirtualized Interface:**

    *   **Passthrough Mode:**
        *   The guest OS uses standard vendor-provided drivers (e.g., NVIDIA CUDA drivers, AMD ROCm drivers) for the physical GPU/NPU being passed through. No V-Architect specific paravirtualized API is involved in the data path for computation itself.
        *   V-Architect's role is to ensure correct PCIe passthrough, IOMMU isolation, and ROM/firmware handling (as detailed in Section III.B.2 for general GPU passthrough).
    *   **Mediated Passthrough Mode (vGPU Profiles):**
        *   The guest OS uses vendor-provided vGPU guest drivers (e.g., NVIDIA AI Enterprise guest drivers, AMD vGPU drivers).
        *   These guest drivers communicate with the vendor's host-side vGPU manager, which then schedules tasks on the physical GPU. V-Architect's Core Engine facilitates this by:
            *   Setting up the vGPU instance on the physical GPU via the vendor's management APIs (e.g., `nvidia-vgpu-mgr` or equivalent).
            *   Exposing the virtual PCIe device corresponding to the vGPU profile to the guest.
    *   **Conceptual Paravirtualized AI Compute API (Future Enhancement - Lower Priority):**
        *   Similar to the "VirtIO-AI-Accelerator" concept for vNPUs (Section III.F.2), a high-level paravirtualized API could be developed in the long term for common AI compute operations. This would abstract away some vendor specifics but is a significant undertaking and not part of the initial specification for vAI-GPUs, which will rely on vendor driver models.

**3. Backend Logic (Core Engine):**

    *   **Passthrough/SR-IOV Mode:**
        *   Utilizes VFIO for assigning the physical GPU/NPU PCIe device or a Virtual Function (VF) to the VM.
        *   Manages IOMMU mappings and ensures exclusive access for the VM.
    *   **Mediated Passthrough Mode (vGPU Profiles):**
        *   **Vendor API Integration:** The Core Engine (or a dedicated V-Architect service on the host) interfaces with the GPU vendor's vGPU management software (e.g., NVIDIA AI Enterprise host drivers, AMD ROCm host components with vGPU support).
        *   **Profile Allocation:** When a VM requests a specific vAI-GPU profile (e.g., "nvidia_a100_20g_profile"), the Core Engine uses the vendor API to instantiate that profile on a compatible physical GPU with available resources.
        *   **Resource Management:** The vendor's vGPU manager handles the fine-grained scheduling and multiplexing of the physical GPU's resources (compute units, VRAM, encoders/decoders) among the vGPU instances assigned to different VMs. V-Architect monitors overall resource availability.
        *   **Licensing:** The Core Engine must pass through any licensing checks required by the vendor's vGPU solution. V-Architect will not bypass vendor licensing.

**4. Performance Counter Definitions:**

    *   Metrics will largely depend on what the vendor drivers and vGPU management software expose. V-Architect will aim to collect and present:
        *   **vGPU Engine Utilization:** (e.g., graphics/compute engine load).
        *   **vGPU Framebuffer Memory Usage:** (VRAM consumed by the vGPU instance).
        *   **Tensor Core Activity / AI Compute Unit Utilization (if exposed by vendor tools for specific profiles).**
        *   **Power Consumption (if exposed for the vGPU instance or allocatable portion of physical GPU).**
        *   **Encoder/Decoder Utilization (if relevant for AI vision workloads).**
    *   These metrics will be exposed via `GetVMStatusResponse` or a dedicated metrics RPC, and ideally correlated with specific AI workloads if possible (e.g., via tagging or process monitoring within the guest, with user consent).

**5. Configuration in `vm_config_schema.json`:**

    *   The `ai_accelerated_hardware.ai_graphics_card_config` object will contain fields like:
        *   `type`: (enum: "passthrough", "mediated_vgpu")
        *   `physical_device_id`: (string, PCIe address for passthrough, e.g., "0000:01:00.0")
        *   `vendor_specific_profile_id`: (string, e.g., "nvidia_grid_a40-8q", "amd_firepro_s7150x2_vgpu" - names would align with vendor terminology for specific vGPU profiles suitable for AI).
        *   `dedicated_memory_mb`: (integer, often defined by the profile but can be specified for some configurations).
        *   `driver_options`: (array of strings, e.g., for passing specific parameters to guest drivers if necessary).
    *   The `graphics_config` section in `VMConfig` might also be used in conjunction if the vAI-GPU also provides standard display output, or it might be distinct if the vAI-GPU is a headless compute accelerator.

**Initial Implementation Considerations:**
*   Prioritize PCIe passthrough for a selection of common, high-performance GPUs used in AI (e.g., NVIDIA Ampere/Hopper series, AMD CDNA series).
*   For mediated passthrough, initial efforts should focus on integrating with one leading vendor's vGPU solution (e.g., NVIDIA AI Enterprise, if development licenses and hardware are accessible) as a proof of concept. This requires significant vendor-specific work.
*   Ensure `GetHostCapabilitiesResponse` accurately reports physical GPUs suitable for AI passthrough and any recognized vGPU profiles the host can support.

### I. AI Switches - Technical Specifications

This subsection details the technical specifications for AI Switches, which build upon the standard Virtual Switch capabilities (defined in Section III.E.2) to provide optimized Layer 2 networking for AI workloads, particularly for high-throughput, low-latency inter-VM communication in distributed AI training/inference scenarios.

**1. Relationship to Standard Virtual Switches:**
    *   AI Switches are an enhanced mode or type of standard vSwitch. They inherit all base vSwitch functionalities (MAC learning, VLAN tagging, connection to vNICs).
    *   The `vm_config_schema.json` for a vNIC's network attachment will allow specifying connection to an "AI Switch" instance or enabling "AI-Optimized Mode" on a standard vSwitch connection.

**2. AI-Driven Traffic Shaping & QoS - API and Logic:**
    *   **Telemetry Input to Gemini:**
        *   AI Switches will export detailed flow-level telemetry to Google Gemini (via the V-Architect monitoring service). This includes source/destination MAC, VLAN tags, traffic volume, packet rates, and potentially (if using advanced inspection or guest-provided hints) traffic type identifiers (e.g., "RDMA for gradients," "TensorFlow data feed," "general TCP").
        *   The exact telemetry format will be defined by the monitoring service's API.
    *   **Policy Input from Gemini (API for Core Engine):**
        *   Gemini will provide traffic shaping and QoS policies to the Core Engine, which then configures the AI Switch backend.
        *   **Conceptual API (part of `CoreHypervisorService` or a dedicated `NetworkPolicyService`):**
            ```protobuf
            message AISwitchPolicy {
              string ai_switch_instance_id = 1;
              repeated TrafficRule rules = 2;
            }

            message TrafficRule {
              string rule_id = 1;
              FlowMatcher flow_matcher = 2; // Matches on src/dst MAC, VLAN, ethertype, potentially L4 ports or DPI hints
              QoSParameters qos_params = 3;
              PathPreference path_preference = 4; // e.g., prefer RDMA-capable path
            }

            message FlowMatcher {
              string src_mac = 1;
              string dst_mac = 2;
              uint32 vlan_id = 3;
              uint32 ethertype = 4;
              // ... other L2/L3/L4 matching criteria
            }

            message QoSParameters {
              uint32 priority = 1; // e.g., 802.1p style priority
              uint64 min_bandwidth_bps = 2;
              uint64 max_bandwidth_bps = 3;
              uint32 buffer_allocation_percentage = 4; // Hint for buffer management
            }

            message PathPreference {
              bool prefer_rdma_path = 1;
              // ... other path hints (e.g., specific host NIC if multiple are bridged)
            }

            service NetworkPolicyService {
              rpc ApplyAISwitchPolicy(AISwitchPolicy) returns (PolicyApplyResponse);
            }
            ```
    *   **Backend Logic (AI Switch):**
        *   The AI Switch backend (e.g., enhanced Linux bridge, OVS with programmable rules, or user-space switch) will implement mechanisms to enforce these policies:
            *   Priority queues for high-priority AI traffic.
            *   Bandwidth shaping/rate limiting per flow or per vNIC.
            *   Dynamic buffer allocation.
            *   Steering traffic over RDMA paths if available and policy dictates.

**3. RDMA (Remote Direct Memory Access) Support:**

    *   **Host Prerequisite:** Host NICs and network infrastructure must support RDMA (e.g., RoCEv2 or InfiniBand). Host OS drivers for RDMA must be installed and configured.
    *   **Guest VM Configuration:**
        *   Guest OS must have RDMA-capable drivers (e.g., for `virtio-net` if it supports RDMA offload/passthrough, or for a passed-through SR-IOV VF of an RDMA-capable NIC).
        *   Applications within the guest (e.g., MPI libraries, NCCL for NVIDIA GPUs) must be configured to use RDMA.
    *   **AI Switch Backend Logic for RDMA:**
        *   **Path Discovery:** The AI Switch backend, in conjunction with host RDMA utilities, identifies RDMA-capable paths between host NICs that are part of the switch's bridge (if applicable).
        *   **Traffic Steering:** When a `PathPreference` from Gemini indicates `prefer_rdma_path` for a matched flow, the AI Switch will attempt to direct this traffic over the RDMA path. This might involve:
            *   For SR-IOV: Ensuring VFs assigned to communicating VMs can establish RDMA connections.
            *   For paravirtualized RDMA (conceptual): A `virtio-rdma` like interface or extensions to `virtio-net` allowing guest to signal RDMA operations, with hypervisor facilitating secure mapping to host RDMA resources.
        *   **Security:** IOMMU must be used to ensure memory protection for RDMA operations initiated by guests.

**4. Configuration in `vm_config_schema.json`:**

    *   The `network_interfaces.network_attachment` field might include an option like:
        *   `ai_switch_id`: (string) - ID of the pre-configured AI Switch instance.
        *   Or, a vSwitch attachment could have an `ai_optimized_mode: true` flag.
    *   `network_interfaces.rdma_settings`:
        *   `enabled`: (boolean, default: `false`)
        *   `mode`: (enum: "passthrough_sriov_vf", "paravirtualized_experimental") - if different modes are supported.

**Initial Implementation Considerations:**
*   Focus first on the telemetry required by Gemini and the API for Gemini to push QoS/priority rules to an enhanced Linux bridge or OVS.
*   RDMA support is highly advanced. Initial support might focus on SR-IOV passthrough of RDMA-capable NIC VFs to VMs, with the AI Switch being "aware" of these VFs. Paravirtualized RDMA is a research topic.
*   Develop specific performance metrics for AI Switch throughput, latency for prioritized flows, and RDMA utilization (if applicable).

### J. AI Routers - Technical Specifications

This subsection details the technical specifications for AI Routers, which extend standard Virtual Router capabilities (Section III.E.3) with AI-driven intelligence for Layer 3 traffic prioritization and path selection, especially for AI workloads involving communication across different subnets or to external AI services.

**1. Relationship to Standard Virtual Routers:**
    *   AI Routers are an enhanced mode or type of standard vRouter. They inherit all base vRouter functionalities (IP routing, firewall, NAT, DHCP).
    *   The `vm_config_schema.json` or network topology configuration will allow designating a vRouter instance as an "AI Router" or enabling "AI-Optimized Mode."

**2. AI-Driven Traffic Prioritization & Routing - API and Logic:**

    *   **Telemetry Input to Gemini:**
        *   AI Routers export flow data (source/destination IP/port, protocol, traffic volume, application ID if available via DPI or hints from Phase 4C3 AI Services Gateway) to Gemini.
        *   Network path quality metrics (latency, jitter, packet loss to key external AI service endpoints or between virtual subnets) are also collected.
    *   **Policy Input from Gemini (API for Core Engine):**
        *   Gemini provides routing policies and traffic prioritization rules.
        *   **Conceptual API (extending `NetworkPolicyService`):**
            ```protobuf
            message AIRouterPolicy {
              string ai_router_instance_id = 1;
              repeated RoutingRule routing_rules = 2;
              repeated FirewallPriorityRule firewall_priority_rules = 3; // For dynamic firewall rule adjustments based on AI needs
            }

            message RoutingRule {
              string rule_id = 1;
              FlowMatcherL3L4 flow_matcher = 2; // Matches on IP, port, protocol, DSCP
              RoutePreference route_preference = 3;
            }

            message FlowMatcherL3L4 {
              string src_ip_prefix = 1;
              string dst_ip_prefix = 2;
              uint32 protocol = 3; // e.g., TCP=6, UDP=17
              uint32 src_port = 4;
              uint32 dst_port = 5;
              uint32 dscp_value = 6;
            }

            message RoutePreference {
              string nexthop_gateway_override = 1; // Force specific next hop
              string egress_interface_override = 2; // Force specific egress vNIC/physical NIC
              uint32 dscp_remark_value = 3; // Re-mark DSCP for downstream QoS
              uint32 priority = 4; // Higher value means higher priority route
            }

            // FirewallPriorityRule could allow Gemini to temporarily open/close ports for specific AI tasks
            // or adjust rule strictness based on context.

            service NetworkPolicyService {
              // ... existing RPCs ...
              rpc ApplyAIRouterPolicy(AIRouterPolicy) returns (PolicyApplyResponse);
            }
            ```
    *   **Backend Logic (AI Router):**
        *   The AI Router backend (e.g., enhanced host-based routing with dynamic iptables/nftables updates, or programmable data plane in a network VM/user-space router) implements:
            *   Policy-Based Routing (PBR) to select nexthops or egress interfaces based on Gemini's rules.
            *   DSCP remarking for outbound traffic.
            *   Dynamic updates to firewall rules based on AI policies.
            *   Integration with external service endpoint monitoring (e.g., latency probes to specific AI APIs) to feed path quality data to Gemini.

**3. Integration with External AI Services (Network Path Optimization):**

    *   The AI Router, guided by Gemini, can learn or be configured with optimal network paths or settings for accessing specific external AI services (e.g., Google Vertex AI, OpenAI API).
    *   This might involve selecting a host NIC that has better peering to a cloud provider, or using specific DNS resolvers.

**4. Configuration in `vm_config_schema.json`:**

    *   A vRouter instance in the network topology definition can be flagged as `ai_router_enabled: true`.
    *   Specific policies for AI-driven routing would be managed dynamically via the API from Gemini, not typically as static VM config.

**Initial Implementation Considerations:**
*   Start with telemetry collection from vRouters.
*   Implement the API for Gemini to push PBR rules (e.g., based on source/destination IP/port) to a host-based routing backend (Linux `ip rule` and custom routing tables).
*   DSCP remarking is a relatively straightforward initial feature.
*   Dynamic firewall adjustments and integration with external service endpoint monitoring are more advanced.

## IV. Performance Optimization & Dynamic Scaling - Technical Specifications

This section details the technical specifications for features designed to optimize VM performance and enable dynamic scaling capabilities within V-Architect.

### A. Live Migration - Technical Specifications

This subsection outlines the technical details for live migrating running Virtual Machines between physical V-Architect hosts with minimal service interruption, including AI-driven optimizations.

**1. Core Live Migration Process:**

    *   **Pre-copy Iterative Memory Transfer:**
        *   **Protocol:** TCP will be used for transferring memory pages between source and destination hosts due to its reliability. Secure connection (e.g., TLS over TCP) is mandatory.
        *   **Dirty Page Tracking:** The hypervisor (KVM, WHP, or Core Engine) must support tracking memory pages dirtied by the VM during the pre-copy phase (e.g., KVM's dirty log).
        *   **Iteration Algorithm:** Multiple rounds of pre-copy. Each round copies pages dirtied since the previous round. Iteration stops when the rate of dirtying is low enough that the remaining data can be copied within an acceptable pause window, or a maximum iteration count is reached. This process is influenced by AI (Gemini) for downtime minimization (see point 3).
    *   **Stop-and-Copy Phase (VM Pause):**
        *   VM execution is briefly paused on the source host.
        *   Remaining dirty memory pages are transferred.
        *   vCPU state (registers, etc. - as defined in Section III.A.4) is transferred.
        *   Device states (for emulated and paravirtualized devices) are captured and transferred. This requires each virtual device model to support a save/restore state mechanism.
    *   **Commit & Resume on Destination:**
        *   VM state is restored on the destination host.
        *   VM execution is resumed.
        *   Network state (e.g., ARP entries on physical switches) is updated (e.g., by sending a gratuitous ARP from the VM on the new host).

**2. Device State Migration:**

    *   **VirtIO Devices:** VirtIO devices typically have well-defined state save/restore mechanisms as part of the VirtIO specification, which will be leveraged (e.g., for `virtio-blk`, `virtio-net`).
    *   **Emulated Devices (e.g., IDE, e1000):** QEMU-derived or equivalent device models usually provide state save/restore capabilities.
    *   **Passthrough Devices (PCIe DDA):** Live migration of VMs with passthrough devices is **highly complex and generally not supported** in initial V-Architect versions. It would require:
        *   Identical hardware on source and destination hosts.
        *   Device-specific quiesce/resume and state extraction/injection mechanisms, often vendor-proprietary and not universally available.
        *   This is a research item for future, advanced V-Architect versions.
    *   **vGPU / Mediated Passthrough Devices:** Live migration support depends entirely on the GPU vendor's vGPU solution. Some vendor solutions support live migration by transferring vGPU context. V-Architect will leverage vendor capabilities if available.

**3. Storage Synchronization / Migration:**

    *   **Shared Storage (Preferred for Live Migration):**
        *   **Mechanism:** VMs whose virtual disk images reside on shared storage accessible by both source and destination hosts (e.g., NFS, iSCSI SAN, GlusterFS, Ceph RBD).
        *   **Process:** Only VM memory and device state need to be migrated. The destination host takes over access to the shared disk images. Disk locking mechanisms (e.g., distributed locks if using QCOW2 on shared storage, or SAN-level LUN masking/unmasking) must be carefully managed.
    *   **Local Storage (Migration of Disk Images - "Shared Nothing" Migration):**
        *   **Mechanism:** If shared storage is not used, the VM's virtual disk images must also be transferred to the destination host.
        *   **Process:**
            *   Can occur concurrently with memory pre-copy.
            *   Utilize block-level copy, potentially using QCOW2's ability to rebase onto a transferred base image or by transferring differential snapshots.
            *   Technologies like `rsync` (for file-based images) or block-replication tools (e.g., `dd` over ssh, or custom block streaming) can be used, secured via TLS.
            *   The final synchronization of disk changes occurs during the stop-and-copy phase.
        *   **Performance Impact:** Significantly higher network bandwidth and longer migration times compared to shared storage scenarios.

**4. API for Orchestration (`VMService`):**

    *   `rpc PrepareMigrationTarget(PrepareMigrationTargetRequest) returns (PrepareMigrationTargetResponse)`
        *   `PrepareMigrationTargetRequest`: `vm_id_on_source`, `vm_config_for_dest` (potentially modified by Gemini, e.g., MAC addresses if needed), `source_host_id`, `source_vm_snapshot_for_base` (if migrating disks based on a common snapshot).
        *   `PrepareMigrationTargetResponse`: `status` (READY_FOR_DATA, FAILED), `message`, `destination_session_id`.
        *   (This RPC would set up a "receiving" VM instance on the destination host).
    *   `rpc InitiateMigration(InitiateMigrationRequest) returns (InitiateMigrationResponse)`
        *   `InitiateMigrationRequest`: `vm_id`, `destination_host_id`, `destination_session_id`, `max_downtime_ms_hint` (uint32), `transfer_bandwidth_limit_mbps` (uint32), `migrate_storage` (boolean), `storage_transfer_protocol_preference` (string, e.g., "rsync_tls", "block_stream_tls").
        *   `InitiateMigrationResponse`: `migration_job_id`, `status` (STARTED, FAILED), `message`.
    *   `rpc QueryMigrationStatus(QueryMigrationStatusRequest) returns (QueryMigrationStatusResponse)`
        *   `QueryMigrationStatusRequest`: `migration_job_id`.
        *   `QueryMigrationStatusResponse`: `status` (RUNNING, FAILED, COMPLETED), `progress_percent` (uint32), `transferred_data_gb` (float), `remaining_data_gb` (float), `current_dirty_rate_mbps` (float), `message`.
    *   `rpc FinalizeMigration(FinalizeMigrationRequest) returns (FinalizeMigrationResponse)`
        *   `FinalizeMigrationRequest`: `migration_job_id`.
        *   (This RPC would be called during the stop-and-copy phase to commit to the destination).
    *   `rpc CancelMigration(CancelMigrationRequest) returns (CancelMigrationResponse)`

**5. AI-Driven Optimizations (Google Gemini Inputs/Outputs):**

    *   **Optimal Target Host Selection (Input to Gemini):**
        *   List of potential destination hosts and their full `GetHostCapabilitiesResponse` data.
        *   Source VM's `VMConfig` and current resource utilization (CPU, memory, network, AI accelerator usage).
        *   Real-time network conditions (latency, available bandwidth) between source and potential destinations.
    *   **Optimal Target Host Selection (Output from Gemini):**
        *   `chosen_destination_host_id`.
        *   Potentially adjusted `VMConfig` for the destination (e.g., if MAC addresses need to change, or if a slightly different mediated GPU profile is chosen due to availability).
    *   **Downtime Minimization Parameters (Output from Gemini):**
        *   Suggestions for `max_precopy_iterations`, target `dirty_page_rate_threshold_mbps` to trigger stop-and-copy. These would be fed as internal parameters to the Core Engine's migration logic.
    *   **Migration Scheduling (Output from Gemini):**
        *   Recommended time window to start migration based on predicted low network load or low source/destination host activity.

**Initial Implementation Considerations:**
*   Start with live migration for VMs on shared storage (NFS initially).
*   Focus on migrating memory and VirtIO device states.
*   Implement the core pre-copy and stop-and-copy logic.
*   Develop the basic `InitiateMigration` and `QueryMigrationStatus` RPCs.
*   AI-driven target selection and downtime minimization are subsequent enhancements.
*   Live migration of local storage is a significantly more complex follow-on.
*   Passthrough device migration is a research item.

### B. "Double Specs" Feature - Technical Specifications

This subsection details the technical specifications for V-Architect's unique "Double Specs" feature, allowing users to dynamically increase (and revert) key virtual hardware resources for a running VM, orchestrated by AI (Google Gemini).

**1. Core Principle: Hot-Plug/Hot-Add & Dynamic QoS Adjustment:**
    *   The feature aims to double (or significantly increase) vCPU count, vRAM, and performance limits for storage/network I/O **without requiring a VM reboot**, leveraging guest OS and hypervisor hot-plug/hot-add capabilities and dynamic Quality of Service (QoS) adjustments.
    *   Reverting to original specs should also be a hot operation where possible.

**2. Hot-Plug API Definitions (Core Engine `VMService`):**

    *   **vCPU Hot-Add/Remove:**
        *   `rpc HotPlugVCPU(HotPlugVCPURequest) returns (HotPlugVCPUResponse)`
            *   `HotPlugVCPURequest`: `vm_id` (string), `num_vcpus_to_add` (uint32).
            *   `HotPlugVCPUResponse`: `status` (SUCCESS, FAILED, PARTIALLY_COMPLETED), `message` (string), `current_vcpu_count` (uint32).
        *   `rpc HotUnplugVCPU(HotUnplugVCPURequest) returns (HotUnplugVCPUResponse)`
            *   `HotUnplugVCPURequest`: `vm_id` (string), `num_vcpus_to_remove` (uint32) or `vcpu_ids_to_remove` (list of uint32).
            *   `HotUnplugVCPUResponse`: `status`, `message`, `current_vcpu_count`.
        *   **Backend:** Interacts with KVM/WHP APIs for CPU hot-plug. Requires guest OS support to online/offline CPUs.
    *   **Memory Hot-Add/Remove:**
        *   `rpc HotAddMemory(HotAddMemoryRequest) returns (HotAddMemoryResponse)`
            *   `HotAddMemoryRequest`: `vm_id` (string), `memory_mb_to_add` (uint64).
            *   `HotAddMemoryResponse`: `status`, `message`, `current_memory_mb` (uint64).
        *   `rpc HotRemoveMemory(HotRemoveMemoryRequest) returns (HotRemoveMemoryResponse)` (More complex, may rely on ballooning down first or specific hardware support like ACPI memory unplug)
            *   `HotRemoveMemoryRequest`: `vm_id` (string), `memory_mb_to_remove` (uint64).
            *   `HotRemoveMemoryResponse`: `status`, `message`, `current_memory_mb`.
        *   **Backend:** Leverages ACPI memory hot-plug if supported by guest and hypervisor. For adding memory, can involve allocating new memory blocks and updating EPT/NPT mappings. Removing memory is more challenging; may initially be limited to what `virtio-balloon` can reclaim or if specific memory device unplug is supported.

**3. Guest OS Requirements & Communication:**

    *   **CPU Hot-Plug:**
        *   **Linux:** Requires kernel compiled with ACPI CPU hotplug support. CPUs are typically added in an "offline" state and need to be onlined by the guest via sysfs (e.g., `echo 1 > /sys/devices/system/cpu/cpuX/online`).
        *   **Windows Server:** Supports CPU hot-add on compatible hardware/hypervisors.
    *   **Memory Hot-Plug:**
        *   **Linux:** Requires kernel compiled with ACPI memory hotplug support. Added memory needs to be onlined.
        *   **Windows Server:** Supports memory hot-add.
    *   **V-Architect Guest Tools (Conceptual):** Lightweight, optional guest tools could facilitate smoother hot-plug operations by:
        *   Automatically onlining added CPUs/memory.
        *   Notifying the V-Architect management plane about successful resource recognition by the guest.
        *   Providing a channel for the hypervisor to gracefully request CPU/memory offlining for revert operations.

**4. Resource Feasibility Check Logic (Google Gemini):**

    *   **Inputs to Gemini (via `CoreHypervisorService.GetHostCapabilities` and `VMService.GetVMStatus`):**
        *   Current total and available host resources (CPU load average, free memory, available CPU cores not heavily utilized).
        *   Target VM's current `VMConfig` (vCPU, RAM, storage/network QoS settings).
        *   Target VM's current resource utilization (average and peak for CPU, memory, I/O).
        *   Policy information (e.g., does user allow overcommitment for this feature?).
    *   **Output from Gemini (to V-Architect Management Plane):**
        *   **Feasibility Score/Boolean:** Can the "Double Specs" request be met fully, partially, or not at all?
        *   **List of Resources that Can Be Doubled:** (e.g., CPU: Yes, RAM: Yes, Storage IOPS: No - host limit reached).
        *   **Estimated Impact on Host:** (e.g., "Host CPU load will increase by X%", "Host free memory will drop to Y MB").
        *   **Recommended Adjustments:** (e.g., "Can double vCPUs from 2 to 4, and RAM from 4GB to 8GB. Storage IOPS currently at max host capability.").
    *   **Logic:** Gemini employs a model considering host headroom, VM's current utilization (doubling idle resources is less impactful than doubling already maxed-out ones), and potential impact on other running VMs (if in a shared host scenario).

**5. Dynamic QoS Adjustment Mechanisms (Storage/Network):**

    *   **Storage I/O (e.g., `virtio-blk`, NVMe):**
        *   If the hypervisor backend supports I/O throttling per virtual disk (e.g., via cgroups I/O controller on Linux for file-backed disks, or LVM/storage array QoS features for block-backed), V-Architect will:
            *   Query current IOPS/throughput limits.
            *   Attempt to set new limits (e.g., 2x the current, up to a pre-defined maximum or what Gemini deems feasible for the host).
        *   This will be an update to the device parameters in the Core Engine.
    *   **Network I/O (e.g., `virtio-net`):**
        *   If the vSwitch/vRouter backend supports traffic shaping per vNIC (e.g., Linux `tc` with HTB/TBF, OVS QoS), V-Architect will:
            *   Query current bandwidth limits/priority.
            *   Attempt to set new limits or increase priority.
        *   This will be an update to the vNIC's port parameters on its attached vSwitch/vRouter.

**6. AI Hardware Scaling (Mediated Passthrough):**

    *   **Mechanism:** If a VM is using a mediated passthrough vAI-GPU or vNPU profile (e.g., NVIDIA vGPU).
    *   **Process:**
        1.  Gemini checks if larger/more capable profiles are available on the physical AI accelerator.
        2.  If yes, and feasible, V-Architect (via Core Engine) will request the vendor's vGPU/vNPU manager on the host to:
            *   Detach the current smaller profile from the VM (may require brief quiesce of AI tasks).
            *   Attach a new, larger profile to the VM.
        *   This is highly dependent on the vendor's software capabilities for dynamic profile changes. A full detach/re-attach might be required, which could be disruptive. A less disruptive approach might be to allocate more time-slices or compute units to the existing profile if the vendor manager supports it.
    *   **Passthrough AI Hardware:** "Doubling" is not applicable as the VM already has full access.

**7. Revert Operation:**
    *   The feature must allow reverting to the original specifications.
    *   This involves calling `HotUnplugVCPU`, `HotRemoveMemory` (or ballooning down), and resetting QoS parameters to their previous values.
    *   Reverting AI hardware profiles would follow the reverse of the scaling-up process.

**Initial Implementation Considerations:**
*   Focus initially on CPU and Memory hot-add for Linux guests with guest tools to automate onlining.
*   Implement basic Gemini feasibility check based on host CPU/Memory availability.
*   Dynamic QoS for storage/network can be a subsequent enhancement, starting with simple limit adjustments.
*   AI hardware scaling is the most complex and vendor-dependent; start with monitoring and feasibility assessment.
*   Ensure robust error handling and rollback if any part of the "doubling" process fails.

[end of technical_specifications/phase1_core_virtualization_engine.md]
