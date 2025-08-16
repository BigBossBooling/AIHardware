import pytest
from v_architect.resource_manager import ResourceManager

@pytest.fixture
def resource_manager():
    """Provides a ResourceManager instance with a default set of total resources."""
    return ResourceManager(total_cpu_cores=8, total_memory_gb=32, total_bandwidth_mbps=1000)

def test_resource_manager_initialization(resource_manager):
    """Tests that the ResourceManager initializes with the correct total and available resources."""
    assert resource_manager.available_cpu_cores == 8
    assert resource_manager.available_memory_gb == 32
    assert resource_manager.available_bandwidth_mbps == 1000

def test_successful_resource_allocation(resource_manager):
    """Tests a simple, successful resource allocation."""
    allocation = resource_manager.request_resources(cpu_cores=2, memory_gb=4, bandwidth_mbps=100)
    assert allocation is not None
    assert "allocation_id" in allocation
    assert allocation["cpu_cores"] == 2
    assert allocation["memory_gb"] == 4
    assert allocation["bandwidth_mbps"] == 100

    # Check that available resources have been updated
    assert resource_manager.available_cpu_cores == 6
    assert resource_manager.available_memory_gb == 28
    assert resource_manager.available_bandwidth_mbps == 900

def test_failed_allocation_due_to_insufficient_cpu(resource_manager):
    """Tests that allocation fails when not enough CPU is available."""
    allocation = resource_manager.request_resources(cpu_cores=10, memory_gb=4, bandwidth_mbps=100)
    assert allocation is None
    # Ensure resources were not changed
    assert resource_manager.available_cpu_cores == 8

def test_failed_allocation_due_to_insufficient_memory(resource_manager):
    """Tests that allocation fails when not enough memory is available."""
    allocation = resource_manager.request_resources(cpu_cores=2, memory_gb=64, bandwidth_mbps=100)
    assert allocation is None
    assert resource_manager.available_memory_gb == 32

def test_resource_release(resource_manager):
    """Tests that resources are correctly returned to the pool upon release."""
    # First, allocate some resources
    allocation = resource_manager.request_resources(cpu_cores=4, memory_gb=8, bandwidth_mbps=200)
    assert allocation is not None
    assert resource_manager.available_cpu_cores == 4
    assert resource_manager.available_memory_gb == 24
    assert resource_manager.available_bandwidth_mbps == 800

    # Now, release them
    allocation_id = allocation["allocation_id"]
    released_successfully = resource_manager.release_resources(allocation_id)
    assert released_successfully is True

    # Check that resources are back to their original state
    assert resource_manager.available_cpu_cores == 8
    assert resource_manager.available_memory_gb == 32
    assert resource_manager.available_bandwidth_mbps == 1000

def test_release_of_non_existent_allocation(resource_manager):
    """Tests that trying to release a non-existent allocation fails gracefully."""
    released_successfully = resource_manager.release_resources("non-existent-id")
    assert released_successfully is False
    assert resource_manager.available_cpu_cores == 8 # No change

def test_multiple_allocations_and_releases(resource_manager):
    """Tests a sequence of allocations and a release."""
    alloc1 = resource_manager.request_resources(cpu_cores=2, memory_gb=8, bandwidth_mbps=300)
    assert alloc1 is not None
    assert resource_manager.available_cpu_cores == 6
    assert resource_manager.available_memory_gb == 24

    alloc2 = resource_manager.request_resources(cpu_cores=3, memory_gb=12, bandwidth_mbps=400)
    assert alloc2 is not None
    assert resource_manager.available_cpu_cores == 3
    assert resource_manager.available_memory_gb == 12

    # Release the first allocation
    resource_manager.release_resources(alloc1["allocation_id"])
    assert resource_manager.available_cpu_cores == 5 # 3 + 2
    assert resource_manager.available_memory_gb == 20 # 12 + 8

    # Release the second allocation
    resource_manager.release_resources(alloc2["allocation_id"])
    assert resource_manager.available_cpu_cores == 8
    assert resource_manager.available_memory_gb == 32
