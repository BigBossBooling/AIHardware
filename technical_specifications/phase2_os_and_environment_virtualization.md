# Phase 2: Operating System & Environment Virtualization - Detailed Technical Specifications

## Introduction

This document provides the detailed technical specifications for Phase 2: Operating System & Environment Virtualization of the V-Architect project. It builds upon the core virtualization engine and virtual hardware components specified in `technical_specifications/phase1_core_virtualization_engine.md` and translates the conceptual designs outlined in `conceptual_designs/v_architect_conceptual_blueprint.md` (specifically "Phase 2: Operating System & Environment Virtualization - Sculpting AI-Enhanced Digital Realities") into actionable technical details for developers.

The goal of this specification is to define the precise mechanisms, APIs, data structures, and AI integration points necessary to implement features that enable the hosting and management of diverse operating systems and server environments. This includes OS virtualization with AI-optimized deployment, bare-metal VM provisioning with AI guidance, server virtualization capabilities (including containerization and AI-managed clusters), VM snapshots and clones, advanced virtual network topology management, and AI-driven predictive resource management for these environments.

## II. OS Virtualization & Provisioning - Technical Specifications

This section details the technical specifications for virtualizing various operating systems, managing installation media, and integrating AI for optimized and guided deployment processes.

### A. OS Virtualization & AI-Optimized Deployment - Technical Details

This subsection specifies the mechanisms for supporting diverse guest operating systems, managing their installation media, and leveraging AI (Google Gemini) to streamline their deployment.

**1. Supported OS List & Metadata Store:**

    *   **Format & Location:** A JSON file, e.g., `supported_os_metadata.json`, will be maintained within the V-Architect application data or bundled with releases. This file will be versioned.
    *   **Schema for `supported_os_metadata.json` (Illustrative):**
        ```json
        [
          {
            "os_id": "ubuntu_server_22.04_x64",
            "name": "Ubuntu Server 22.04 LTS (Jammy Jellyfish)",
            "vendor": "Canonical",
            "family": "Linux",
            "version": "22.04",
            "architecture": "x86-64",
            "default_vm_config_template": { // Subset of VMConfig from Phase 1
              "vcpu_config": { "count": 2 },
              "vram_config": { "size_mb": 2048 },
              "storage_devices": [
                { "disk_id": "root_disk", "controller_type": "virtio-blk", "size_gb": 20, "is_boot_disk": true, "format": "qcow2" }
              ],
              "network_interfaces": [
                { "nic_id": "nic0", "vnic_model": "virtio-net", "network_attachment": "default_nat" }
              ],
              "graphics_config": { "type": "vga_compatible" } // Serial console often preferred for servers
            },
            "installation_media_suggestions": [
              { "type": "iso_url", "url": "https://releases.ubuntu.com/22.04/ubuntu-22.04.3-live-server-amd64.iso", "checksum_sha256": "EXPECTED_CHECKSUM_HERE" }
            ],
            "virtio_driver_info": {
              "windows": null, // Not applicable
              "linux": "kernel_native" // Or specify package names if needed for older distros
            },
            "guest_tools_recommendation": "Install V-Architect Guest Tools for optimal performance and integration."
          },
          {
            "os_id": "windows_11_x64_pro",
            "name": "Windows 11 Pro",
            "vendor": "Microsoft",
            "family": "Windows",
            "version": "11 (23H2)", // Example version
            "architecture": "x86-64",
            "default_vm_config_template": {
              "vcpu_config": { "count": 2, "topology": {"sockets": 1, "cores_per_socket": 2, "threads_per_core": 1} }, // Example, ensure compatibility
              "vram_config": { "size_mb": 4096 },
              "storage_devices": [
                { "disk_id": "os_disk", "controller_type": "nvme", "size_gb": 64, "is_boot_disk": true, "format": "qcow2" }
              ],
              "network_interfaces": [
                { "nic_id": "nic0", "vnic_model": "virtio-net" }
              ],
              "graphics_config": { "type": "virtio-gpu" }, // Or mediated vGPU if available/selected
              "firmware_type": "uefi", // Windows 11 requires UEFI and Secure Boot
              "secure_boot_enabled": true,
              "virtual_tpm_enabled": true // Windows 11 requires TPM 2.0
            },
            "installation_media_suggestions": [], // User must provide their own media
            "virtio_driver_info": {
              "windows": {
                "iso_url": "URL_TO_STABLE_VIRTIO_WIN_ISO", // e.g., Fedora's VirtIO ISO
                "instructions": "Attach VirtIO drivers ISO during Windows setup to load storage and network drivers."
              },
              "linux": null
            },
            "guest_tools_recommendation": "Install V-Architect Guest Tools for VirtIO drivers, clipboard sharing, and dynamic resolution."
          }
          // ... other OS entries, including macOS (with EULA caveats) and ChromeOS Flex
        ]
        ```
    *   **Updating Metadata:** This file can be updated with new V-Architect releases or potentially via a secure online update mechanism for V-Architect itself.

**2. ISO/Installation Media Management - APIs & Logic:**

    *   **Desktop Orchestration Layer (DOL) Functionality:**
        *   The DOL UI will allow users to:
            *   Maintain a library of local ISO file paths.
            *   Add ISOs by browsing the local filesystem or providing a URL.
            *   For URLs, the DOL can download the ISO to a local cache, verifying checksums if provided in `supported_os_metadata.json`.
            *   Select an ISO from the library or a direct URL when creating/configuring a VM.
    *   **Core Engine API for Media Attachment (Extending `VMService` from Phase 1):**
        ```protobuf
        message VMMediaAttachment {
          string drive_id = 1;      // e.g., "ide0-0-0" (IDE bus 0, master, LUN 0)
          string media_path = 2;    // Host path to the ISO file
          bool is_cdrom = 3;        // True for CD-ROM, false for floppy/USB image
          bool read_only = 4;       // Typically true for ISOs
        }

        message ManageVMMediaRequest {
          string vm_id = 1;
          VMMediaAttachment attachment = 2; // Provide to attach
          string drive_id_to_eject = 3;     // Provide to eject
        }

        message ManageVMMediaResponse {
          enum Status { SUCCESS = 0; FAILED = 1; NOT_FOUND = 2; }
          Status status = 1;
          string message = 2;
        }
        // This RPC was already defined in Phase 1 API stubs, here we detail its usage.
        ```
    *   **Core Engine Logic:**
        *   When `ManageVMMediaRequest` is received to attach an ISO, the Core Engine configures the specified VM's emulated IDE/SATA controller (from Phase 1 specs) to map the `media_path` to the virtual CD-ROM drive.
        *   The `VMConfig` for the VM will be updated to reflect the attached media.

**3. AI-Optimized Deployment (Google Gemini Integration Points - DOL & Core Engine interaction):**

    *   **A. VM Configuration Suggestion (DOL-Side Gemini Integration):**
        *   **Trigger:** User selects an OS from a list (populated from `supported_os_metadata.json`) or an ISO file (DOL attempts to identify OS from ISO name/metadata).
        *   **DOL Action:** DOL sends the identified `os_id` (or characteristics) and current `GetHostCapabilitiesResponse` to a Gemini model/service.
        *   **Gemini Logic:** Gemini looks up `default_vm_config_template` from OS metadata. It then refines this template based on host capabilities (e.g., if host has many cores, suggest more vCPUs than minimal default; if specific AI accelerators are present and OS/workload implies AI use, suggest attaching them). It also considers any user-stated workload intent (e.g., "gaming," "development").
        *   **DOL UI:** Presents Gemini's suggested `VMConfig` to the user, who can then accept or customize it.
    *   **B. Driver Assistance - Post OS Installation (DOL with Guest Tools & Gemini):**
        *   **V-Architect Guest Tools (Lightweight & Optional):**
            *   Installed inside the guest VM by the user (or via Automated Setup Wizard - Phase 4).
            *   **Functionality:** Securely report key guest information to the DOL via a paravirtualized channel (e.g., a dedicated VirtIO serial port or custom device). Reported info includes:
                *   Detected OS version.
                *   Status of VirtIO drivers (e.g., "virtio_net loaded", "virtio_blk not detected").
                *   Presence of V-Architect guest tools themselves.
        *   **DOL Action & Gemini Logic:**
            *   DOL receives guest tools report. If VirtIO drivers are missing/suboptimal for the `os_id`, DOL queries Gemini.
            *   Gemini uses `supported_os_metadata.json.virtio_driver_info` (which may contain URLs to driver ISOs like VirtIO-Win ISO, or package names for Linux) and the reported guest OS state.
            *   Gemini formulates a recommendation:
                *   "For optimal performance with your Windows 11 VM, VirtIO drivers are recommended. Would you like to attach the VirtIO Windows driver ISO and view instructions?"
                *   "For your Ubuntu VM, ensure the 'linux-kvm' kernel package (or equivalent) is installed for latest VirtIO drivers."
        *   **DOL UI Action:** Presents Gemini's recommendation. If user accepts to attach driver ISO, DOL uses `ManageVMMedia` RPC. Instructions can be shown in a UI panel.
    *   **C. AI-Driven Image Optimization & Natural Language Guidance (Conceptual - Future):**
        *   These remain conceptual as per the blueprint, with Gemini integration points at the DOL level for UI/interaction and potentially Core Engine for advanced image processing if ever implemented.

**Initial Implementation Considerations:**
*   Develop a well-structured `supported_os_metadata.json` with initial entries for popular Linux distros and Windows.
*   Implement robust ISO library management and download/caching in the Desktop Orchestration Layer.
*   Focus Gemini integration first on VM configuration suggestions based on OS selection and host capabilities.
*   Develop basic V-Architect Guest Tools for Linux and Windows to report VirtIO driver status.
*   Gemini driver assistance should initially focus on providing correct VirtIO driver ISOs/package names and clear instructions.

### B. Bare-Metal VM Provisioning & AI-Guidance - Technical Details

This subsection specifies the technical details for provisioning a VM with only virtual hardware defined (allowing users to install an OS from scratch) and how AI (Google Gemini) provides guidance during this manual process.

**1. Process Flow (User Interaction via Desktop Orchestration Layer - DOL):**

    *   **Step 1: Virtual Hardware Configuration:**
        *   User initiates "Create Custom VM" (or similar) workflow in the DOL UI.
        *   User manually configures or uses Gemini's assistance (from Phase 4A.2 - Virtual Hardware Recommendations) to define the full `VMConfig` (vCPU, vRAM, storage devices, network interfaces, graphics, AI accelerators, etc., as per Phase 1 Technical Specifications).
        *   A `vm_id` and `vm_name` are assigned.
    *   **Step 2: Installation Media Attachment:**
        *   DOL UI prompts user to attach OS installation media.
        *   User selects a local ISO file or provides a URL (DOL downloads to cache, as specified in II.A.2).
        *   The selected media path is associated with a virtual CD-ROM device (e.g., `ide0-0-0`) in the `VMConfig`. The `boot_order` in `VMConfig` is set to prioritize this virtual CD-ROM.
    *   **Step 3: VM Creation & Power-On:**
        *   DOL sends `CreateVMRequest` with the complete `VMConfig` (including media attachment) to the Core Engine's `VMService`.
        *   Upon successful creation, DOL sends `StartVMRequest`.
    *   **Step 4: OS Installation via Console:**
        *   DOL UI provides a console view for the VM (e.g., using a SPICE or noVNC client connected to the Core Engine's graphics framebuffer endpoint for that VM, or serial console).
        *   User interacts directly with the OS installer within the console.
    *   **Step 5: AI Guidance Panel (DOL UI):**
        *   A dedicated, non-intrusive panel or chat-like interface within the DOL UI provides access to Gemini's guidance. This panel is *external* to the VM console.

**2. AI Guidance - Gemini Integration (via DOL):**

    *   **Knowledge Base for OS Installation (Conceptual Structure):**
        *   Gemini will access a knowledge base (KB). This KB is distinct from `supported_os_metadata.json` but may be cross-referenced.
        *   **Content:**
            *   Common installation steps and decision points for various OS families (Windows, major Linux distros like Debian/Ubuntu, RHEL/Fedora, Arch).
            *   Troubleshooting tips for common installation errors (e.g., "disk not found," "network not detected," "bootloader issues").
            *   Driver loading procedures:
                *   Specific instructions for loading VirtIO storage/network drivers during installation for different OSs (e.g., Windows "Load Driver" from VirtIO-Win ISO, Linux kernel parameters for specific VirtIO modules if not auto-detected).
                *   Information on where to find vendor GPU drivers post-installation.
            *   Basic post-installation configuration (e.g., setting up networking, installing guest tools, common security updates).
        *   **Format:** Could be a structured database, a collection of markdown documents with semantic tagging, or a fine-tuned LLM model trained on installation guides and troubleshooting forums.
        *   **Maintenance:** Requires regular updates to cover new OS versions and common issues.
    *   **Triggering AI Guidance:**
        *   **User Queries:** The primary trigger. User types questions into the AI Guidance Panel (e.g., "My Windows installer doesn't see any disks," "How to partition disk for Ubuntu server?"). DOL sends query to Gemini.
        *   **Heuristic State Detection (Conceptual - DOL side, very limited):**
            *   The DOL *could* attempt very basic, non-intrusive heuristic detection of common issues by observing VM state changes *externally* (e.g., if a VM reboots multiple times and always tries to PXE boot, it might suggest checking boot order or installation media).
            *   **Important:** No direct introspection into the guest OS installer environment is performed by Gemini or DOL during this phase to maintain security and avoid interference. Guidance is based on user queries and general knowledge.
    *   **Gemini's Response Generation:**
        *   Gemini processes user query against its OS Installation KB.
        *   Provides step-by-step instructions, troubleshooting suggestions, or links to relevant sections of the KB or external official documentation.
        *   Responses are displayed in the AI Guidance Panel in the DOL UI.
    *   **Example Interaction Flow (Disk Not Found):**
        1.  User installing Windows encounters "No drives were found" error in installer.
        2.  User types in AI Guidance Panel: "Windows setup no disk".
        3.  DOL sends query to Gemini.
        4.  Gemini accesses KB, finds common cause is missing VirtIO storage driver for Windows.
        5.  Gemini responds in UI: "Windows Setup might need VirtIO storage drivers. 1. Ensure the VirtIO-Win ISO is attached as a second virtual CD-ROM. 2. Click 'Load driver' in the Windows installer. 3. Browse to the VirtIO SCSI/Block driver folder on the VirtIO-Win CD for your architecture (amd64). Would you like me to attach the latest VirtIO-Win ISO for you?"
        6.  If user agrees, DOL uses `ManageVMMedia` to attach the driver ISO.

**3. Post-OS Installation Guidance (Leveraging Guest Tools - if installed):**

    *   Once the OS is installed and (optionally) V-Architect Guest Tools are running (as per II.A.3.B), Gemini can offer more specific post-installation guidance via the DOL UI based on telemetry from the tools (e.g., "VirtIO network driver is active. Consider installing [common application] for your [OS type] server.").

**Initial Implementation Considerations:**
*   Focus the AI Guidance KB initially on 2-3 popular Linux distributions and the latest Windows version.
*   Prioritize driver loading assistance for VirtIO storage and network as these are common pain points.
*   The AI Guidance Panel should clearly indicate that Gemini's advice is external and based on general knowledge unless guest tools are active.
*   Heuristic state detection should be conservative to avoid incorrect or annoying suggestions.

## III. Server Virtualization & Management - Technical Specifications

This section provides the detailed technical specifications for V-Architect's server virtualization capabilities, including containerization support, virtual server management, and AI-managed VM clustering.

### A. Containerization Support - Technical Details

V-Architect will provide robust support for running containerized workloads, primarily by enabling VMs to act as efficient container hosts, with AI (Google Gemini) assistance for optimization.

**1. VMs as Container Hosts:**

    *   **Optimized VM Templates:**
        *   V-Architect will offer predefined `VMConfig` templates specifically optimized for running container workloads. These templates will be accessible via the Desktop Orchestration Layer (DOL) UI and referenced in `supported_os_metadata.json`.
        *   **"Docker-Optimized" Template Example:**
            *   **OS:** A minimal, security-hardened Linux distribution (e.g., Alpine Linux, Fedora CoreOS, or a custom V-Architect minimal Linux build).
            *   **Kernel:** Configured with necessary features for container runtimes (e.g., cgroups, namespaces, overlayfs).
            *   **Software:** Pre-installed Docker engine (`docker-ce` or `moby-engine`) and `containerd`.
            *   **VM Resources:** Defaults might include 2 vCPUs, 4GB RAM, a moderately sized `virtio-blk` disk for images/volumes, and `virtio-net`.
            *   **Guest Tools:** V-Architect Guest Tools pre-installed for telemetry and management.
        *   **"Kubernetes-Node-Optimized" Template Example:**
            *   Similar minimal OS and kernel as Docker-Optimized.
            *   Pre-installed `containerd` (or other CRI-compatible runtime), `kubelet`, `kubeadm` (or k3s/k0s components).
            *   Networking configured for Kubernetes pod networking (e.g., CNI plugin requirements met by the host or vSwitch configuration).
            *   Specific kernel modules enabled (e.g., `br_netfilter`).
    *   **Customization:** Users can customize these templates or build their own container host VMs using Bare-Metal Provisioning (Section II.B).
    *   **V-Architect Guest Tools Reporting:**
        *   Guest Tools running on a container host VM will report:
            *   Presence and version of Docker engine / Kubernetes components.
            *   Number of running containers (basic count).
            *   Overall resource utilization by the container runtime (if accessible without deep introspection).
        *   This telemetry is sent to the V-Architect monitoring service for use by the DOL and Gemini.

**2. AI-Optimization for Resource Packing & Scheduling (Google Gemini):**

    *   **Telemetry Requirements for Gemini:**
        *   From Container Host VMs (via Guest Tools or hypervisor): Overall CPU/memory/disk/network load. Number of active containers.
        *   (Conceptual - Advanced) If deeper integration with Docker/Kubernetes APIs within the guest is possible via a secure V-Architect agent: Per-container resource requests, limits, and actual usage. This is complex and has security implications.
    *   **Gemini's Optimization Logic (DOL-Side or Cluster Management Service):**
        *   **VM Sizing Recommendations:** Based on the number and type of containers a user intends to run (if specified), Gemini can recommend an optimal size for the container host VM (vCPUs, RAM).
        *   **Container Host Placement (in a V-Architect Cluster):** If multiple container host VMs are being deployed in a cluster, Gemini can recommend which physical hosts to place them on to balance overall load or to consolidate for power saving, considering the aggregate resource needs of the containers they are expected to run.
        *   **Resource Packing Advice (Conceptual):** If per-container telemetry is available, Gemini could identify:
            *   Over-provisioned container host VMs that could be downsized.
            *   Opportunities to co-locate certain types of container workloads on the same VM for better resource utilization (e.g., packing I/O-bound containers with CPU-bound ones).
            *   This advice would be presented to the user; V-Architect would not directly manage individual containers within the VM initially.

**3. V-Architect Managed Containers (Conceptual - Future Direction):**

    *   **Rationale:** To provide a more integrated, higher-level abstraction for users who want to deploy applications as containers without managing the underlying host VMs directly.
    *   **Potential Architecture:**
        *   A V-Architect "Container Service" that exposes a Kubernetes-compatible API (subset) or a simplified V-Architect specific API for defining containerized application deployments (pods, services, volumes).
        *   This service would manage a pool of V-Architect VMs (based on the "Container-Optimized" templates) as worker nodes.
        *   It would schedule container pods onto these worker VMs, manage their lifecycle, networking (using V-Architect's virtual networking), and storage (using V-Architect's virtual disks for persistent volumes).
    *   **Technology Considerations:** Could leverage lightweight Kubernetes distributions like K3s or K0s embedded within V-Architect, or custom orchestration logic using `containerd` directly on worker VMs.
    *   **This is a significant future undertaking, not part of the initial Phase 2 specification beyond conceptual alignment.**

**Initial Implementation Considerations:**
*   Focus on providing well-defined "Docker-Optimized" and "Kubernetes-Node-Optimized" VM templates.
*   Implement basic Guest Tool reporting for container runtime presence and container count.
*   Gemini's initial role will be advisory for VM sizing and placement based on expected container load (user-provided estimates).
*   Direct container management by V-Architect is a post-MVP, advanced feature.

### B. Virtual Server Management - Technical Details

This subsection specifies the technical details for managing headless server VMs, providing remote console access, and integrating AI-driven monitoring for performance and anomaly detection.

**1. Headless Server VM Deployment:**

    *   **`VMConfig` Specification:**
        *   The `vm_config_schema.json` (Phase 1 Tech Spec) must fully support configurations that do not require a graphical display.
        *   `graphics_config.type` can be set to `"none"` or rely solely on an emulated serial port (e.g., `serial_port_config` enabling a virtual serial port).
        *   VM templates for server OSs (e.g., "Ubuntu Server," "Windows Server Core") in `supported_os_metadata.json` will default to headless configurations.
    *   **Core Engine API:** `VMService.CreateVM` and `VMService.StartVM` must correctly handle and instantiate VMs based on such headless `VMConfig`s.
    *   **Desktop Orchestration Layer (DOL) UI:** The UI must allow users to create and manage VMs without necessarily opening or expecting a graphical console window, relying instead on serial console access or network protocols like SSH/RDP.

**2. Remote Console Access:**

    *   **a. Secure Shell (SSH) for Linux/macOS Guests:**
        *   **Facilitation by DOL/Guest Tools:**
            *   During VM creation from a template or via the Automated Setup Wizard (Phase 4A), the DOL can offer to inject a user-provided public SSH key into the guest's `authorized_keys` file. This can be achieved via:
                *   `cloud-init` (if the guest image supports it and V-Architect provides a `user-data` mechanism).
                *   V-Architect Guest Tools running in the guest after first boot, receiving the key via a secure paravirtualized channel.
            *   The DOL UI should display the VM's IP address (obtained via guest tools or DHCP lease information from the vRouter/network) to help users connect.
        *   **No Direct SSH Proxying by V-Architect:** V-Architect itself will generally not act as an SSH proxy. Users will connect directly to the VM's IP address using their standard SSH clients.
    *   **b. Remote Desktop Protocol (RDP) for Windows Guests:**
        *   **Network Configuration:** The DOL must make it easy for users to configure network settings (e.g., Bridged mode, or NAT on a vRouter with RDP port forwarding rules) that allow RDP clients to reach Windows VMs. Gemini (AI Hardware Recommendations / Network Topology Management) can assist in suggesting these configurations.
        *   **Credential Management:** Users manage Windows credentials themselves. V-Architect does not store or inject Windows passwords.
        *   **Displaying IP Address:** Similar to SSH, the DOL UI will display the VM's IP for RDP connection.
    *   **c. Serial Console Access (Emulated Serial Port):**
        *   **Core Engine Emulation:** The Core Engine must provide emulation for at least one virtual serial port (e.g., compatible with a 16550A UART) per VM. This is standard in most VMMs.
        *   **Backend Exposure:** The Core Engine will expose this virtual serial port via:
            *   A Pseudo-Terminal (PTY) on Linux hosts.
            *   A named pipe on Windows hosts.
            *   A TCP socket (listening on localhost or a configurable interface) for broader accessibility.
        *   **DOL UI Access:** The DOL UI will provide a "Serial Console" option for each VM, which connects to the backend PTY/named pipe/TCP socket and displays the serial output in a terminal-like window.
        *   **`VMConfig`:** `serial_port_config` array within `VMConfig` to define one or more serial ports, their type (e.g., "pty", "tcp_socket"), and parameters (e.g., path, port number).

**3. AI-Driven Monitoring & Anomaly Detection (Google Gemini):**

    *   **Telemetry Collection (Technical Details):**
        *   **Source - Hypervisor/Core Engine:**
            *   Per-VM CPU utilization (e.g., `%vm_cpu_time`, `steal_time` if applicable).
            *   Per-VM memory usage (active vs. ballooned vs. swapped - if host swapping is involved for the VM's memory).
            *   Per-VM virtual disk I/O (read/write ops/sec, bytes/sec, latency - from `virtio-blk`/`nvme` backends).
            *   Per-VM virtual network I/O (packets/sec, bytes/sec, errors - from `virtio-net` backend).
            *   These metrics are collected by the Core Engine and exposed via a metrics RPC (e.g., `VMService.GetVMMetrics`).
        *   **Source - V-Architect Guest Tools (Optional, Opt-in):**
            *   More granular in-guest metrics:
                *   Per-process CPU/memory usage for top N processes.
                *   Guest filesystem usage (per mount point).
                *   Status of critical guest services/daemons (configurable list).
                *   Key security event log snippets (e.g., failed logins, new listening ports - requires careful filtering and privacy considerations).
            *   **Communication:** Guest tools send this data securely (e.g., TLS over a dedicated VirtIO channel or `virtio-serial` port) to a V-Architect Telemetry Aggregation Service running on the host or as part of the DOL/Cluster Manager.
        *   **Data Format:** Standardized format like Prometheus exposition format or OpenTelemetry.
        *   **Collection Interval:** Configurable, e.g., every 15-60 seconds.
    *   **V-Architect Telemetry Aggregation Service:**
        *   Receives telemetry from Core Engine and Guest Tools.
        *   Stores data in a time-series database (e.g., Prometheus, InfluxDB - as per Phase 2F).
    *   **Gemini Analysis Logic & API Interaction:**
        *   Gemini models (running as part of a central V-Architect AI service or within the DOL/Cluster Manager for local deployments) query the Telemetry Aggregation Service.
        *   **Baseline Establishment:** Gemini learns normal operating ranges for each VM's key metrics over time (e.g., typical CPU load during business hours).
        *   **Anomaly Detection Algorithms:**
            *   Statistical methods (e.g., 3-sigma rule, seasonal decomposition of time series - STL) for identifying significant deviations from baseline.
            *   Machine learning models (e.g., autoencoders, LSTMs trained on normal behavior) for detecting more complex, subtle anomalies.
        *   **Alert Generation:**
            *   If an anomaly is detected and persists or exceeds a confidence threshold, an alert is generated.
            *   **Alert Structure (Conceptual):** `vm_id`, `timestamp`, `severity` (INFO, WARNING, CRITICAL), `metric_name`, `observed_value`, `expected_range`, `anomaly_score`, `gemini_diagnosis_hypothesis` (string - e.g., "High CPU usage correlates with increased network errors on eth0, potential network storm or DoS on guest service X.").
            *   Alerts are sent to the DOL UI and potentially to external notification systems (email, PagerDuty - future integration).
    *   **User Interface (DOL):**
        *   Dashboards displaying key performance metrics for server VMs.
        *   Alert list with details and Gemini's diagnostic hypothesis.
        *   User feedback mechanism on alert accuracy.

**Initial Implementation Considerations:**
*   Prioritize robust serial console access.
*   Facilitate SSH key injection for Linux VMs using `cloud-init` if guest images support it.
*   Implement basic hypervisor-level telemetry collection for CPU, memory, disk, network.
*   Develop initial Gemini models for baselining and statistical anomaly detection on these core metrics.
*   Guest tools and more advanced in-guest telemetry/anomaly types are subsequent enhancements.

### C. VM Clustering & Orchestration (AI-Optimized) - Technical Details

This subsection specifies the technical details for V-Architect's VM clustering and orchestration capabilities, designed to provide High Availability (HA), load balancing, and AI-driven optimization for groups of VMs and hosts.

**1. V-Architect Cluster Management Service (VCMS):**

    *   **Architecture:** A distributed service. Each V-Architect host participating in a cluster will run a VCMS agent. One or more hosts may take on a leader/coordinator role (e.g., using a Raft consensus algorithm).
    *   **State Store:**
        *   Utilize a distributed key-value store for maintaining cluster state, host membership, VM-to-host assignments, HA policies, and live migration locks.
        *   **Technology:** Consider embedding `etcd` (using its client libraries) or a similar lightweight Raft-based consensus library directly within the VCMS agents.
    *   **Host Membership & Heartbeating:**
        *   Hosts joining a cluster register with the VCMS.
        *   VCMS agents send regular heartbeats to maintain membership and detect host failures. Missed heartbeats trigger a failure detection process.
    *   **Cluster API (Conceptual - could be part of `CoreHypervisorService` or a new `ClusterService` gRPC definition):**
        ```protobuf
        service ClusterService {
          rpc JoinCluster(JoinClusterRequest) returns (JoinClusterResponse); // Host requests to join
          rpc LeaveCluster(LeaveClusterRequest) returns (LeaveClusterResponse); // Host gracefully leaves
          rpc GetClusterStatus(GetClusterStatusRequest) returns (GetClusterStatusResponse); // Overall cluster health, hosts, resources
          rpc SetVMHAPolicy(SetVMHAPolicyRequest) returns (SetVMHAPolicyResponse);
          rpc GetVMHAPolicy(GetVMHAPolicyRequest) returns (GetVMHAPolicyResponse);
          // RPCs for managing shared storage pools for the cluster could also reside here
        }

        message HostInfo {
          string host_id = 1;
          string host_address = 2; // For inter-host communication
          HostCapabilities capabilities = 3; // From Phase 1 GetHostCapabilitiesResponse
          enum HostStatus { ONLINE = 0; OFFLINE = 1; DEGRADED = 2; MAINTENANCE = 3; }
          HostStatus status = 4;
        }

        message GetClusterStatusResponse {
          repeated HostInfo hosts = 1;
          // ... other cluster-wide info
        }

        message SetVMHAPolicyRequest {
          string vm_id = 1;
          bool ha_enabled = 2; // Enable/disable HA for this VM
          uint32 restart_priority = 3; // Lower number = higher priority for restart
          // ... other policy options e.g., preferred failover hosts
        }
        ```

**2. High-Availability (HA) for VMs:**

    *   **Shared Storage Prerequisite:** Robust HA requires VM virtual disks to reside on shared storage accessible by all hosts in the cluster (e.g., NFS, iSCSI LUNs, GlusterFS, Ceph RBD). The Core Engine (Phase 1 Storage Spec) must support these.
    *   **Failure Detection:**
        *   VCMS detects host failure via missed heartbeats and consensus among remaining nodes.
        *   VCMS can also monitor VM health (e.g., guest tools responsiveness, application-level heartbeats if configured) to detect VM-specific failures even if the host is up.
    *   **Failover Logic (Orchestrated by VCMS leader):**
        1.  Declare failed host/VM as down.
        2.  Release any resource locks held by the failed host/VM (e.g., disk locks on shared storage).
        3.  Identify a suitable failover host from the available pool based on:
            *   VM's HA policy (restart priority, preferred hosts).
            *   Resource availability on potential target hosts (CPU, RAM, network capacity, AI accelerator availability if required by VM). **Gemini input is crucial here for optimal placement.**
            *   Anti-affinity rules (e.g., don't restart on a host already running a redundant peer of this VM).
        4.  Instruct the chosen target host's Core Engine (`VMService`) to:
            *   Re-attach the VM's shared storage disks.
            *   Start the VM using its existing `VMConfig`.
        5.  Update network configurations (e.g., ARP, DNS) if the VM's IP is now active on a new host MAC address.
    *   **VM Restart Priority:** `SetVMHAPolicyRequest.restart_priority` determines the order in which VMs are restarted during a mass failover event if resources are constrained.

**3. Intelligent Load Balancing (Orchestrated by VCMS with Gemini):**

    *   **Goal:** Distribute VMs across cluster hosts to optimize resource utilization, prevent hotspots, and maintain performance SLAs.
    *   **Telemetry Inputs to Gemini:**
        *   Per-host resource utilization (CPU, memory, network I/O, disk I/O - from `GetHostCapabilitiesResponse` and ongoing monitoring).
        *   Per-VM resource utilization (from `GetVMMetrics` and guest tools).
        *   Network latency and bandwidth between hosts in the cluster.
        *   User-defined load balancing policies (e.g., "spread for performance," "pack for power saving," "affinity groups").
    *   **Gemini's Logic & Recommendations:**
        *   Gemini analyzes cluster-wide telemetry and policies.
        *   Identifies imbalances (e.g., one host consistently over 80% CPU while others are at 30%).
        *   Recommends VM migrations (source VM, source host, destination host) to the VCMS to achieve better balance.
        *   For "pack for power saving," Gemini might recommend consolidating VMs onto fewer hosts and powering down idle hosts (requires Wake-on-LAN or similar for powered-down hosts).
    *   **VCMS Action:** Receives migration recommendations from Gemini and uses the Core Engine's Live Migration API (`VMService.MigrateVM` - Phase 1 Tech Spec) to execute them, respecting HA policies and maintenance windows.

**4. Predictive Failure Resilience (Gemini & VCMS):**

    *   **Telemetry Inputs to Gemini (Extending Server Monitoring - Section III.B.3):**
        *   Host hardware sensor data (SMART for disks, CPU/RAM temperatures, fan speeds - if accessible to VCMS agent via host OS).
        *   Hypervisor error logs (e.g., KVM warnings, IOMMU faults).
        *   VM error rates (e.g., excessive disk read/write errors reported by guest tools or `virtio-blk` backend, frequent guest OS crashes).
        *   Patterns of resource consumption that historically precede failures.
    *   **Gemini's Predictive Model:**
        *   ML models trained to identify patterns indicative of impending host hardware failures, storage failures, or VM instability.
    *   **Proactive Action (Recommendation to VCMS):**
        *   If Gemini predicts a high probability of failure for a host or critical VM component:
            *   It alerts administrators via the DOL UI.
            *   It recommends proactively live migrating critical VMs off the potentially failing host.
            *   For critical VMs, if policy allows, VCMS might automatically initiate such preventative migrations.
    *   **Automated Failover Management:** In case of an unpredicted, actual failure, the standard HA failover logic (Point 2 above) is triggered by VCMS. Gemini's role here is primarily in optimizing placement of the restarted VMs.

**Initial Implementation Considerations:**
*   Start with a simplified VCMS focusing on host membership and heartbeating.
*   Implement HA for VMs on shared storage (NFS) as the first HA capability.
*   Basic load balancing can be policy-driven (e.g., round-robin placement, prevent host CPU > X%) before full Gemini integration.
*   Predictive failure resilience is an advanced feature; initial focus should be on robust reactive HA.
*   Ensure secure communication between VCMS agents and for the distributed state store.
