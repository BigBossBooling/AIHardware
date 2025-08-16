import pytest
from v_architect.resource_manager import ResourceManager
from v_architect.virtualization_layer import VirtualizationLayer, VirtualEnvironment

@pytest.fixture
def manager():
    """Provides a ResourceManager instance for the tests."""
    return ResourceManager(total_cpu_cores=8, total_memory_gb=32)

@pytest.fixture
def v_layer(manager):
    """Provides a VirtualizationLayer instance linked to a resource manager."""
    return VirtualizationLayer(manager)

def test_ve_creation_queues_successfully(v_layer):
    """Tests that creating a VE returns a VE object in a 'pending' state."""
    ve = v_layer.create_ve(cpu_cores=2, memory_gb=4, priority=5)
    assert ve is not None
    assert isinstance(ve, VirtualEnvironment)
    assert ve.status == "pending"
    assert ve.ve_id in v_layer.get_all_ves()

def test_processing_updates_ve_to_running(v_layer, manager):
    """Tests that after processing the queue, the VE status updates to 'running'."""
    ve = v_layer.create_ve(cpu_cores=2, memory_gb=4, priority=5)
    assert ve.status == "pending"

    # Process the resource manager's queue
    manager.process_queue()

    # Manually trigger a status update in the virtualization layer
    v_layer.update_ve_statuses()

    updated_ve = v_layer.get_ve_details(ve.ve_id)
    assert updated_ve.status == "running"
    # Check that the allocation details are now populated
    assert updated_ve.allocation is not None

def test_create_ve_fails_if_request_invalid(v_layer, manager):
    """Tests that a VE for an unfulfillable request remains pending."""
    ve = v_layer.create_ve(cpu_cores=100, memory_gb=100, priority=5) # Impossible request
    manager.process_queue()
    v_layer.update_ve_statuses()

    # In a more complex system, this might become 'failed'. For now, it stays pending.
    assert ve.status == "pending"


def test_destroy_running_ve_releases_resources(v_layer, manager):
    """Tests that destroying a 'running' VE releases its resources."""
    ve = v_layer.create_ve(cpu_cores=4, memory_gb=8, priority=5)
    manager.process_queue()
    v_layer.update_ve_statuses()
    assert v_layer.get_ve_details(ve.ve_id).status == "running"
    assert manager.available_cpu_cores == 4

    # Now destroy it
    destroyed = v_layer.destroy_ve(ve.ve_id)
    assert destroyed is True

    # Check that resources are restored
    assert manager.available_cpu_cores == 8
    assert ve.ve_id not in v_layer.get_all_ves()

def test_cannot_destroy_pending_ve(v_layer):
    """Tests that a 'pending' VE cannot be destroyed (for now)."""
    ve = v_layer.create_ve(cpu_cores=2, memory_gb=4, priority=5)
    assert ve.status == "pending"

    destroyed = v_layer.destroy_ve(ve.ve_id)
    assert destroyed is False
