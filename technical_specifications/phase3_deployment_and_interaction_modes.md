# Phase 3: Deployment & Interaction Modes - Detailed Technical Specifications

## Introduction

This document provides the detailed technical specifications for Phase 3: Deployment & Interaction Modes of the V-Architect project. It builds upon the core virtualization engine (Phase 1 Tech Specs) and the OS/environment virtualization capabilities (Phase 2 Tech Specs). This document translates the conceptual designs outlined in `conceptual_designs/v_architect_conceptual_blueprint.md` (specifically "Phase 3: Deployment & Interaction Modes - The Universal Canvas Unites") into actionable technical details for developers.

The goal of this specification is to define the precise mechanisms, APIs, data structures, security enforcement points, and AI integration details necessary to implement how users interact with and deploy their virtualized environments. This includes specifications for local client mode, secure sandboxing, conceptual distributed/remote execution, robust VM isolation with vTPM support, AI-enhanced security monitoring and auditing, AI-configured and verified security policies, and the overarching V-Architect trust model.

## II. Deployment Modes - Technical Specifications

This section details the technical specifications for the various modes in which users can deploy and interact with their virtual machines within V-Architect.

### A. Local Client Mode - Technical Details

This subsection specifies the technical details for the standard Local Client Mode, where VMs run directly on the user's personal machine, managed by the Desktop Orchestration Layer (DOL) interacting with the Core Engine.

**1. DOL Interaction with Core Engine (Local Instance):**

    *   **API Usage:** The DOL acts as a primary client to the Core Engine's gRPC APIs (`CoreHypervisorService` and `VMService`) as defined in Phase 1 Technical Specifications (Section II.A).
    *   **Communication Protocol:** gRPC over local IPC (Unix domain socket on Linux/macOS, named pipe on Windows) as specified in Phase 1 Technical Specifications (Section II.B).
    *   **VM Lifecycle Management:**
        *   **Creation:** DOL collects user input (or uses templates/AI suggestions) to construct the `VMConfig` JSON object. It then calls `VMService.CreateVM(CreateVMRequest)` with this configuration.
        *   **Starting/Stopping/Pausing/Resuming:** DOL issues the corresponding `StartVM`, `StopVM`, `PauseVM`, `ResumeVM` RPCs to the Core Engine.
        *   **Configuration Updates:** DOL uses `VMService.UpdateVMConfiguration` for changes that can be applied to a running or stopped VM (e.g., attaching media, changing boot order, some hot-plug operations).
        *   **Deletion:** DOL uses `VMService.DeleteVM`.
    *   **Host Capability Awareness:** DOL calls `CoreHypervisorService.GetHostCapabilities` at startup and potentially periodically to understand available resources (CPUs, RAM, GPUs for passthrough, AI accelerators, storage paths) and to inform user choices or AI recommendations.
    *   **VM Status Monitoring:** DOL calls `VMService.GetVMStatus` and potentially `VMService.GetVMMetrics` (defined in Phase 2 Server Management) to display real-time VM status (running, paused, CPU/memory usage) in its UI.

**2. User Interface (UI) Considerations (Desktop Orchestration Layer):**

    *   **VM Dashboard:** A central view listing all local VMs, their status, basic configuration (vCPU, RAM), and quick action buttons (Start, Stop, Console, Settings).
    *   **VM Creation Wizard:** Guides users through creating new VMs, integrating AI recommendations for OS selection and hardware configuration (as per Phase 2 & 4 specs).
    *   **VM Settings/Configuration Panel:** Allows detailed viewing and modification of a VM's `VMConfig` parameters. Changes here are translated by DOL into appropriate `UpdateVMConfiguration` calls.
    *   **Console Access:**
        *   **Graphical Console:** For VMs with emulated graphics (`stdvga`, `virtio-gpu`) or vGPU, the DOL will embed or launch a display client. This client connects to the framebuffer/display endpoint exposed by the Core Engine for that VM (e.g., a SPICE or VNC server endpoint per VM, or a shared memory framebuffer). The choice of display protocol (SPICE, RDP-like, custom WebRTC) needs to be specified in Phase 1 vGPU/Graphics spec.
        *   **Serial Console:** As specified in Phase 2 (Sec III.B.2.c), DOL provides a terminal window connected to the PTY/named pipe/TCP socket for the VM's virtual serial port.
    *   **Resource Monitoring Display:**
        *   DOL UI should display host-level CPU and memory usage (from OS APIs).
        *   Per-VM CPU, memory, basic disk/network I/O usage obtained via `GetVMMetrics` from the Core Engine.
    *   **Hardware Passthrough Configuration:** UI elements to list available host PCIe/USB devices (from `GetHostCapabilitiesResponse`) and allow users to assign them to VMs (updating `VMConfig`). Clear warnings about driver requirements and potential host instability.
    *   **ISO & Media Management:** UI for managing the local ISO library and attaching media to VMs (using `ManageVMMedia` RPC).

**3. Local Resource Management:**

    *   The Core Engine is responsible for managing resource allocation (CPU scheduling, memory mapping, I/O prioritization) between running local VMs and the host OS, as detailed in Phase 1 and Phase 2 (Resource Management sections).
    *   The DOL's role is primarily to present resource usage information to the user and to pass configuration settings (priorities, limits from `VMConfig`) to the Core Engine.

**Initial Implementation Considerations:**
*   Robust gRPC client implementation within the DOL for all relevant `VMService` and `CoreHypervisorService` RPCs.
*   A clear and responsive VM list/dashboard in the UI.
*   Reliable graphical console display; SPICE is a mature option to consider for initial implementation.
*   Seamless VM creation and configuration workflow.

### B. Sandbox/Virtual Environment Mode - Technical Details

This subsection specifies the technical details for V-Architect's Sandbox/Virtual Environment Mode, designed for running untrusted applications or performing experiments in a highly isolated and disposable manner.

**1. Policy Enforcement for Enhanced Isolation:**

    *   **Default Sandbox Policy Set:** V-Architect will include a predefined, strict "Default Sandbox Policy" that is applied to VMs launched in this mode. This policy will be a specific instance of the Security Policies schema (defined in Section III.C of this Phase 3 Tech Spec).
    *   **Key Policy Restrictions Applied via `VMConfig` / Core Engine:**
        *   **Network Isolation:**
            *   Default: No network interfaces configured in `VMConfig`, or a `network_attachment` to a special "isolated_vswitch" that has no uplink and no inter-VM communication allowed by default.
            *   Optional: "NAT-only egress" via a dedicated, restricted vRouter instance if minimal internet access is needed (e.g., for downloading updates to a sandboxed tool). This vRouter would have strict egress filtering.
        *   **File System Access:**
            *   No host shared folders configured in `VMConfig` by default.
            *   If `virtio-fs` or similar is ever supported for host directory sharing, it would be explicitly disabled by policy for sandboxed VMs.
        *   **Hardware Passthrough:**
            *   `usb_devices_passthrough` and `pcie_devices_passthrough` in `VMConfig` will be empty by default.
            *   Policy may prevent DOL from even offering passthrough options for VMs in Sandbox Mode.
        *   **Clipboard Sharing:** Disabled by default (controlled via DOL and potentially guest tools interaction).
        *   **Device Access:** Minimal set of emulated devices. For example, no sound card emulation unless explicitly enabled for a specific sandbox profile.
    *   **Core Engine Enforcement:** The Core Engine must rigorously enforce these restricted `VMConfig` settings and associated runtime behaviors.

**2. Resource Capping Implementation:**

    *   **`VMConfig` Limits:** The "Default Sandbox Policy" or user-customized sandbox profiles will recommend/enforce stricter default limits in `VMConfig`:
        *   `vcpu_config.count`: e.g., default to 1 or 2 vCPUs.
        *   `vram_config.size_mb`: e.g., default to a modest amount like 1024MB or 2048MB.
        *   Storage device `size_gb`: Default to a small disk size.
    *   **Hypervisor Enforcement (Core Engine):**
        *   **CPU:** The Core Engine's CPU scheduler will enforce limits, potentially using cgroup CPU quotas for the VM process on Linux hosts.
        *   **Memory:** The allocated `size_mb` is a hard limit. `virtio-balloon` can be disabled by policy or its max inflation capped to prevent abuse.
        *   **Disk I/O:** IOPS/throughput throttling (as per Phase 1 Storage Spec) can be applied with conservative defaults for sandboxed VMs.
        *   **Network I/O:** Bandwidth limits (as per Phase 1 Networking Spec) applied if any network access is permitted.
    *   **Rationale:** Prevents a potentially malicious or runaway application in the sandbox from exhausting host resources and impacting other VMs or the host system.

**3. Disposable & Quick Reset Mechanism:**

    *   **Base Sandbox Images/Templates:**
        *   V-Architect will provide or allow users to create minimal "base sandbox images" (e.g., a minimal Linux with essential tools, or a clean Windows install). These are standard VM disk images (e.g., QCOW2).
        *   These base images are kept read-only.
    *   **Linked Clone for Sandbox Instances:**
        *   When a user starts a new sandbox session, V-Architect will rapidly create a **linked clone** (as specified in Phase 2 Tech Spec, Section IV.A) using the chosen base sandbox image as its backing file.
        *   This linked clone uses a differential disk (QCOW2 overlay) to store all changes made during the sandbox session.
    *   **Reset Operation:**
        *   "Resetting" a sandbox VM involves:
            1.  Powering off the VM (if running).
            2.  Deleting the differential disk (overlay file).
            3.  The VM is now effectively reverted to the clean state of its base image.
            4.  Optionally, a new empty differential disk is created if the user wants to restart the "same" (but reset) sandbox.
    *   **Disposable Operation:**
        *   "Disposing" or "Deleting" a sandbox VM involves deleting its differential disk and its `VMConfig`. The base read-only image remains untouched.
    *   **API Calls (Leveraging `VMService`):**
        *   `CloneVMRequest` with `CloneType.LINKED_CLONE` to create a new sandbox instance.
        *   `DeleteVMRequest` to dispose of a sandbox (which would include its differential disk).
        *   A higher-level DOL operation for "Reset" would internally orchestrate Delete + Re-Clone (or just delete overlay and restart with same config pointing to base).

**4. TEE Integration (Conceptual - Intel SGX, AMD SEV):**

    *   **Rationale:** To provide hardware-enforced memory encryption and integrity protection for the sandboxed VM, making it more resilient even if the hypervisor is compromised. This is a very advanced feature.
    *   **Core Engine Requirements (High-Level):**
        *   **Host Capability Detection:** `GetHostCapabilitiesResponse` must indicate if SGX/SEV (and specific variants like SEV-ES, SEV-SNP) are available and enabled on the host.
        *   **VM Configuration (`VMConfig`):** A new section, e.g., `tee_config`, with fields like:
            *   `tee_mode`: (enum: "none", "sgx_enclave", "sev_encrypted_vm", "sev_es_encrypted_state", "sev_snp_attested_vm").
            *   SGX-specific: `enclave_size_mb`, path to enclave definition files.
            *   SEV-specific: Guest attestation report parameters.
        *   **Hypervisor Interaction with Hardware:** The Core Engine would need to use specific KVM ioctls or platform APIs to:
            *   Launch an SEV-encrypted VM (e.g., setting C-bit in page tables, managing guest encryption keys via PSP).
            *   Manage SGX enclaves (if V-Architect itself were to run parts of its logic in an enclave to protect a VM, or if a guest application uses SGX directly - less common for whole-VM sandboxing).
    *   **Guest OS Support:** Requires specific guest OS support for SEV (e.g., kernel patches, specific boot process) or for applications to use SGX.
    *   **Initial Implementation:** Due to complexity, TEE integration is a research item for far-future V-Architect versions. Initial sandboxing will rely on hypervisor software isolation and standard hardware virtualization features.

**Initial Implementation Considerations:**
*   Focus on strong policy enforcement using `VMConfig` parameters (network, device, resource limits).
*   Implement the linked clone mechanism using QCOW2 for fast sandbox creation and reset.
*   Ensure the DOL UI clearly distinguishes Sandbox Mode and makes reset/dispose operations intuitive.
*   Defer TEE integration to later research phases.

### C. Distributed/Remote Execution Mode - Technical Details (Conceptual)

This subsection outlines conceptual technical details for V-Architect's Distributed/Remote Execution Mode. This is a long-term visionary feature, and these specifications are high-level, focusing on requirements and interaction points rather than definitive protocols or APIs for initial implementation.

**1. V-Architect Inter-Instance Communication Protocol:**

    *   **Transport Protocol:** gRPC over TLS with mutual authentication (mTLS) using X.509 certificates for secure and authenticated communication between V-Architect instances (DOL or Core Engine acting as a client/server).
    *   **Service Definitions (Conceptual - extending existing services or new ones):**
        *   **Host Discovery Service:**
            *   `rpc RegisterRemoteHost(RegisterHostRequest) returns (RegisterHostResponse)`: A V-Architect instance makes itself discoverable to a trusted registry or peer network.
            *   `rpc DiscoverRemoteHosts(DiscoverHostsRequest) returns (DiscoverHostsResponse)`: A V-Architect instance queries for available remote hosts based on criteria (e.g., trust level, resource availability, geolocation hints).
        *   **Remote VM Management Service (extending `VMService` or new `RemoteVMService`):**
            *   `rpc DeployVMToRemoteHost(RemoteDeployVMRequest) returns (RemoteDeployVMResponse)`: Initiates deployment of a VM (`VMConfig` and disk images) to a target remote host.
            *   `rpc GetRemoteVMStatus(RemoteVMStatusRequest) returns (VMStatusResponse)`: Proxies `GetVMStatus` to a remote host.
            *   `rpc StreamRemoteVMConsole(RemoteVMConsoleRequest) returns (stream ConsoleDataChunk)`: For streaming display/input.
        *   **VM Image/State Transfer Service:**
            *   `rpc InitiateVMImagePush(ImagePushRequest) returns (ImagePushResponse)`: Source initiates pushing disk image(s) to destination.
            *   `rpc PullVMImage(ImagePullRequest) returns (stream ImageChunk)`: Destination pulls disk image(s).
            *   Mechanism for differential/delta transfer based on QCOW2 snapshots or `rsync`-like block comparison.
    *   **Authentication & Authorization:**
        *   Each V-Architect instance (or user managing it) would have its own identity (e.g., DID, X.509 certificate).
        *   Policies would define which instances/users can discover, deploy to, or manage VMs on other instances.

**2. AI Orchestration (Google Gemini - High-Level API Interaction via DOL/VCMS):**

    *   **Remote Host Selection (Input to Gemini):**
        *   User's deployment criteria (e.g., "lowest latency," "cheapest compute," "host with available NVIDIA A100," "trusted development group hosts").
        *   List of discovered remote V-Architect hosts (from Host Discovery Service) including their:
            *   `GetHostCapabilitiesResponse` data (CPU, RAM, available GPUs/AI accelerators, storage).
            *   Current load/utilization metrics (if hosts publish this).
            *   Network telemetry (latency, bandwidth from deploying host to remote host).
            *   Conceptual trust score or affiliation data.
    *   **Remote Host Selection (Output from Gemini to DOL/VCMS):**
        *   Ranked list of suitable remote host IDs.
        *   Recommended `VMConfig` adjustments for the target remote environment (e.g., specific vNIC attachments for remote network, available mediated GPU profiles on remote).
    *   **Secure VM Provisioning Orchestration (DOL/VCMS, informed by Gemini):**
        *   DOL/VCMS uses Gemini's recommendation to select a target host.
        *   It then calls `RemoteVMService.DeployVMToRemoteHost` on the target host's V-Architect instance.
    *   **Health Monitoring & Relocation (Gemini & VCMS - building on Phase 2 Clustering):**
        *   VCMS (if managing a cluster that includes remote hosts) or DOL monitors health of remote VMs via `GetRemoteVMStatus`.
        *   If a remote VM or its host fails or performs poorly, Gemini can analyze alternative available remote hosts and recommend/initiate migration (which would involve state transfer and re-instantiation).

**3. VM State Transfer Mechanisms:**

    *   **Virtual Disk Images (QCOW2):**
        *   **Full Transfer:** Initial deployment to a new remote host may require full transfer of QCOW2 base images and relevant snapshot overlays.
        *   **Differential Transfer:** For updates or migrations where a common base snapshot exists on both source and destination, only transfer the differential QCOW2 overlays.
        *   **Compression:** Use on-the-fly compression (e.g., zstd, lz4) during stream transfer.
        *   **Security:** Transfer over TLS-encrypted gRPC streams.
    *   **`VMConfig`:** Transferred as a JSON object (or Protobuf message) over the secure gRPC channel.
    *   **VM Memory State (for Remote Live Migration - Highly Conceptual):**
        *   Would use the same iterative pre-copy memory transfer protocol (TCP over TLS) defined for local live migration (Phase 1 Tech Spec, Section IV.A), but between remote hosts. Extremely sensitive to network latency and bandwidth.

**4. Remote Console Streaming Protocols:**

    *   **SPICE (Simple Protocol for Independent Computing Environments):** Mature, well-suited for full graphical console, supports audio, USB redirection. V-Architect Core Engine on remote host runs SPICE server for the VM. DOL on local machine runs SPICE client.
    *   **RDP (Remote Desktop Protocol):** If guest OS is Windows and RDP is enabled, DOL can facilitate establishing a direct RDP connection or proxy it if direct connection is complex due to NATs (though direct RDP is preferred).
    *   **WebRTC (Web Real-Time Communication):** For browser-based console access. Remote V-Architect instance streams VM display (e.g., from a virtual framebuffer or X server) via WebRTC to the user's browser.
    *   **Serial Console:** Streamed over the secure V-Architect gRPC protocol.

**5. Nexus Protocol Integration Points (Conceptual):**

    *   **Host Discovery & Reputation:** A V-Architect instance could query a Nexus Protocol compatible directory service to find other V-Architect hosts that have registered themselves as available for distributed compute.
    *   The directory could include host capabilities, (self-attested or community-verified) reputation scores, and connection endpoints.
    *   V-Architect would use this information to populate its list of potential remote hosts for Gemini's selection process.

**6. EmPower1 Blockchain Integration Points (Conceptual):**

    *   **Resource Accounting/Billing:** For P2P distributed execution, EmPower1 smart contracts could manage:
        *   Staking of tokens by hosts offering resources.
        *   Payment of tokens by users consuming resources.
        *   Automated settlement based on measured resource consumption (e.g., vCPU hours, data transferred – requires trusted oracles or verifiable telemetry).
    *   **Access Permissions/Entitlements:** Storing permissions (e.g., "User X is allowed to deploy VMs on Host Y's V-Architect instance") as NFTs or records on EmPower1, providing a decentralized way to manage access control in a P2P network.

**Initial Implementation Considerations:**
*   This entire mode is highly conceptual and long-term.
*   Initial steps would involve defining the secure inter-instance gRPC protocol for basic remote VM status and console streaming between two trusted V-Architect instances on a local network.
*   VM disk image transfer mechanisms (full, then differential) would be the next focus.
*   Full AI-driven orchestration, Nexus, and EmPower1 integrations are significant research and development efforts for much later stages.

## III. Security Mechanisms - Technical Specifications

This section details the technical specifications for the core security mechanisms within V-Architect, focusing on ensuring VM isolation, integrity, and providing a foundation for secure operations.

### A. Secure VM Isolation & vTPM - Technical Details

This subsection specifies the technical approaches for achieving strong VM isolation and integrating virtual Trusted Platform Module (vTPM) capabilities.

**1. Hardware-Enforced Isolation (Reiteration & Emphasis):**

    *   **Memory Isolation:**
        *   **Technical Detail:** Mandatory use of Intel EPT (Extended Page Tables) or AMD RVI/NPT (Nested Page Tables) by the Core Engine, as specified in Phase 1 Technical Specifications (Section III.C.1 - vRAM Management). This is the primary mechanism for preventing memory access between VMs and between VMs and the hypervisor.
        *   The Core Engine is responsible for correctly configuring and managing these page table structures for each VM.
    *   **I/O Isolation (IOMMU):**
        *   **Technical Detail:** Mandatory use of Intel VT-d or AMD-Vi by the Core Engine for all PCIe device passthrough scenarios (Phase 1 Technical Specifications, Sections III.B.2 for vGPU, III.A.2 for AI CPU, III.H.2 for vAI-GPU, and general PCIe passthrough).
        *   The Core Engine must ensure that DMA remapping is correctly configured to restrict device access strictly to the owning VM's assigned memory.
        *   IOMMU fault handling must be robust to prevent or log unauthorized DMA attempts.
    *   **vCPU Execution Contexts:**
        *   **Technical Detail:** The Core Engine ensures that CPU state (registers, VMCS/VMCB) for each vCPU is saved and restored securely on VM-entry/VM-exit, preventing state leakage between vCPUs of different VMs or the host, as covered in Phase 1 vCPU specs.

**2. Virtual TPM (vTPM) Emulation:**

    *   **Specification Version:** Emulate a **TPM 2.0** compliant device.
    *   **Backend Library/Implementation:**
        *   **Primary Candidate:** Leverage `libtpms` library, which provides a software emulation of a TPM. This library is used by QEMU.
        *   **Integration with Core Engine:** The Core Engine will link against `libtpms` or an equivalent library. For each VM configured with a vTPM, the Core Engine will instantiate a separate TPM context using the library.
    *   **vTPM State Persistence:**
        *   The vTPM's persistent state (e.g., endorsement keys, storage root keys, NVRAM) must be saved when the VM is powered off or snapshotted.
        *   **Storage:** The state will be stored as a file on the host system, associated with the VM's configuration data (e.g., `[vm_id]_vtpm_state.bin`). This file must be protected by host file system permissions.
        *   **Encryption (Conceptual - Future):** For enhanced security, the vTPM state file itself could be encrypted at rest using a key derived from a host-level secret or a user-provided passphrase (though this adds key management complexity).
    *   **Interface to Guest OS:**
        *   The vTPM will be exposed to the guest OS typically as a memory-mapped device on an ISA bus or via a specific ACPI entry, emulating a standard TPM hardware interface.
        *   The guest OS (e.g., Windows with its TPM driver, Linux with `CONFIG_TCG_TPM`) should automatically detect and use the vTPM.
    *   **API for Enabling/Managing vTPM (`VMConfig` and `VMService`):**
        *   **`VMConfig` Schema (`vm_config_schema.json`):**
            ```json
            // Inside "properties":
            "virtual_tpm_enabled": {
              "description": "Enable a virtual TPM 2.0 device for this VM.",
              "type": "boolean",
              "default": false
            },
            "vtpm_state_path": {
                "description": "Path to the vTPM state file (managed internally by V-Architect). Read-only for users.",
                "type": "string",
                "readOnly": true
            }
            ```
        *   The `VMService.CreateVM` and `UpdateVMConfiguration` RPCs will handle the `virtual_tpm_enabled` flag.
        *   A dedicated RPC might be needed for specific vTPM management actions if required beyond simple enabling:
            ```protobuf
            // Conceptual - If more granular control is needed later
            // service TPMService {
            //   rpc ResetVMTPM(ResetVMTPMRequest) returns (ResetVMTPMResponse); // Clears vTPM persistent state
            // }
            ```
    *   **Use Cases Supported:**
        *   Guest OS Secure Boot (when combined with UEFI firmware for the VM).
        *   Full-disk encryption key management within the guest (e.g., Windows BitLocker, Linux LUKS with TPM).
        *   Application-level use of TPM functionalities (signing, attestation, secure storage).

**3. Measured Boot & Attestation (Leveraging vTPM):**

    *   **Measured Boot Process (Guest Responsibility, Enabled by vTPM):**
        *   If the VM is configured with UEFI firmware and Secure Boot enabled, the guest's boot components (UEFI firmware, bootloader, kernel, key drivers) will measure themselves (calculate cryptographic hashes) and extend these measurements into the vTPM's Platform Configuration Registers (PCRs) during the boot sequence.
    *   **Exposing PCRs for Attestation (Core Engine API):**
        *   The Core Engine needs an API to allow the DOL or an external attestation service to query vTPM PCR values for a given VM.
        *   **Conceptual RPC (part of `VMService` or a new `AttestationService`):**
            ```protobuf
            message GetVMTPMPCRsRequest {
              string vm_id = 1;
              repeated uint32 pcr_indexes = 2; // Which PCRs to read (e.g., 0-23 for TPM 2.0)
            }

            message PCRValue {
              uint32 pcr_index = 1;
              bytes pcr_hash = 2; // The hash value
              string hash_algorithm = 3; // e.g., "sha256"
            }

            message GetVMTPMPCRsResponse {
              string vm_id = 1;
              repeated PCRValue pcr_values = 2;
              // Potentially include a vTPM Quote for remote attestation - see below
            }
            ```
    *   **Remote Attestation Support (Conceptual):**
        *   To enable a remote challenger to verify a VM's integrity:
            1.  Challenger sends a nonce to the VM (or to V-Architect acting on behalf of VM).
            2.  The VM's vTPM generates a "Quote" – a signed message containing selected PCR values and the nonce, signed by one of its Attestation Keys (AK).
            3.  The Quote, along with the AK's public certificate (which chains back to a trusted Endorsement Key - EK certificate), is sent to the challenger.
            4.  Challenger verifies the Quote signature and checks PCR values against known good ("golden") measurements for that VM's software configuration.
        *   **V-Architect Role:**
            *   May need an API to request a vTPM Quote for a VM, including a nonce.
            *   Securely manage or provide access to the vTPM's EK certificate if needed for the attestation process.
        *   This is an advanced feature; initial focus is on enabling vTPM and exposing PCRs.

**Initial Implementation Considerations:**
*   Integrate `libtpms` or a similar library into the Core Engine.
*   Implement vTPM state file creation, loading, and saving tied to VM lifecycle.
*   Add `virtual_tpm_enabled` flag to `VMConfig` and ensure it's honored during VM creation.
*   Focus on enabling guest OS Secure Boot with a vTPM and UEFI firmware (e.g., OVMF).
*   Basic PCR value exposure via API can be an early goal. Full remote attestation flows are more complex.

### B. AI-Driven Monitoring & Auditing - Technical Details

This subsection details the technical specifications for V-Architect's AI-enhanced security monitoring, auditing, and threat detection capabilities, leveraging Google Gemini and conceptual integrations with other AI security platforms.

**1. Telemetry Collection for Security Monitoring:**

    *   **Sources & Data Points (Extending Phase 2 Server Monitoring, Section III.B.3):**
        *   **Core Engine/Hypervisor Level:**
            *   **VM Exits:** High frequency or unusual VM exit reasons (e.g., repeated attempts to access privileged MSRs, specific page faults indicative of exploit attempts).
            *   **IOMMU Faults:** Logged by the hypervisor, indicating potential unauthorized DMA attempts.
            *   **vTPM Events (Conceptual):** Logged alerts from vTPM (e.g., PCR validation failures if checked against a policy, failed self-tests).
            *   **Inter-VM Network Traffic (Metadata):** For VMs on the same vSwitch/AI Switch, metadata of connection attempts that violate microsegmentation policies (if defined).
            *   **API Call Audits:** Security-sensitive Core Engine API calls (e.g., `DeleteVM`, `UpdateVMConfiguration` changing security settings, `ManageVMMedia` attaching new media).
        *   **V-Architect Guest Tools (Optional, Opt-in, Security-Focused Telemetry):**
            *   **Process Creation Events:** Monitoring for suspicious process creations (e.g., unexpected processes, processes with unusual names or paths, execution from unexpected locations like `/tmp`).
            *   **Network Connection Events (Guest View):** Outbound connections to known malicious IPs/domains (requires threat intelligence feed). New listening ports opened by applications.
            *   **Authentication Events:** Failed login attempts (SSH, RDP within guest), privilege escalation events (e.g., `sudo` usage logs).
            *   **File Integrity Monitoring (Conceptual - Advanced):** Monitoring critical system files or directories for unauthorized changes using checksums.
            *   **Security Log Forwarding:** Forwarding selected critical entries from guest OS security logs (e.g., Windows Security Event Log, Linux `auditd` logs).
        *   **Data Format & Transport:**
            *   Security telemetry will use a standardized, structured format (e.g., JSON objects or OpenTelemetry logs with security-specific attributes).
            *   Transported securely (TLS) from Guest Tools to the V-Architect Telemetry Aggregation Service (defined in Phase 2).
            *   Core Engine security events are logged directly to the same service or a dedicated security log component.

**2. AI Analysis for Security (Google Gemini & Conceptual Integrations):**

    *   **Gemini's Role (Security Anomaly Detection & Correlation):**
        *   **Input:** Consumes security telemetry from the V-Architect Telemetry Aggregation Service.
        *   **Anomaly Detection Models:**
            *   **Behavioral Baselines:** Establishes baselines for "normal" security-relevant behavior per VM (e.g., typical network connections, common processes, user login patterns).
            *   **Deviation Detection:** Identifies significant deviations from these baselines (e.g., a server VM suddenly attempting SSH to many internal IPs, a user logging in from an unusual geolocation).
            *   **Known Bad Patterns:** Uses signatures or patterns of known malicious activity (e.g., specific commands used by malware, connections to C2 server domains – requires threat intelligence feeds).
        *   **Threat Intelligence Correlation (Conceptual):**
            *   Gemini could be integrated with external threat intelligence feeds (e.g., commercial feeds, OSINT feeds like AlienVault OTX). Observed IOCs (IPs, domains, file hashes) from VM telemetry would be checked against these feeds.
        *   **UEBA (User and Entity Behavior Analytics - Conceptual):**
            *   Analyze patterns of user access to V-Architect management functions and (if guest tools provide sufficient data) user activity within VMs to detect compromised accounts or malicious insider activity.
        *   **Alert Prioritization & Contextualization:** Gemini prioritizes alerts based on severity, confidence, and potential impact. It provides contextual information and hypotheses about the detected threat (as per Alert Structure in Phase 2, Section III.B.3).
    *   **IBM Watson Security Integration (Conceptual API Interaction):**
        *   **Purpose:** To leverage Watson's advanced threat intelligence and security analytics for deeper investigation or second-opinion analysis of critical alerts flagged by Gemini.
        *   **API Interaction:**
            1.  V-Architect (DOL or central AI service) sends structured alert data (IOCs, behavioral summary from Gemini) to a Watson Security API endpoint (e.g., QRadar Advisor with Watson API).
            2.  Watson performs its analysis (correlating with its threat intelligence, risk databases, etc.).
            3.  Watson returns an enriched analysis report, potential MITRE ATT&CK tactics/techniques, confidence scores, and recommended incident response playbook steps.
        *   This information is then presented in the V-Architect UI alongside Gemini's initial findings.

**3. `AIAuditLog` - Technical Specification:**

    *   **Purpose:** To provide a comprehensive, secure, and potentially immutable record of all significant actions and events within the V-Architect ecosystem, especially those relevant to security and AI decision-making.
    *   **Log Fields (Extending standard audit log fields):**
        *   `event_id`: (UUID, unique).
        *   `timestamp_unix_epoch_ns`: (uint64, nanosecond precision).
        *   `event_source_component`: (string, e.g., "CoreEngine.VMService", "DOL.UserSession", "Gemini.SecurityMonitor", "GuestTools.ProcessMonitor").
        *   `actor_id`: (string, e.g., user_id, system_service_name, vm_id if action initiated by guest).
        *   `action_type`: (string, e.g., "VM_CREATE", "VM_START", "SNAPSHOT_DELETE", "SECURITY_POLICY_APPLIED", "ANOMALY_DETECTED", "AI_RECOMMENDATION_GENERATED", "REMOTE_HOST_LOGIN_FAILED").
        *   `target_resource_id`: (string, e.g., vm_id, host_id, policy_id, snapshot_id).
        *   `status`: (enum: SUCCESS, FAILED, PENDING, INFO, WARNING, CRITICAL).
        *   `details_json`: (JSON string, for structured event-specific details, e.g., parameters of an API call, specific metrics that triggered an anomaly).
        *   `ai_recommendation_id_if_applicable`: (string, links to a specific AI recommendation that triggered or is related to this event).
        *   `correlation_id`: (string, to link related events in a sequence).
        *   `log_hash`: (string, hash of this log entry - for integrity checks).
        *   `previous_log_hash`: (string, hash of the previous log entry in the sequence - for chaining if blockchain-like properties are emulated locally).
    *   **Storage & Security:**
        *   Logs are written to a local, append-only secure log file or database on each V-Architect host (for Core Engine events) or central management component (for DOL/cluster events).
        *   Log files should be periodically rotated and archived.
        *   Access to raw audit logs should be strictly controlled.
    *   **EmPower1 Blockchain Anchoring (Conceptual):**
        *   **Process:**
            1.  Periodically (e.g., every hour, or based on log volume), V-Architect batches recent `AIAuditLog` entries.
            2.  A cryptographic hash (e.g., SHA256 or a Merkle root) of this batch is calculated.
            3.  A V-Architect system identity (with an EmPower1 wallet) creates a transaction on the EmPower1 Blockchain.
            4.  This transaction includes the hash of the log batch and a `TxType` field (e.g., `"VARCH_AUDIT_ANCHOR"`).
            5.  The transaction ID and block number from EmPower1 are stored by V-Architect, associated with that log batch.
        *   **Verification:** Allows an external auditor to verify that the V-Architect audit logs have not been tampered with after the anchoring point by recalculating the batch hash and comparing it with the one stored on the immutable blockchain.

**Initial Implementation Considerations:**
*   Focus on collecting key hypervisor-level security events and basic guest authentication logs (via guest tools).
*   Develop initial Gemini anomaly detection models for common scenarios like network scanning or unusual process execution (if process creation events are collected).
*   Implement a robust local `AIAuditLog` system with structured logging.
*   Blockchain anchoring and integration with external AI security platforms like Watson are advanced features for later stages.
*   Ensure user consent and privacy considerations are paramount for any guest-level telemetry collection.
```
