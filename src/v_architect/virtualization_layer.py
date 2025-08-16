"""
This module contains the VirtualizationLayer and VirtualEnvironment classes.
"""

import uuid
from dataclasses import dataclass
from .resource_manager import ResourceManager

@dataclass
class VirtualEnvironment:
    """A data class representing a single virtual environment."""
    ve_id: str
    cpu_cores: int
    memory_gb: int
    bandwidth_mbps: int
    status: str
    _allocation_id: str # Internal link to the resource manager's allocation

class VirtualizationLayer:
    """
    Manages the lifecycle of virtual environments (VEs).

    This layer acts as an orchestrator, requesting resources from a
    ResourceManager and binding them to conceptual virtual environments.
    """

    def __init__(self, resource_manager: ResourceManager):
        """
        Initializes the VirtualizationLayer with a given ResourceManager.

        Args:
            resource_manager: An instance of ResourceManager to handle resource allocations.
        """
        self.resource_manager = resource_manager
        self._active_ves = {} # Tracks active VirtualEnvironment objects

    def create_ve(self, cpu_cores: int, memory_gb: int, bandwidth_mbps: int) -> VirtualEnvironment | None:
        """
        Creates a new virtual environment.

        This involves requesting resources from the ResourceManager and, if
        successful, creating a VirtualEnvironment object to represent the VE.

        Args:
            cpu_cores: The number of CPU cores required.
            memory_gb: The amount of memory in GB required.
            bandwidth_mbps: The network bandwidth in Mbps required.

        Returns:
            A VirtualEnvironment object if creation is successful, otherwise None.
        """
        allocation = self.resource_manager.request_resources(
            cpu_cores=cpu_cores,
            memory_gb=memory_gb,
            bandwidth_mbps=bandwidth_mbps
        )

        if allocation is None:
            return None # Resource allocation failed

        ve_id = str(uuid.uuid4())
        ve = VirtualEnvironment(
            ve_id=ve_id,
            cpu_cores=cpu_cores,
            memory_gb=memory_gb,
            bandwidth_mbps=bandwidth_mbps,
            status="running",
            _allocation_id=allocation["allocation_id"]
        )

        self._active_ves[ve_id] = ve
        return ve

    def destroy_ve(self, ve_id: str) -> bool:
        """
        Destroys an existing virtual environment and releases its resources.

        Args:
            ve_id: The ID of the virtual environment to destroy.

        Returns:
            True if the destruction was successful, False otherwise.
        """
        if ve_id not in self._active_ves:
            return False

        ve_to_destroy = self._active_ves.pop(ve_id)

        # Release the resources back to the manager
        released = self.resource_manager.release_resources(ve_to_destroy._allocation_id)

        if released:
            ve_to_destroy.status = "destroyed"
            return True
        else:
            # This case should ideally not happen if the VE is tracked properly.
            # Put the VE back in the dict to maintain a consistent state.
            self._active_ves[ve_id] = ve_to_destroy
            return False


    def get_active_ves(self) -> list[str]:
        """Returns a list of IDs for all active virtual environments."""
        return list(self._active_ves.keys())

    def get_ve_details(self, ve_id: str) -> VirtualEnvironment | None:
        """
        Retrieves the VirtualEnvironment object for a given ID.

        Args:
            ve_id: The ID of the virtual environment.

        Returns:
            The VirtualEnvironment object, or None if not found.
        """
        return self._active_ves.get(ve_id)
