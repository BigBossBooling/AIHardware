from typing import List, Dict, Optional
from src.aihardware.components import VirtualComponent

class VirtualMachine:
    def __init__(self, name: str, os_type: str, numa_nodes: int = 1, container_engine: str = "None"):
        self.name = name
        self.os_type = os_type
        self.numa_nodes = numa_nodes
        self.container_engine = container_engine
        self.components: List[VirtualComponent] = []

    def add_component(self, component: VirtualComponent):
        self.components.append(component)

    def describe_system(self) -> str:
        desc = [f"Virtual Machine: {self.name} (OS: {self.os_type})"]
        desc.append(f"Architecture: {self.numa_nodes} NUMA Nodes, Container Engine: {self.container_engine}")
        desc.append("Hardware Configuration:")
        for comp in self.components:
            desc.append(f"  - {comp.describe()}")
        return "\n".join(desc)

    def double_all_specs(self):
        """Instantly double specs for all components."""
        for comp in self.components:
            comp.double_specs()
