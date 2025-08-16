import pytest
from unittest.mock import MagicMock, Mock
from v_architect.automation_engine import AutomationEngine

# --- Mocks for all dependencies ---

@pytest.fixture
def mock_metrics_engine():
    """A mock MetricsEngine that allows us to simulate event firing."""
    engine = MagicMock()
    # We need to store subscribers so we can call them manually
    engine.subscribers = {}

    def mock_subscribe(event_type, callback):
        engine.subscribers[event_type] = callback

    engine.subscribe.side_effect = mock_subscribe
    return engine

@pytest.fixture
def mock_resource_manager():
    """A mock ResourceManager to check if actions are called."""
    return MagicMock()

@pytest.fixture
def mock_virtualization_layer():
    """A mock VirtualizationLayer."""
    return MagicMock()

# --- AutomationEngine Tests ---

@pytest.fixture
def automation_engine(mock_metrics_engine, mock_resource_manager, mock_virtualization_layer):
    """Provides an AutomationEngine instance with mocked dependencies."""
    return AutomationEngine(mock_metrics_engine, mock_resource_manager, mock_virtualization_layer)

def test_automation_engine_initialization(automation_engine, mock_metrics_engine):
    """Tests that the engine initializes and is ready."""
    assert automation_engine.metrics_engine == mock_metrics_engine
    assert automation_engine.rules == {}

def test_add_rule(automation_engine):
    """Tests that a rule can be successfully added."""
    action = lambda event: print(f"Action for {event['event_type']}")
    automation_engine.add_rule("TEST_EVENT", action)

    assert "TEST_EVENT" in automation_engine.rules
    assert automation_engine.rules["TEST_EVENT"] == action

def test_engine_subscribes_to_events_on_rule_addition(automation_engine, mock_metrics_engine):
    """Tests that adding a rule makes the engine subscribe to the MetricsEngine."""
    action = lambda event: None
    automation_engine.add_rule("CPU_EVENT", action)

    # Check that the subscribe method was called on the mock
    mock_metrics_engine.subscribe.assert_called_once_with("CPU_EVENT", automation_engine.handle_event)

def test_event_triggers_correct_resource_manager_action(automation_engine, mock_metrics_engine, mock_resource_manager):
    """Tests that a received event correctly triggers an action on the ResourceManager."""
    # Define a rule: if CPU is high, add 2 more cores.
    action = lambda event: mock_resource_manager.add_resources(cpu_cores=2, memory_gb=0)
    automation_engine.add_rule("CPU_HIGH_LOAD", action)

    # Simulate the MetricsEngine firing the event
    test_event = {"event_type": "CPU_HIGH_LOAD", "details": "some data"}
    # Manually call the handler that subscribe would have registered
    event_handler = mock_metrics_engine.subscribers["CPU_HIGH_LOAD"]
    event_handler(test_event)

    # Check that the correct action was called on the mock
    mock_resource_manager.add_resources.assert_called_once_with(cpu_cores=2, memory_gb=0)

def test_event_triggers_correct_virtualization_layer_action(automation_engine, mock_metrics_engine, mock_virtualization_layer):
    """Tests that an event can trigger an action on the VirtualizationLayer."""
    # Define a rule: if memory is low, trigger a (mocked) migration.
    action = lambda event: mock_virtualization_layer.migrate_ve("some_ve_id")
    automation_engine.add_rule("MEMORY_LOW", action)

    # Simulate the event
    test_event = {"event_type": "MEMORY_LOW", "details": "some data"}
    event_handler = mock_metrics_engine.subscribers["MEMORY_LOW"]
    event_handler(test_event)

    mock_virtualization_layer.migrate_ve.assert_called_once_with("some_ve_id")

def test_no_action_taken_for_unregistered_event(automation_engine, mock_resource_manager):
    """Tests that no action is taken for an event that has no rule."""
    # Define a rule for one event
    action = lambda event: mock_resource_manager.add_resources(cpu_cores=2, memory_gb=0)
    automation_engine.add_rule("CPU_HIGH_LOAD", action)

    # Simulate a DIFFERENT event
    test_event = {"event_type": "SOME_OTHER_EVENT", "details": "data"}
    automation_engine.handle_event(test_event)

    # Assert that the resource manager's method was NOT called
    mock_resource_manager.add_resources.assert_not_called()
