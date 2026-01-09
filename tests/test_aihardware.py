import unittest
import sys
import os

sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.aihardware.components import vCPU, AICPU
from src.aihardware.vm import VirtualMachine
from src.v_architect import VArchitect

class TestAIHardware(unittest.TestCase):
    def test_vcpu_creation(self):
        cpu = vCPU("TestCPU", 4, 3.0)
        self.assertIn("4 Cores", cpu.describe())

    def test_aicpu_creation(self):
        aicpu = AICPU("TestAI", 8, 4.0, 16)
        self.assertIn("16 NPU Cores", aicpu.describe())

    def test_vm_aggregation(self):
        vm = VirtualMachine("TestVM", "Linux")
        vm.add_component(vCPU("Core", 2, 2.0))
        self.assertIn("vCPU: Core", vm.describe_system())

    def test_double_specs(self):
        cpu = vCPU("TestCPU", 4, 3.0)
        cpu.double_specs()
        self.assertEqual(cpu.specs["cores"], 8)
        self.assertEqual(cpu.specs["frequency"], 6.0)

class TestVArchitect(unittest.TestCase):
    def setUp(self):
        self.architect = VArchitect()

    def test_sculpt_reality(self):
        result = self.architect.sculpt_reality("I need a high performance AI server")
        self.assertIn("ARCHITECTED REALITY", result)
        self.assertIn("AI-CPU", result) # Should select AI components

if __name__ == '__main__':
    unittest.main()
