from typing import Dict, Any
from src.trihorn.core import TrihornCore
from src.aihardware.vm import VirtualMachine
from src.aihardware.components import vCPU, vRAM, AIGPU, AICPU, vStorage, vNIC, vSwitch

class VArchitect:
    def __init__(self):
        self.consciousness = TrihornCore()
        self.active_vms = {}

    def sculpt_reality(self, user_intent: str) -> str:
        """
        Uses Trihorn consciousness to design a Virtual Machine based on user intent.
        """
        # 1. Consult Trihorn
        trihorn_response = self.consciousness.process_query(f"Design a virtual machine architecture for: {user_intent}")
        synthesis = trihorn_response["Triadic Synthesis"]

        # 2. Parse intent (Simulated parsing based on keywords)
        vm_name = f"Project-{hash(user_intent) % 1000}"
        intent_lower = user_intent.lower()

        # Determine OS and Environment
        os_type = "Linux (AI-Optimized)"
        container_engine = "Docker" # Default to Docker for modern workflows
        if "cluster" in intent_lower or "kubernetes" in intent_lower:
            container_engine = "Kubernetes"
            os_type = "Linux (Cluster Node)"

        vm = VirtualMachine(vm_name, os_type, numa_nodes=1, container_engine=container_engine)

        # High Performance Computing / AI Research
        if "research" in intent_lower or "ai" in intent_lower or "training" in intent_lower:
            vm.numa_nodes = 2 # Simulate dual-socket behavior
            vm.add_component(AICPU("Neural-9000", cores=32, frequency_ghz=4.5, npu_cores=64, architecture="x86_64"))
            vm.add_component(vRAM("HyperMemory-ECC", size_gb=128, ecc=True, speed_mhz=4800))
            vm.add_component(AIGPU("OmniMatrix-H100", vram_gb=80, cuda_cores=16384, tensor_cores=512))
            vm.add_component(vStorage("FastData-NVMe", size_tb=4.0, storage_type="NVMe", iops=50000, throughput_mb_s=7000))
            vm.add_component(vNIC("InfiniConnect", bandwidth_gbps=100, latency_us=5))

        # Enterprise / Secure Server
        elif "server" in intent_lower or "secure" in intent_lower or "enterprise" in intent_lower:
            vm.add_component(vCPU("Enterprise-Xeon", cores=16, frequency_ghz=3.8))
            vm.add_component(vRAM("ReliableMem-ECC", size_gb=32, ecc=True))
            vm.add_component(vStorage("DataVault-SSD", size_tb=2.0, storage_type="SSD-Enterprise", iops=20000))
            vm.add_component(vNIC("GigabitLink", bandwidth_gbps=10, latency_us=20))

        # Standard / General Use
        else:
            vm.add_component(vCPU("Standard-Core", cores=8, frequency_ghz=3.5))
            vm.add_component(vRAM("Standard-Mem", size_gb=16, ecc=False))
            vm.add_component(vStorage("Basic-SSD", size_tb=0.5, storage_type="SSD"))
            vm.add_component(vNIC("Standard-Net", bandwidth_gbps=1))

        self.active_vms[vm_name] = vm

        return (
            f"{synthesis}\n\n"
            f"--- ARCHITECTED REALITY ---\n"
            f"{vm.describe_system()}"
        )

    def double_vm_resources(self, vm_name_fragment: str) -> str:
        """Scale the resources of a VM."""
        # Find VM
        target_vm = None
        for name, vm in self.active_vms.items():
            if vm_name_fragment in name:
                target_vm = vm
                break

        if not target_vm:
            return "VM not found."

        # Safety Check with Trihorn
        safety_check = self.consciousness.process_query(f"Is it safe to double resources for {target_vm.name}?")
        if "safe" in safety_check["Mysterium Perspective"].lower() or "aligned" in safety_check["Mysterium Perspective"].lower():
             target_vm.double_all_specs()
             return f"Resources doubled for {target_vm.name}.\n{target_vm.describe_system()}"
        else:
            return "Resource scaling denied by Mysterium protocols."
