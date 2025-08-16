"""
This module contains the MetricsEngine for V-Architect.
"""

import collections
from typing import Callable, Dict, Any

class MetricsEngine:
    """
    Monitors resource utilization and generates events when thresholds are breached.
    """

    def __init__(self, resource_manager, thresholds: Dict[str, Callable[[Dict], bool]]):
        """
        Initializes the MetricsEngine.

        Args:
            resource_manager: An instance of a ResourceManager (or a mock).
            thresholds: A dictionary where keys are event types (str) and
                        values are functions that return True if the
                        threshold is breached.
        """
        self.resource_manager = resource_manager
        self.thresholds = thresholds
        self._subscribers = collections.defaultdict(list)

    def subscribe(self, event_type: str, callback: Callable[[Dict], None]):
        """Subscribes a listener to a specific event type."""
        self._subscribers[event_type].append(callback)

    def unsubscribe(self, event_type: str, callback: Callable[[Dict], None]):
        """Unsubscribes a listener from an event type."""
        try:
            self._subscribers[event_type].remove(callback)
        except ValueError:
            # Callback not found, ignore silently.
            pass

    def _fire_event(self, event_type: str, details: Dict[str, Any]):
        """Fires an event to all subscribers of the event type."""
        event = {
            "event_type": event_type,
            "details": details,
        }
        for callback in self._subscribers[event_type]:
            callback(event)

    def check_metrics(self):
        """
        Polls the resource manager, checks all thresholds, and fires
        events for any breached thresholds.
        """
        current_utilization = self.resource_manager.get_utilization()

        for event_type, check_func in self.thresholds.items():
            if check_func(current_utilization):
                self._fire_event(event_type, current_utilization)
