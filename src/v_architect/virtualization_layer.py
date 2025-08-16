"""
This module contains the VirtualizationLayer and VirtualEnvironment classes.
"""

import uuid
from dataclasses import dataclass, field
from typing import Dict, Any
from .resource_manager import ResourceManager

@dataclass
class VirtualEnvironment:
    """A data class representing a single virtual environment."""
    ve_id: str
    status: str  # e.g., "pending", "running", "failed", "destroyed"
    request: dict
    allocation: Dict[str, Any] | None = None

    @property
    def cpu_cores(self) -> int:
        return self.request.get("cpu_cores", 0)

    @property
    def memory_gb(self) -> int:
        return self.request.get("memory_gb", 0)

class VirtualizationLayer:
    """
    Manages the lifecycle of virtual environments (VEs).
    """

    def __init__(self, resource_manager: ResourceManager):
        """Initializes the VirtualizationLayer with a ResourceManager."""
        self.resource_manager = resource_manager
        self._ves = {}  # Tracks all VEs by ve_id

    def create_ve(self, cpu_cores: int, memory_gb: int, priority: int) -> VirtualEnvironment:
        """
        Requests a new virtual environment.

        This creates a VE in a 'pending' state and queues a resource
        request with the ResourceManager.

        Returns:
            A VirtualEnvironment object in a 'pending' state.
        """
        request_details = {"cpu_cores": cpu_cores, "memory_gb": memory_gb, "priority": priority}
        request_id = self.resource_manager.request_resources(
            cpu_cores=cpu_cores,
            memory_gb=memory_gb,
            priority=priority
        )

        ve_id = str(uuid.uuid4())
        ve = VirtualEnvironment(
            ve_id=ve_id,
            status="pending",
            request={"id": request_id, **request_details}
        )
        self._ves[ve_id] = ve
        return ve

    def update_ve_statuses(self):
        """
        Polls the ResourceManager and updates the status of pending VEs.
        """
        for ve in self._ves.values():
            if ve.status == "pending":
                request_id = ve.request["id"]
                status = self.resource_manager.get_request_status(request_id)
                if status == "fulfilled":
                    ve.status = "running"
                    ve.allocation = self.resource_manager.get_allocation(request_id)
                # In a real system, we might also handle a "failed" status.

    def destroy_ve(self, ve_id: str) -> bool:
        """
        Destroys a 'running' virtual environment and releases its resources.

        Args:
            ve_id: The ID of the virtual environment to destroy.

        Returns:
            True if destruction was successful, False otherwise.
        """
        ve = self.get_ve_details(ve_id)
        if not ve or ve.status != "running" or ve.allocation is None:
            return False

        allocation_id = ve.allocation["allocation_id"]
        released = self.resource_manager.release_resources(allocation_id)

        if released:
            ve.status = "destroyed"
            del self._ves[ve_id]
            return True

        return False

    def get_all_ves(self) -> list[str]:
        """Returns a list of IDs for all virtual environments."""
        return list(self._ves.keys())

    def get_ve_details(self, ve_id: str) -> VirtualEnvironment | None:
        """Retrieves the VirtualEnvironment object for a given ID."""
        return self._ves.get(ve_id)
