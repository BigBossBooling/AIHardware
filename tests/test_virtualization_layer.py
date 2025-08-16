import pytest
from v_architect.resource_manager import ResourceManager
from v_architect.virtualization_layer import VirtualizationLayer, VirtualEnvironment

@pytest.fixture
def resource_manager():
    """Provides a ResourceManager instance for the tests."""
    return ResourceManager(total_cpu_cores=8, total_memory_gb=32, total_bandwidth_mbps=1000)

@pytest.fixture
def virtualization_layer(resource_manager):
    """Provides a VirtualizationLayer instance linked to a resource manager."""
    return VirtualizationLayer(resource_manager)

def test_virtualization_layer_initialization(virtualization_layer):
    """Tests that the VirtualizationLayer can be initialized."""
    assert virtualization_layer is not None
    assert virtualization_layer.resource_manager is not None

def test_create_virtual_environment_successfully(virtualization_layer, resource_manager):
    """Tests the successful creation of a new virtual environment."""
    ve = virtualization_layer.create_ve(cpu_cores=2, memory_gb=4, bandwidth_mbps=100)
    assert ve is not None
    assert isinstance(ve, VirtualEnvironment)
    assert ve.cpu_cores == 2
    assert ve.memory_gb == 4
    assert ve.bandwidth_mbps == 100
    assert ve.status == "running"

    # Check that resources were deducted from the manager
    assert resource_manager.available_cpu_cores == 6
    assert resource_manager.available_memory_gb == 28

    # Check that the VE is being tracked
    assert ve.ve_id in virtualization_layer.get_active_ves()

def test_create_ve_with_insufficient_resources(virtualization_layer, resource_manager):
    """Tests that creating a VE fails if the resource manager lacks resources."""
    ve = virtualization_layer.create_ve(cpu_cores=10, memory_gb=4, bandwidth_mbps=100)
    assert ve is None

    # Check that no resources were deducted
    assert resource_manager.available_cpu_cores == 8
    assert len(virtualization_layer.get_active_ves()) == 0

def test_destroy_virtual_environment(virtualization_layer, resource_manager):
    """Tests that destroying a virtual environment releases its resources."""
    # Create a VE first
    ve = virtualization_layer.create_ve(cpu_cores=4, memory_gb=8, bandwidth_mbps=200)
    assert ve is not None
    assert resource_manager.available_cpu_cores == 4
    assert resource_manager.available_memory_gb == 24
    assert len(virtualization_layer.get_active_ves()) == 1

    # Now destroy it
    ve_id = ve.ve_id
    destroyed_successfully = virtualization_layer.destroy_ve(ve_id)
    assert destroyed_successfully is True

    # Check that resources are restored
    assert resource_manager.available_cpu_cores == 8
    assert resource_manager.available_memory_gb == 32

    # Check that the VE is no longer tracked
    assert ve_id not in virtualization_layer.get_active_ves()
    # Check that the VE object status is updated
    assert ve.status == "destroyed"


def test_destroy_non_existent_ve(virtualization_layer):
    """Tests that trying to destroy a non-existent VE fails gracefully."""
    destroyed_successfully = virtualization_layer.destroy_ve("non-existent-id")
    assert destroyed_successfully is False

def test_get_ve_details(virtualization_layer):
    """Tests that we can retrieve the details of a specific virtual environment."""
    ve = virtualization_layer.create_ve(cpu_cores=1, memory_gb=2, bandwidth_mbps=50)
    assert ve is not None

    retrieved_ve = virtualization_layer.get_ve_details(ve.ve_id)
    assert retrieved_ve is not None
    assert retrieved_ve.ve_id == ve.ve_id
    assert retrieved_ve.cpu_cores == 1
    assert retrieved_ve.memory_gb == 2
