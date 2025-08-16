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
    automation_engine = AutomationEngine(
        metrics_engine=metrics_engine,
        resource_manager=resource_manager,
        virtualization_layer=None # Passing None as it's not used in the action
    )

    # 2. Define a rule in the AutomationEngine
    # If CPU is overloaded, add 2 more cores to the pool.
    action = lambda event: resource_manager.add_resources(cpu_cores=2, memory_gb=0)
    automation_engine.add_rule("CPU_OVERLOAD_75", action)

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
