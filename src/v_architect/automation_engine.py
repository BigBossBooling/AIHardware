"""
This module contains the AutomationEngine for V-Architect.
"""

from typing import Callable, Dict, Any

class AutomationEngine:
    """
    Contains a set of rules and executes actions when events are handled.
    """

    def __init__(self, metrics_engine, predictive_engine, resource_manager, virtualization_layer):
        """
        Initializes the AutomationEngine.

        Args:
            metrics_engine: An instance of MetricsEngine.
            predictive_engine: An instance of PredictiveEngine.
            resource_manager: An instance of ResourceManager to act upon.
            virtualization_layer: An instance of VirtualizationLayer to act upon.
        """
        self.metrics_engine = metrics_engine
        self.predictive_engine = predictive_engine
        self.resource_manager = resource_manager
        self.virtualization_layer = virtualization_layer
        self.rules: Dict[str, Callable[[Dict], None]] = {}

    def add_rule(self, event_type: str, action: Callable[[Dict], None]):
        """
        Adds a new rule to the engine. A rule maps an event type to an action.

        Args:
            event_type: The type of event to listen for (e.g., "CPU_HIGH_LOAD").
            action: A callable that will be executed when the event occurs.
        """
        self.rules[event_type] = action
        # Note: Subscription is no longer handled here. It is the responsibility
        # of the main application logic to wire the event sources to the handler.

    def handle_event(self, event: Dict[str, Any]):
        """
        The public event handler. When an event is passed to this method,
        it checks for a matching rule and executes the corresponding action.
        """
        event_type = event.get("event_type")
        if event_type in self.rules:
            action = self.rules[event_type]
            action(event)
