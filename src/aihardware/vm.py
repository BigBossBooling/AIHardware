from typing import List, Dict
from src.aihardware.components import VirtualComponent

class VirtualMachine:
    def __init__(self, name: str, os_type: str):
        self.name = name
        self.os_type = os_type
        self.components: List[VirtualComponent] = []

    def add_component(self, component: VirtualComponent):
        self.components.append(component)

    def describe_system(self) -> str:
        desc = [f"Virtual Machine: {self.name} (OS: {self.os_type})"]
        desc.append("Hardware Configuration:")
        for comp in self.components:
            desc.append(f"  - {comp.describe()}")
        return "\n".join(desc)

    def double_all_specs(self):
        """Instantly double specs for all components."""
        for comp in self.components:
            comp.double_specs()
