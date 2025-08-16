import pytest
from v_architect.metrics_engine import MetricsEngine

# A mock ResourceManager for testing purposes
class MockResourceManager:
    def __init__(self, cpu_util, mem_util):
        self._utilization = {
            "cpu_utilization_percent": cpu_util,
            "memory_utilization_percent": mem_util,
        }

    def get_utilization(self):
        return self._utilization

    def set_utilization(self, cpu_util, mem_util):
        self._utilization["cpu_utilization_percent"] = cpu_util
        self._utilization["memory_utilization_percent"] = mem_util

@pytest.fixture
def mock_manager():
    """Provides a mock ResourceManager with default utilization."""
    return MockResourceManager(cpu_util=50.0, mem_util=50.0)

@pytest.fixture
def metrics_engine(mock_manager):
    """Provides a MetricsEngine instance with default thresholds."""
    thresholds = {
        "CPU_HIGH_LOAD": lambda util: util["cpu_utilization_percent"] > 80,
        "MEMORY_LOW": lambda util: util["memory_utilization_percent"] > 85,
    }
    return MetricsEngine(mock_manager, thresholds)

def test_metrics_engine_initialization(metrics_engine, mock_manager):
    """Tests that the MetricsEngine initializes correctly."""
    assert metrics_engine.resource_manager == mock_manager
    assert "CPU_HIGH_LOAD" in metrics_engine.thresholds

def test_no_event_fired_when_below_threshold(metrics_engine):
    """Tests that no event is fired when utilization is below the threshold."""
    events_fired = []
    metrics_engine.subscribe("CPU_HIGH_LOAD", lambda event: events_fired.append(event))

    metrics_engine.check_metrics()

    assert len(events_fired) == 0

# --- Tests for History Tracking ---

def test_metric_history_is_stored(metrics_engine, mock_manager):
    """Tests that the engine correctly stores a history of metrics."""
    # First check
    mock_manager.set_utilization(cpu_util=10.0, mem_util=20.0)
    metrics_engine.check_metrics()

    # Second check
    mock_manager.set_utilization(cpu_util=15.0, mem_util=25.0)
    metrics_engine.check_metrics()

    history = metrics_engine.get_history()
    assert len(history) == 2
    assert history[0]["cpu_utilization_percent"] == 10.0
    assert history[1]["cpu_utilization_percent"] == 15.0

def test_history_is_rolling(mock_manager):
    """Tests that the history is a rolling window of a fixed size."""
    # Create an engine with a small history size
    small_history_engine = MetricsEngine(mock_manager, {}, history_size=3)

    # Add 5 data points
    for i in range(5):
        mock_manager.set_utilization(cpu_util=float(i), mem_util=float(i))
        small_history_engine.check_metrics()

    history = small_history_engine.get_history()
    assert len(history) == 3
    # The history should contain the last 3 data points: 2.0, 3.0, and 4.0
    assert history[0]["cpu_utilization_percent"] == 2.0
    assert history[1]["cpu_utilization_percent"] == 3.0
    assert history[2]["cpu_utilization_percent"] == 4.0

def test_event_fired_when_cpu_threshold_breached(metrics_engine, mock_manager):
    """Tests that an event is fired correctly when CPU utilization breaches a threshold."""
    events_fired = []
    # Subscribe a simple listener that just appends the event to a list
    metrics_engine.subscribe("CPU_HIGH_LOAD", lambda event: events_fired.append(event))

    # Push utilization over the threshold
    mock_manager.set_utilization(cpu_util=90.0, mem_util=50.0)

    metrics_engine.check_metrics()

    assert len(events_fired) == 1
    event = events_fired[0]
    assert event["event_type"] == "CPU_HIGH_LOAD"
    assert event["details"]["cpu_utilization_percent"] == 90.0

def test_event_fired_for_memory_threshold(metrics_engine, mock_manager):
    """Tests that an event is fired for a memory-related threshold."""
    events_fired = []
    metrics_engine.subscribe("MEMORY_LOW", lambda event: events_fired.append(event))

    mock_manager.set_utilization(cpu_util=50.0, mem_util=95.0)
    metrics_engine.check_metrics()

    assert len(events_fired) == 1
    assert events_fired[0]["event_type"] == "MEMORY_LOW"

def test_multiple_subscribers_for_one_event(metrics_engine, mock_manager):
    """Tests that multiple listeners can subscribe to the same event."""
    listener1_calls = []
    listener2_calls = []

    metrics_engine.subscribe("CPU_HIGH_LOAD", lambda event: listener1_calls.append(event))
    metrics_engine.subscribe("CPU_HIGH_LOAD", lambda event: listener2_calls.append(event))

    mock_manager.set_utilization(cpu_util=95.0, mem_util=50.0)
    metrics_engine.check_metrics()

    assert len(listener1_calls) == 1
    assert len(listener2_calls) == 1
    assert listener1_calls[0]["event_type"] == "CPU_HIGH_LOAD"

def test_unsubscribe_removes_listener(metrics_engine, mock_manager):
    """Tests that a listener can be unsubscribed."""
    events_fired = []

    def my_listener(event):
        events_fired.append(event)

    metrics_engine.subscribe("CPU_HIGH_LOAD", my_listener)
    metrics_engine.unsubscribe("CPU_HIGH_LOAD", my_listener)

    mock_manager.set_utilization(cpu_util=99.0, mem_util=50.0)
    metrics_engine.check_metrics()

    assert len(events_fired) == 0
