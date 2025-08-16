"""
This module contains the PredictiveEngine for V-Architect.
"""

import collections
import numpy as np
from typing import Callable, Dict, Any, List

class PredictiveEngine:
    """
    Analyzes historical metrics to predict future resource needs and
    generates predictive events.
    """

    def __init__(self, metrics_engine, config: Dict):
        """
        Initializes the PredictiveEngine.

        Args:
            metrics_engine: An instance of MetricsEngine to get history from.
            config: A dictionary containing configuration, e.g.:
                    {
                        "prediction_steps": int,
                        "thresholds": { "EVENT_TYPE": lambda forecast: bool }
                    }
        """
        self.metrics_engine = metrics_engine
        self.config = config
        self._subscribers = collections.defaultdict(list)

    def subscribe(self, event_type: str, callback: Callable[[Dict], None]):
        """Subscribes a listener to a specific predictive event type."""
        self._subscribers[event_type].append(callback)

    def unsubscribe(self, event_type: str, callback: Callable[[Dict], None]):
        """Unsubscribes a listener from an event type."""
        try:
            self._subscribers[event_type].remove(callback)
        except ValueError:
            pass

    def _fire_event(self, event_type: str, details: Dict[str, Any]):
        """Fires a predictive event to all subscribers."""
        event = {
            "event_type": event_type,
            "details": details,
        }
        for callback in self._subscribers[event_type]:
            callback(event)

    def analyze_and_predict(self) -> List[Dict[str, float]]:
        """
        Analyzes historical data, forecasts future utilization, and fires
        predictive events if thresholds are likely to be breached.

        Returns:
            A list of forecasted data points.
        """
        history = self.metrics_engine.get_history()
        if len(history) < 2:
            # Not enough data to make a prediction
            return []

        # For now, we only predict CPU. A real system would handle all metrics.
        cpu_series = [data["cpu_utilization_percent"] for data in history]
        time_steps = np.arange(len(cpu_series))

        # Fit a line (polynomial of degree 1) to the data
        # The model is y = m*x + c
        model = np.polyfit(time_steps, cpu_series, 1)
        m, c = model[0], model[1]

        # Predict the next few steps
        prediction_steps = self.config.get("prediction_steps", 1)
        future_time_steps = np.arange(len(cpu_series), len(cpu_series) + prediction_steps)
        predicted_cpu = m * future_time_steps + c

        forecasts = [{"cpu_utilization_percent": round(val, 2)} for val in predicted_cpu]

        # Check forecasts against predictive thresholds
        for event_type, check_func in self.config.get("thresholds", {}).items():
            for i, forecast_point in enumerate(forecasts):
                if check_func(forecast_point):
                    # Fire event with the full forecast data
                    self._fire_event(event_type, {"forecast": forecasts, "trigger_step": i})
                    # Fire only once per analysis cycle for a given event type
                    break

        return forecasts
