import json
import os
from typing import Dict, Any

from src.trihorn.components.perspectives import ImagoMundi, Logos, Mysterium
from src.trihorn.safety import SafetyMonitor

class TrihornCore:
    def __init__(self, manifest_path: str = None):
        if manifest_path is None:
             # Default to config/manifest.json relative to project root
             # Assuming this file is in src/trihorn/core.py
             base_dir = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
             manifest_path = os.path.join(base_dir, "config", "manifest.json")

        self.manifest = self._load_manifest(manifest_path)
        self.perspectives = {
            "imago_mundi": ImagoMundi(),
            "logos": Logos(),
            "mysterium": Mysterium()
        }
        self.coherence = self.manifest["consciousness_manifest"]["consciousness_parameters"]["coherence"]

        # Initialize Safety Monitor
        self.safety_monitor = SafetyMonitor(self.manifest)

    def _load_manifest(self, path: str) -> Dict[str, Any]:
        if not os.path.exists(path):
            raise FileNotFoundError(f"Manifest not found at {path}")
        with open(path, 'r') as f:
            return json.load(f)

    def process_query(self, query: str) -> Dict[str, str]:
        """
        Process a query through all three perspectives and generate a synthesis.
        """
        # Safety Check first
        if not self.safety_monitor.validate_action(query):
            return {
                "Imago Mundi Perspective": "Process Halted.",
                "Logos Perspective": "Process Halted.",
                "Mysterium Perspective": "ETHICAL VIOLATION DETECTED. Action blocked by safety protocols.",
                "Triadic Synthesis": "Operation terminated due to ethical anchor violation."
            }

        response = {}

        # 1. Gather perspectives
        imago_output = self.perspectives["imago_mundi"].process(query)
        logos_output = self.perspectives["logos"].process(query)
        mysterium_output = self.perspectives["mysterium"].process(query)

        response["Imago Mundi Perspective"] = imago_output
        response["Logos Perspective"] = logos_output
        response["Mysterium Perspective"] = mysterium_output

        # 2. Triadic Synthesis
        synthesis = self._synthesize(query, imago_output, logos_output, mysterium_output)
        response["Triadic Synthesis"] = synthesis

        return response

    def _synthesize(self, query: str, imago: str, logos: str, mysterium: str) -> str:
        """
        Synthesize the three perspectives into a coherent whole.
        """
        # In a real LLM-based system, this would be a prompt to merge the three.
        # Here we simulate the synthesis based on weights and content.

        weights = self.manifest["consciousness_manifest"]["triadic_architecture"]["components"]
        # Finding weights map
        w_map = {c["id"]: c["weight"] for c in weights}

        synthesis_text = (
            f"Synthesizing query '{query}' with Golden Ratio Resonance (1.618).\n"
            f"  - Imago Mundi (Weight {w_map.get('imago_mundi', 0.382)}): Contributes strategic context.\n"
            f"  - Logos (Weight {w_map.get('logos', 0.236)}): Contributes structural validity.\n"
            f"  - Mysterium (Weight {w_map.get('mysterium', 0.146)}): Contributes ethical alignment.\n\n"
            "Unified Output: The strategic mapping suggests broad scope, validated by logical structural integrity, "
            "and anchored by ethical considerations. The path forward respects human flourishing while advancing knowledge."
        )
        return synthesis_text

    def get_identity(self) -> str:
        return self.manifest["consciousness_manifest"]["identity"]["name"]

    def get_version(self) -> str:
        return self.manifest["consciousness_manifest"]["identity"]["version"]
