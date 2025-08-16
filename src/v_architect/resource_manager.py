import heapq
import uuid
import itertools
from dataclasses import dataclass

@dataclass
class ResourcePool:
    """A data class to hold a set of resources."""
    cpu_cores: int = 0
    memory_gb: int = 0

class ResourceManager:
    """
    Manages a dynamic pool of resources with priority-based allocation.
    """

    def __init__(self, total_cpu_cores: int, total_memory_gb: int):
        self._total_pool = ResourcePool(total_cpu_cores, total_memory_gb)
        self._available_pool = ResourcePool(total_cpu_cores, total_memory_gb)

        self._priority_queue = [] # Min-heap used as a priority queue
        self._requests = {} # Tracks all requests by request_id
        self._allocations = {} # Tracks fulfilled allocations by allocation_id
        self._counter = itertools.count() # For FIFO tie-breaking

    @property
    def total_cpu_cores(self):
        return self._total_pool.cpu_cores

    @property
    def total_memory_gb(self):
        return self._total_pool.memory_gb

    @property
    def available_cpu_cores(self):
        return self._available_pool.cpu_cores

    @property
    def available_memory_gb(self):
        return self._available_pool.memory_gb

    def add_resources(self, cpu_cores: int, memory_gb: int):
        """Dynamically adds resources to the total and available pools."""
        self._total_pool.cpu_cores += cpu_cores
        self._total_pool.memory_gb += memory_gb
        self._available_pool.cpu_cores += cpu_cores
        self._available_pool.memory_gb += memory_gb

    def remove_resources(self, cpu_cores: int, memory_gb: int):
        """Dynamically removes resources from the pool."""
        if cpu_cores > self.available_cpu_cores or memory_gb > self.available_memory_gb:
            raise ValueError("Cannot remove more resources than are currently available.")

        self._total_pool.cpu_cores -= cpu_cores
        self._total_pool.memory_gb -= memory_gb
        self._available_pool.cpu_cores -= cpu_cores
        self._available_pool.memory_gb -= memory_gb

    def get_utilization(self) -> dict:
        """Calculates and returns the current resource utilization rates."""
        used_cpu = self._total_pool.cpu_cores - self._available_pool.cpu_cores
        used_mem = self._total_pool.memory_gb - self._available_pool.memory_gb

        cpu_util = (used_cpu / self._total_pool.cpu_cores * 100) if self._total_pool.cpu_cores > 0 else 0.0
        mem_util = (used_mem / self._total_pool.memory_gb * 100) if self._total_pool.memory_gb > 0 else 0.0

        return {
            "cpu_utilization_percent": round(cpu_util, 2),
            "memory_utilization_percent": round(mem_util, 2),
        }

    def request_resources(self, cpu_cores: int, memory_gb: int, priority: int) -> str:
        """Queues a request for resources and returns a request ID."""
        request_id = str(uuid.uuid4())
        count = next(self._counter) # For FIFO tie-breaking
        request_details = {"cpu_cores": cpu_cores, "memory_gb": memory_gb}

        # Use negative priority because heapq is a min-heap
        heap_item = (-priority, count, request_id)
        heapq.heappush(self._priority_queue, heap_item)

        self._requests[request_id] = {"status": "pending", "details": request_details}
        return request_id

    def process_queue(self) -> int:
        """Processes pending requests in the queue based on priority."""
        processed_count = 0
        re_queue = [] # To hold requests that can't be fulfilled yet

        while self._priority_queue:
            priority, count, request_id = heapq.heappop(self._priority_queue)
            request = self._requests[request_id]
            details = request["details"]

            if (details["cpu_cores"] <= self.available_cpu_cores and
                details["memory_gb"] <= self.available_memory_gb):

                # Fulfill the request
                self._available_pool.cpu_cores -= details["cpu_cores"]
                self._available_pool.memory_gb -= details["memory_gb"]

                allocation_id = str(uuid.uuid4())
                allocation = {
                    "allocation_id": allocation_id,
                    "cpu_cores": details["cpu_cores"],
                    "memory_gb": details["memory_gb"],
                }
                self._allocations[allocation_id] = allocation

                request["status"] = "fulfilled"
                request["allocation"] = allocation
                processed_count += 1
            else:
                # Cannot fulfill, put it back for later
                re_queue.append((priority, count, request_id))

        # Add unfulfilled requests back to the main queue
        for item in re_queue:
            heapq.heappush(self._priority_queue, item)

        return processed_count

    def get_request_status(self, request_id: str) -> str | None:
        """Returns the status of a given request."""
        return self._requests.get(request_id, {}).get("status")

    def get_allocation(self, request_id: str) -> dict | None:
        """Returns the allocation details for a fulfilled request."""
        request = self._requests.get(request_id)
        if request and request["status"] == "fulfilled":
            return request.get("allocation")
        return None

    def release_resources(self, allocation_id: str) -> bool:
        """Releases a fulfilled allocation and re-processes the queue."""
        if allocation_id not in self._allocations:
            return False

        allocation = self._allocations.pop(allocation_id)
        self._available_pool.cpu_cores += allocation["cpu_cores"]
        self._available_pool.memory_gb += allocation["memory_gb"]

        # After releasing resources, try to process the queue again
        self.process_queue()

        return True
