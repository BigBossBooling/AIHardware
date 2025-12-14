from typing import Dict, Any
from src.trihorn.core import TrihornCore
from src.aihardware.vm import VirtualMachine
from src.aihardware.components import vCPU, vRAM, AIGPU, AICPU

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

        # Default high-end specs
        vm = VirtualMachine(vm_name, "Linux (AI-Optimized)")

        if "research" in user_intent.lower() or "ai" in user_intent.lower():
            vm.add_component(AICPU("Neural-9000", cores=16, frequency_ghz=4.2, npu_cores=32))
            vm.add_component(vRAM("HyperMemory", size_gb=64))
            vm.add_component(AIGPU("OmniMatrix", vram_gb=24, cuda_cores=8192, tensor_cores=256))
        else:
            vm.add_component(vCPU("Standard-Core", cores=8, frequency_ghz=3.5))
            vm.add_component(vRAM("Standard-Mem", size_gb=16))

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
