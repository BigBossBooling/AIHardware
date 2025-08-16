import pytest
from unittest.mock import MagicMock
from v_architect.automation_engine import AutomationEngine

# --- Mocks for all dependencies ---

@pytest.fixture
def mock_metrics_engine():
    return MagicMock()

@pytest.fixture
def mock_predictive_engine():
    return MagicMock()

@pytest.fixture
def mock_resource_manager():
    return MagicMock()

@pytest.fixture
def mock_virtualization_layer():
    return MagicMock()

# --- AutomationEngine Tests ---

@pytest.fixture
def automation_engine(mock_metrics_engine, mock_predictive_engine, mock_resource_manager, mock_virtualization_layer):
    """Provides an AutomationEngine instance with mocked dependencies."""
    # Pass all engines to the constructor for simplicity in testing
    return AutomationEngine(
        metrics_engine=mock_metrics_engine,
        predictive_engine=mock_predictive_engine,
        resource_manager=mock_resource_manager,
        virtualization_layer=mock_virtualization_layer
    )

def test_add_rule(automation_engine):
    """Tests that a rule can be successfully added."""
    action = lambda event: None
    automation_engine.add_rule("TEST_EVENT", action)
    assert "TEST_EVENT" in automation_engine.rules
    assert automation_engine.rules["TEST_EVENT"] == action

def test_handle_event_triggers_correct_action(automation_engine, mock_resource_manager):
    """Tests that handle_event executes the correct action for a given event."""
    # Define a rule
    action = lambda event: mock_resource_manager.add_resources(cpu_cores=2, memory_gb=0)
    automation_engine.add_rule("CPU_HIGH_LOAD", action)

    # Manually fire the event
    test_event = {"event_type": "CPU_HIGH_LOAD", "details": "some data"}
    automation_engine.handle_event(test_event)

    # Check that the correct action was called
    mock_resource_manager.add_resources.assert_called_once_with(cpu_cores=2, memory_gb=0)

def test_handle_predictive_event_triggers_preemptive_action(automation_engine, mock_resource_manager):
    """Tests that a predictive event triggers a pre-emptive action."""
    # Define a rule for a predictive event
    action = lambda event: mock_resource_manager.add_resources(cpu_cores=4, memory_gb=10)
    automation_engine.add_rule("PREDICTIVE_CPU_OVERLOAD", action)

    # Manually fire the predictive event
    test_event = {"event_type": "PREDICTIVE_CPU_OVERLOAD", "details": "forecast data"}
    automation_engine.handle_event(test_event)

    # Check that the pre-emptive action was called
    mock_resource_manager.add_resources.assert_called_once_with(cpu_cores=4, memory_gb=10)

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
