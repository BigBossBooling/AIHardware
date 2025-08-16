import pytest
from unittest.mock import MagicMock
from v_architect.predictive_engine import PredictiveEngine

# A mock MetricsEngine for testing purposes
class MockMetricsEngine:
    def __init__(self, history):
        self._history = history

    def get_history(self):
        return self._history

@pytest.fixture
def mock_metrics_engine():
    """Provides a mock MetricsEngine with a clear upward trend."""
    history = [
        {"cpu_utilization_percent": 10},
        {"cpu_utilization_percent": 20},
        {"cpu_utilization_percent": 30},
        {"cpu_utilization_percent": 40},
    ]
    return MockMetricsEngine(history)

@pytest.fixture
def predictive_engine(mock_metrics_engine):
    """Provides a PredictiveEngine instance."""
    # Predict 2 steps into the future
    # Fire an alert if the prediction for CPU is > 55%
    config = {
        "prediction_steps": 2,
        "thresholds": {
            "PREDICTIVE_CPU_OVERLOAD": lambda forecast: forecast["cpu_utilization_percent"] > 55
        }
    }
    return PredictiveEngine(mock_metrics_engine, config)

def test_predictive_engine_initialization(predictive_engine, mock_metrics_engine):
    """Tests that the engine initializes correctly."""
    assert predictive_engine.metrics_engine == mock_metrics_engine

def test_linear_regression_forecast(predictive_engine):
    """Tests the internal forecasting model with a simple linear trend."""
    # History is [10, 20, 30, 40]. A linear fit should predict 50 and 60.
    forecast = predictive_engine.analyze_and_predict()

    # Check the forecast for the configured number of steps ahead
    assert len(forecast) == 2
    # The first predicted point (t=5) should be close to 50
    assert 49 < forecast[0]["cpu_utilization_percent"] < 51
    # The second predicted point (t=6) should be close to 60
    assert 59 < forecast[1]["cpu_utilization_percent"] < 61

def test_predictive_event_is_fired(predictive_engine):
    """Tests that a predictive event is fired when a forecast breaches a threshold."""
    events_fired = []
    predictive_engine.subscribe("PREDICTIVE_CPU_OVERLOAD", lambda event: events_fired.append(event))

    # The forecast will be ~60 for the second step, which is > 55 (the threshold)
    predictive_engine.analyze_and_predict()

    assert len(events_fired) == 1
    event = events_fired[0]
    assert event["event_type"] == "PREDICTIVE_CPU_OVERLOAD"
    # The event details should include the forecast that triggered it
    assert "forecast" in event["details"]
    assert event["details"]["forecast"][1]["cpu_utilization_percent"] > 55

def test_no_predictive_event_when_below_threshold(predictive_engine, mock_metrics_engine):
    """Tests that no event is fired if the forecast is below the threshold."""
    # Change history to a flat line
    mock_metrics_engine._history = [
        {"cpu_utilization_percent": 10},
        {"cpu_utilization_percent": 10},
        {"cpu_utilization_percent": 10},
    ]

    events_fired = []
    predictive_engine.subscribe("PREDICTIVE_CPU_OVERLOAD", lambda event: events_fired.append(event))

    # The forecast will be ~10, which is not > 55
    predictive_engine.analyze_and_predict()

    assert len(events_fired) == 0
