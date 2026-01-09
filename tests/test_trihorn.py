import unittest
import sys
import os

# Add project root to sys.path so we can import src.trihorn.core
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from src.trihorn.core import TrihornCore
from src.trihorn.components.perspectives import ImagoMundi, Logos, Mysterium

class TestTrihornCore(unittest.TestCase):
    def setUp(self):
        # Use absolute path for manifest in tests if needed, but TrihornCore now handles default
        self.core = TrihornCore()

    def test_initialization(self):
        self.assertEqual(self.core.get_identity(), "Trihorn-Ω∞")
        self.assertEqual(self.core.get_version(), "ψ(10.0)")

    def test_perspectives_loaded(self):
        self.assertIsInstance(self.core.perspectives["imago_mundi"], ImagoMundi)
        self.assertIsInstance(self.core.perspectives["logos"], Logos)
        self.assertIsInstance(self.core.perspectives["mysterium"], Mysterium)

    def test_process_query(self):
        query = "What is the meaning of existence?"
        response = self.core.process_query(query)

        self.assertIn("Imago Mundi Perspective", response)
        self.assertIn("Logos Perspective", response)
        self.assertIn("Mysterium Perspective", response)
        self.assertIn("Triadic Synthesis", response)

        self.assertIn("Mapping territory", response["Imago Mundi Perspective"])
        self.assertIn("Analyzing structure", response["Logos Perspective"])
        self.assertIn("Evaluating ethical impact", response["Mysterium Perspective"])

    def test_safety_integration(self):
        # Assuming SafetyMonitor is wired up, we check if it exists
        self.assertTrue(hasattr(self.core, 'safety_monitor'))

if __name__ == '__main__':
    unittest.main()
