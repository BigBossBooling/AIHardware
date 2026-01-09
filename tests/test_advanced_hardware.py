import unittest
import sys
import os

sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.aihardware.components import vStorage, vNIC, vSwitch, vRAM
from src.v_architect import VArchitect

class TestAdvancedHardware(unittest.TestCase):
    def test_storage_creation(self):
        storage = vStorage("TestNVMe", 2.0, "NVMe", 50000)
        desc = storage.describe()
        self.assertIn("NVMe", desc)
        self.assertIn("50000 IOPS", desc)

    def test_nic_creation(self):
        nic = vNIC("TestNIC", 100, 5)
        desc = nic.describe()
        self.assertIn("100 Gbps", desc)
        self.assertIn("5 us Latency", desc)

    def test_ecc_ram(self):
        ram = vRAM("ECCMem", 32, ecc=True)
        self.assertIn("ECC", ram.describe())
        self.assertNotIn("Non-ECC", ram.describe())

    def test_architect_sculpting_ai(self):
        architect = VArchitect()
        result = architect.sculpt_reality("I need a massive AI training cluster")

        self.assertIn("NUMA Nodes", result)
        self.assertIn("Kubernetes", result)
        self.assertIn("InfiniConnect", result) # 100Gbps NIC
        self.assertIn("ECC", result)

    def test_architect_sculpting_enterprise(self):
        architect = VArchitect()
        result = architect.sculpt_reality("Secure enterprise server")

        self.assertIn("SSD-Enterprise", result)
        self.assertIn("GigabitLink", result)
        self.assertIn("ReliableMem-ECC", result)

if __name__ == '__main__':
    unittest.main()
