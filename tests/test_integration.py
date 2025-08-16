import pytest
from v_architect.resource_manager import ResourceManager
from v_architect.metrics_engine import MetricsEngine
from v_architect.automation_engine import AutomationEngine

def test_full_feedback_loop_integration():
    """
    An integration test to verify the complete feedback loop:
    ResourceManager -> MetricsEngine -> AutomationEngine -> ResourceManager
    """
    # 1. Setup all real components
    resource_manager = ResourceManager(total_cpu_cores=10, total_memory_gb=100)

    thresholds = {
        "CPU_OVERLOAD_75": lambda util: util["cpu_utilization_percent"] > 75,
    }
    metrics_engine = MetricsEngine(resource_manager, thresholds)

    # The virtualization layer is not needed for this specific feedback loop test
    from unittest.mock import MagicMock
    automation_engine = AutomationEngine(
        metrics_engine=metrics_engine,
        predictive_engine=MagicMock(), # Pass a mock for this test
        resource_manager=resource_manager,
        virtualization_layer=None # Passing None as it's not used in the action
    )

    # 2. Define a rule in the AutomationEngine
    # If CPU is overloaded, add 2 more cores to the pool.
    action = lambda event: resource_manager.add_resources(cpu_cores=2, memory_gb=0)
    automation_engine.add_rule("CPU_OVERLOAD_75", action)
    # Manually wire the subscription, as this is now the application's responsibility
    metrics_engine.subscribe("CPU_OVERLOAD_75", automation_engine.handle_event)

    # 3. Initial state check
    assert resource_manager.total_cpu_cores == 10

    # 4. Trigger the condition
    # Allocate 8 out of 10 cores to push utilization to 80%
    request_id = resource_manager.request_resources(cpu_cores=8, memory_gb=10, priority=5)
    resource_manager.process_queue()
    assert resource_manager.get_request_status(request_id) == "fulfilled"
    assert resource_manager.get_utilization()["cpu_utilization_percent"] == 80.0

    # 5. Run the monitoring check, which should trigger the event and action
    metrics_engine.check_metrics()

    # 6. Verify the outcome
    # The automation action should have been triggered, adding 2 cores.
    assert resource_manager.total_cpu_cores == 12
    # Also check available, which should be 12 total - 8 used = 4
    assert resource_manager.available_cpu_cores == 4


def test_full_predictive_feedback_loop():
    """
    An integration test for the full predictive feedback loop:
    ResourceManager -> MetricsEngine -> PredictiveEngine -> AutomationEngine -> ResourceManager
    """
    # 1. Setup
    from v_architect.predictive_engine import PredictiveEngine
    resource_manager = ResourceManager(total_cpu_cores=20, total_memory_gb=100)
    metrics_engine = MetricsEngine(resource_manager, thresholds={}, history_size=5)

    predictive_config = {
        "prediction_steps": 2,
        "thresholds": {
            "PREDICTIVE_CPU_DANGER": lambda f: f["cpu_utilization_percent"] > 60
        }
    }
    predictive_engine = PredictiveEngine(metrics_engine, predictive_config)

    automation_engine = AutomationEngine(metrics_engine, predictive_engine, resource_manager, None)

    # 2. Define a pre-emptive rule
    action = lambda event: resource_manager.add_resources(cpu_cores=10, memory_gb=0)
    automation_engine.add_rule("PREDICTIVE_CPU_DANGER", action)
    # Wire the subscription for the predictive event
    predictive_engine.subscribe("PREDICTIVE_CPU_DANGER", automation_engine.handle_event)

    # 3. Simulate a rising trend, but keep it BELOW any reactive thresholds
    # Initial state: 0% utilization
    # We will add 4 cores of usage at each step. Pool is 20 cores.
    # Step 1: 4 cores used (20%)
    # Step 2: 8 cores used (40%)
    # Step 3: 12 cores used (60%) -> The PREDICTION should now be > 60%
    for i in range(3):
        # Use different priorities to ensure they are processed
        resource_manager.request_resources(cpu_cores=4, memory_gb=1, priority=5-i)
        resource_manager.process_queue()
        metrics_engine.check_metrics()

    # Verify we are not yet in a reactive state
    assert resource_manager.get_utilization()["cpu_utilization_percent"] == 60.0

    # 4. Run the predictive analysis
    initial_total_cpu = resource_manager.total_cpu_cores
    assert initial_total_cpu == 20

    predictive_engine.analyze_and_predict()

    # 5. Verify the pre-emptive action was taken
    # The engine should have predicted the next step would be 80% (> 60% threshold)
    # and fired the event, which triggered the action to add 10 cores.
    assert resource_manager.total_cpu_cores == initial_total_cpu + 10 # 20 + 10 = 30
