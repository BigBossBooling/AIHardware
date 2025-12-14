from abc import ABC, abstractmethod
from typing import Dict, Any

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
            if isinstance(value, (int, float)) and key not in ["version", "id"]:
                self.specs[key] = value * 2

class vCPU(VirtualComponent):
    def __init__(self, name: str, cores: int, frequency_ghz: float):
        super().__init__(name, {"cores": cores, "frequency": frequency_ghz})

    def describe(self) -> str:
        return f"vCPU: {self.name} ({self.specs['cores']} Cores @ {self.specs['frequency']} GHz)"

class vRAM(VirtualComponent):
    def __init__(self, name: str, size_gb: int):
        super().__init__(name, {"size_gb": size_gb})

    def describe(self) -> str:
        return f"vRAM: {self.name} ({self.specs['size_gb']} GB)"

class vGPU(VirtualComponent):
    def __init__(self, name: str, vram_gb: int, cuda_cores: int):
        super().__init__(name, {"vram_gb": vram_gb, "cuda_cores": cuda_cores})

    def describe(self) -> str:
        return f"vGPU: {self.name} ({self.specs['vram_gb']} GB VRAM, {self.specs['cuda_cores']} CUDA Cores)"

# AI-Native Components
class AICPU(vCPU):
    def __init__(self, name: str, cores: int, frequency_ghz: float, npu_cores: int):
        super().__init__(name, cores, frequency_ghz)
        self.specs["npu_cores"] = npu_cores

    def describe(self) -> str:
        return f"AI-CPU: {self.name} ({self.specs['cores']} Cores @ {self.specs['frequency']} GHz + {self.specs['npu_cores']} NPU Cores)"

class AIGPU(vGPU):
    def __init__(self, name: str, vram_gb: int, cuda_cores: int, tensor_cores: int):
        super().__init__(name, vram_gb, cuda_cores)
        self.specs["tensor_cores"] = tensor_cores

    def describe(self) -> str:
        return f"AI-GPU: {self.name} ({self.specs['vram_gb']} GB VRAM, {self.specs['cuda_cores']} CUDA Cores, {self.specs['tensor_cores']} Tensor Cores)"
