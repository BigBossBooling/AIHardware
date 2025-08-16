"""
This module contains the ResourceManager class for V-Architect.
"""

import uuid

class ResourceManager:
    """
    Manages the allocation and release of virtual resources.
    """

    def __init__(self, total_cpu_cores: int, total_memory_gb: int, total_bandwidth_mbps: int):
        """
        Initializes the ResourceManager with a total pool of resources.

        Args:
            total_cpu_cores: The total number of virtual CPU cores available.
            total_memory_gb: The total amount of virtual memory in GB available.
            total_bandwidth_mbps: The total network bandwidth in Mbps available.
        """
        self.total_cpu_cores = total_cpu_cores
        self.total_memory_gb = total_memory_gb
        self.total_bandwidth_mbps = total_bandwidth_mbps

        self.available_cpu_cores = total_cpu_cores
        self.available_memory_gb = total_memory_gb
        self.available_bandwidth_mbps = total_bandwidth_mbps

        self._active_allocations = {}

    def request_resources(self, cpu_cores: int, memory_gb: int, bandwidth_mbps: int) -> dict | None:
        """
        Requests a set of resources.

        If the resources are available, they are allocated and a dictionary
        representing the allocation is returned. Otherwise, returns None.

        Args:
            cpu_cores: The number of CPU cores to allocate.
            memory_gb: The amount of memory in GB to allocate.
            bandwidth_mbps: The network bandwidth in Mbps to allocate.

        Returns:
            A dictionary with allocation details (including an 'allocation_id')
            if successful, otherwise None.
        """
        if (cpu_cores <= 0 or memory_gb <= 0 or bandwidth_mbps < 0):
             return None

        if (cpu_cores > self.available_cpu_cores or
            memory_gb > self.available_memory_gb or
            bandwidth_mbps > self.available_bandwidth_mbps):
            return None

        # Resources are available, so proceed with allocation
        self.available_cpu_cores -= cpu_cores
        self.available_memory_gb -= memory_gb
        self.available_bandwidth_mbps -= bandwidth_mbps

        allocation_id = str(uuid.uuid4())
        allocation = {
            "allocation_id": allocation_id,
            "cpu_cores": cpu_cores,
            "memory_gb": memory_gb,
            "bandwidth_mbps": bandwidth_mbps,
        }

        self._active_allocations[allocation_id] = allocation
        return allocation

    def release_resources(self, allocation_id: str) -> bool:
        """
        Releases a previously allocated set of resources.

        Args:
            allocation_id: The ID of the allocation to release.

        Returns:
            True if the release was successful, False otherwise.
        """
        if allocation_id not in self._active_allocations:
            return False

        allocation_to_release = self._active_allocations.pop(allocation_id)

        self.available_cpu_cores += allocation_to_release["cpu_cores"]
        self.available_memory_gb += allocation_to_release["memory_gb"]
        self.available_bandwidth_mbps += allocation_to_release["bandwidth_mbps"]

        return True
