import pytest
from v_architect.resource_manager import ResourceManager

# Fixture for a clean resource manager before each test
@pytest.fixture
def manager():
    """Provides a ResourceManager instance with a default set of resources."""
    return ResourceManager(total_cpu_cores=10, total_memory_gb=40)

# --- Tests for Dynamic Resource Pool ---

def test_add_resources(manager):
    """Tests that resources can be dynamically added to the pool."""
    manager.add_resources(cpu_cores=2, memory_gb=10)
    assert manager.total_cpu_cores == 12
    assert manager.total_memory_gb == 50
    assert manager.available_cpu_cores == 12
    assert manager.available_memory_gb == 50

def test_remove_resources_successfully(manager):
    """Tests that resources can be dynamically removed from the pool."""
    manager.remove_resources(cpu_cores=4, memory_gb=15)
    assert manager.total_cpu_cores == 6
    assert manager.total_memory_gb == 25
    assert manager.available_cpu_cores == 6
    assert manager.available_memory_gb == 25

def test_remove_more_resources_than_available_fails(manager):
    """Tests that removing more resources than are available fails."""
    with pytest.raises(ValueError):
        manager.remove_resources(cpu_cores=12, memory_gb=10)

def test_utilization_calculation(manager):
    """Tests the resource utilization calculation."""
    assert manager.get_utilization()["cpu_utilization_percent"] == 0.0

    # Request some resources
    request_id = manager.request_resources(cpu_cores=2, memory_gb=10, priority=1)
    manager.process_queue() # Process the queue to allocate

    assert manager.get_utilization()["cpu_utilization_percent"] == 20.0 # 2 used out of 10 total
    assert manager.get_utilization()["memory_utilization_percent"] == 25.0 # 10 used out of 40 total

# --- Tests for Priority Queue Allocation ---

def test_request_queues_successfully(manager):
    """Tests that a resource request is queued and returns a request ID."""
    request_id = manager.request_resources(cpu_cores=2, memory_gb=8, priority=3)
    assert isinstance(request_id, str)
    assert manager.get_request_status(request_id) == "pending"

def test_process_queue_fulfills_highest_priority_first(manager):
    """Tests that the queue is processed based on priority."""
    # Enqueue lower priority first. Request resources that would succeed alone.
    req_low_id = manager.request_resources(cpu_cores=6, memory_gb=25, priority=1)
    # Enqueue higher priority second. Total resources are not enough for both.
    req_high_id = manager.request_resources(cpu_cores=6, memory_gb=25, priority=5)

    # Process the queue
    processed_count = manager.process_queue()
    assert processed_count == 1 # Should only have resources for one

    # Check that the high priority request was fulfilled
    assert manager.get_request_status(req_high_id) == "fulfilled"
    assert manager.get_request_status(req_low_id) == "pending"
    assert manager.available_cpu_cores == 4 # 10 - 6

def test_process_queue_fifo_for_same_priority(manager):
    """Tests that requests with the same priority are first-in, first-out."""
    # Two requests with same priority, but not enough resources for both
    req1_id = manager.request_resources(cpu_cores=6, memory_gb=20, priority=5)
    req2_id = manager.request_resources(cpu_cores=6, memory_gb=20, priority=5)

    processed_count = manager.process_queue()
    assert processed_count == 1

    # The first one should be fulfilled
    assert manager.get_request_status(req1_id) == "fulfilled"
    assert manager.get_request_status(req2_id) == "pending"

def test_process_queue_with_insufficient_resources(manager):
    """Tests that the queue does not process if resources are insufficient for any request."""
    manager.request_resources(cpu_cores=12, memory_gb=50, priority=5)
    processed_count = manager.process_queue()
    assert processed_count == 0

def test_get_fulfilled_allocation(manager):
    """Tests retrieving the details of a fulfilled allocation."""
    request_id = manager.request_resources(cpu_cores=5, memory_gb=15, priority=1)
    manager.process_queue()

    assert manager.get_request_status(request_id) == "fulfilled"
    allocation = manager.get_allocation(request_id)
    assert allocation is not None
    assert allocation["cpu_cores"] == 5
    assert allocation["memory_gb"] == 15

def test_release_resources_triggers_queue_processing(manager):
    """Tests that releasing resources allows a pending request to be fulfilled."""
    # Fulfill a request that uses up most of the CPU
    req1_id = manager.request_resources(cpu_cores=8, memory_gb=30, priority=5)
    manager.process_queue()
    assert manager.get_request_status(req1_id) == "fulfilled"
    assert manager.available_cpu_cores == 2

    # Queue another request that can't be fulfilled yet
    req2_id = manager.request_resources(cpu_cores=4, memory_gb=5, priority=3)
    manager.process_queue()
    assert manager.get_request_status(req2_id) == "pending"

    # Now, release the first allocation
    allocation = manager.get_allocation(req1_id)
    manager.release_resources(allocation["allocation_id"])

    # The pending request should now be fulfilled because release triggers processing
    assert manager.get_request_status(req2_id) == "fulfilled"
    assert manager.available_cpu_cores == 6 # 10 - 4
