from abc import ABC, abstractmethod
from typing import Dict, Any, Optional

class VirtualComponent(ABC):
    def __init__(self, name: str, specs: Dict[str, Any]):
        self.name = name
        self.specs = specs

    @abstractmethod
    def describe(self) -> str:
        pass

    def double_specs(self):
        """Instantly double relevant specifications."""
        for key, value in self.specs.items():
            if isinstance(value, (int, float)) and key not in ["version", "id", "ecc", "architecture", "type", "pci_generation"]:
                self.specs[key] = value * 2

class vCPU(VirtualComponent):
    def __init__(self, name: str, cores: int, frequency_ghz: float, architecture: str = "x86_64"):
        super().__init__(name, {"cores": cores, "frequency": frequency_ghz, "architecture": architecture})

    def describe(self) -> str:
        return f"vCPU: {self.name} ({self.specs['cores']} Cores @ {self.specs['frequency']} GHz, Arch: {self.specs['architecture']})"

class vRAM(VirtualComponent):
    def __init__(self, name: str, size_gb: int, ecc: bool = False, speed_mhz: int = 3200):
        super().__init__(name, {"size_gb": size_gb, "ecc": ecc, "speed_mhz": speed_mhz})

    def describe(self) -> str:
        ecc_str = "ECC" if self.specs['ecc'] else "Non-ECC"
        return f"vRAM: {self.name} ({self.specs['size_gb']} GB, {self.specs['speed_mhz']} MHz, {ecc_str})"

class vGPU(VirtualComponent):
    def __init__(self, name: str, vram_gb: int, cuda_cores: int):
        super().__init__(name, {"vram_gb": vram_gb, "cuda_cores": cuda_cores})

    def describe(self) -> str:
        return f"vGPU: {self.name} ({self.specs['vram_gb']} GB VRAM, {self.specs['cuda_cores']} CUDA Cores)"

# AI-Native Components
class AICPU(vCPU):
    def __init__(self, name: str, cores: int, frequency_ghz: float, npu_cores: int, architecture: str = "x86_64"):
        super().__init__(name, cores, frequency_ghz, architecture)
        self.specs["npu_cores"] = npu_cores

    def describe(self) -> str:
        return f"AI-CPU: {self.name} ({self.specs['cores']} Cores @ {self.specs['frequency']} GHz + {self.specs['npu_cores']} NPU Cores, Arch: {self.specs['architecture']})"

class AIGPU(vGPU):
    def __init__(self, name: str, vram_gb: int, cuda_cores: int, tensor_cores: int):
        super().__init__(name, vram_gb, cuda_cores)
        self.specs["tensor_cores"] = tensor_cores

    def describe(self) -> str:
        return f"AI-GPU: {self.name} ({self.specs['vram_gb']} GB VRAM, {self.specs['cuda_cores']} CUDA Cores, {self.specs['tensor_cores']} Tensor Cores)"

# Storage & Networking
class vStorage(VirtualComponent):
    def __init__(self, name: str, size_tb: float, storage_type: str = "NVMe", iops: int = 10000, throughput_mb_s: int = 3500):
        super().__init__(name, {
            "size_tb": size_tb,
            "type": storage_type,
            "iops": iops,
            "throughput_mb_s": throughput_mb_s
        })

    def describe(self) -> str:
        return f"vStorage: {self.name} ({self.specs['size_tb']} TB {self.specs['type']}, {self.specs['iops']} IOPS, {self.specs['throughput_mb_s']} MB/s)"

class vNIC(VirtualComponent):
    def __init__(self, name: str, bandwidth_gbps: int, latency_us: int = 10, pci_generation: int = 5):
        super().__init__(name, {
            "bandwidth_gbps": bandwidth_gbps,
            "latency_us": latency_us,
            "pci_generation": pci_generation
        })

    def describe(self) -> str:
        return f"vNIC: {self.name} ({self.specs['bandwidth_gbps']} Gbps, {self.specs['latency_us']} us Latency, PCIe Gen{self.specs['pci_generation']})"

class vSwitch(VirtualComponent):
    def __init__(self, name: str, ports: int, total_throughput_tbps: float):
        super().__init__(name, {"ports": ports, "total_throughput_tbps": total_throughput_tbps})

    def describe(self) -> str:
        return f"vSwitch: {self.name} ({self.specs['ports']} Ports, {self.specs['total_throughput_tbps']} Tbps Switching Capacity)"
