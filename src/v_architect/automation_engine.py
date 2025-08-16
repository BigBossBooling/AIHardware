"""
This module contains the AutomationEngine for V-Architect.
"""

from typing import Callable, Dict, Any

class AutomationEngine:
    """
    Listens for events from the MetricsEngine and executes actions based on
    predefined rules.
    """

    def __init__(self, metrics_engine, resource_manager, virtualization_layer):
        """
        Initializes the AutomationEngine.

        Args:
            metrics_engine: An instance of MetricsEngine to subscribe to.
            resource_manager: An instance of ResourceManager to act upon.
            virtualization_layer: An instance of VirtualizationLayer to act upon.
        """
        self.metrics_engine = metrics_engine
        self.resource_manager = resource_manager
        self.virtualization_layer = virtualization_layer
        self.rules: Dict[str, Callable[[Dict], None]] = {}

    def add_rule(self, event_type: str, action: Callable[[Dict], None]):
        """
        Adds a new rule to the engine. A rule maps an event type to an action.

        Args:
            event_type: The type of event to listen for (e.g., "CPU_HIGH_LOAD").
            action: A callable that will be executed when the event occurs.
                    The action will receive the event dictionary as an argument.
        """
        self.rules[event_type] = action
        # Subscribe the engine's event handler to this event type
        self.metrics_engine.subscribe(event_type, self.handle_event)

    def handle_event(self, event: Dict[str, Any]):
        """
        The callback method that is executed by the MetricsEngine when an
        event is fired.
        """
        event_type = event.get("event_type")
        if event_type in self.rules:
            action = self.rules[event_type]
            # Execute the action associated with the rule
            action(event)
