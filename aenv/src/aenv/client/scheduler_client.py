# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
AEnv Scheduler API client for AScheduler integration.
Implements the AScheduler API specification.
"""

import asyncio
from typing import Dict, List, Optional

import httpx

from aenv.core.exceptions import EnvironmentError, NetworkError
from aenv.core.logging import getLogger
from aenv.core.models import (
    APIResponse,
    EnvInstance,
    EnvInstanceCreateRequest,
    EnvInstanceListResponse,
    EnvService,
    EnvStatus,
)

logger = getLogger("aenv.scheduler_client", "colored")


class AEnvSchedulerClient:
    """Client for AEnv Scheduler API (AScheduler)."""

    def __init__(
        self,
        base_url: str,
        timeout: float = 30.0,
        max_retries: int = 3,
        api_key: Optional[str] = None,
    ):
        """
        Initialize AEnv Scheduler client.

        Args:
            base_url: Base URL for AEnv Scheduler API
            timeout: Request timeout in seconds
            max_retries: Maximum retry attempts
            api_key: Optional API key for authentication
        """
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.max_retries = max_retries
        self.api_key = api_key

        self._client: Optional[httpx.AsyncClient] = None

    async def __aenter__(self):
        """Async context manager entry."""
        await self.connect()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit."""
        await self.close()

    async def connect(self):
        """Initialize HTTP client."""
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
        }

        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        # Generate trace ID for logging
        import uuid

        trace_id = str(uuid.uuid4())
        headers["env-instance-trace-id"] = trace_id

        self._client = httpx.AsyncClient(
            base_url=f"{self.base_url}",
            timeout=httpx.Timeout(self.timeout),
            headers=headers,
            limits=httpx.Limits(
                max_keepalive_connections=20,
                max_connections=100,
            ),
        )

    async def close(self):
        """Close HTTP client."""
        if self._client:
            await self._client.aclose()
            self._client = None

    async def create_env_instance(
        self,
        name: str,
        datasource: str = "",
        ttl: str = "",
        environment_variables: Optional[Dict[str, str]] = None,
        arguments: Optional[List[str]] = None,
        owner: Optional[str] = None,
    ) -> EnvInstance:
        """
        Create a new environment instance.

        Args:
            name: Instance name
            datasource: Optional datasource
            environment_variables: Optional environment variables
            arguments: Optional arguments
            ttl: Time to live for instance
            owner: Optional owner of the instance
        Returns:
            Created EnvInstance

        Raises:
            EnvironmentError: If creation fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        logger.info(
            f"Creating environment instance: {name}, datasource: {datasource}, ttl: {ttl}, environment_variables: {environment_variables}, arguments: {arguments}, owner: {owner}, url: {self.base_url}"
        )
        request = EnvInstanceCreateRequest(
            envName=name,
            datasource=datasource,
            environment_variables=environment_variables,
            arguments=arguments,
            ttl=ttl,
            owner=owner,
        )

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.post(
                    "/env-instance",
                    json=request.model_dump(exclude_none=True),
                )

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        instance = EnvInstance(**api_response.data)
                        logger.info(f"Environment instance created: {instance.id}")
                        return instance
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(
                            f"Failed to create instance: {error_msg}"
                        )
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                import random

                if attempt < self.max_retries:
                    wait_time = 2**attempt + random.uniform(0, 1)
                    logger.warning(
                        f"Network error, retrying in {wait_time:.2f}s: {str(e)}"
                    )
                    await asyncio.sleep(wait_time)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def get_env_instance(self, instance_id: str) -> EnvInstance:
        """
        Get environment instance by ID.

        Args:
            instance_id: Environment instance ID

        Returns:
            EnvInstance details

        Raises:
            EnvironmentError: If instance not found
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        logger.debug(f"Querying environment instance: {instance_id}")
        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.get(f"/env-instance/{instance_id}")

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        instance = EnvInstance(**api_response.data)
                        logger.debug(
                            f"Instance status: {instance.id} -> {instance.status}"
                        )
                        return instance
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(f"Failed to get instance: {error_msg}")
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    wait_time = 2**attempt
                    logger.warning(f"Network error, retrying in {wait_time}s: {str(e)}")
                    await asyncio.sleep(wait_time)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def list_env_instances(
        self,
        pool_name: Optional[str] = None,
        status: Optional[EnvStatus] = None,
    ) -> List[EnvInstance]:
        """
        List environment instances.

        Args:
            pool_name: Optional pool name filter
            status: Optional status filter

        Returns:
            List of EnvInstance

        Raises:
            EnvironmentError: If listing fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        url = "/env-instance/list"
        if pool_name:
            url = f"/env-instance/{pool_name}/list"

        params = {}
        if status:
            params["status"] = status.value

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.get(url, params=params)

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        list_response = EnvInstanceListResponse(**api_response.data)
                        return list_response.items
                    else:
                        return []
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    await asyncio.sleep(2**attempt)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def delete_env_instance(self, instance_id: str) -> bool:
        """
        Delete environment instance.

        Args:
            instance_id: Environment instance ID

        Returns:
            True if deletion successful

        Raises:
            EnvironmentError: If deletion fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.delete(f"/env-instance/{instance_id}")

                try:
                    api_response = APIResponse(**response.json())
                    return api_response.success
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    await asyncio.sleep(2**attempt)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def wait_for_status(
        self,
        instance_id: str,
        target_status: EnvStatus,
        timeout: float = 300.0,
        check_interval: float = 2.0,
    ) -> EnvInstance:
        """
        Wait for environment instance to reach target status.

        Args:
            instance_id: Environment instance ID
            target_status: Target status to wait for
            timeout: Maximum wait time in seconds
            check_interval: Check interval in seconds

        Returns:
            EnvInstance when target status is reached

        Raises:
            EnvironmentError: If timeout or status error
        """
        start_time = asyncio.get_event_loop().time()

        while True:
            instance = await self.get_env_instance(instance_id)

            if instance.status == target_status:
                return instance

            if instance.status == EnvStatus.FAILED:
                raise EnvironmentError(f"Environment instance failed: {instance_id}")

            if asyncio.get_event_loop().time() - start_time > timeout:
                raise EnvironmentError(
                    f"Timeout waiting for status {target_status.value}: {instance_id}"
                )

            await asyncio.sleep(check_interval)

    # ========== Service Management Methods ==========

    async def create_env_service(
        self,
        name: str,
        service_name: Optional[str] = None,
        replicas: int = 1,
        environment_variables: Optional[Dict[str, str]] = None,
        owner: Optional[str] = None,
        # Storage configuration
        pvc_name: Optional[str] = None,
        mount_path: Optional[str] = None,
        storage_size: Optional[str] = None,  # If specified, PVC will be created
        # Service configuration
        port: Optional[int] = None,
        # Resource limits
        cpu_request: Optional[str] = None,
        cpu_limit: Optional[str] = None,
        memory_request: Optional[str] = None,
        memory_limit: Optional[str] = None,
        ephemeral_storage_request: Optional[str] = None,
        ephemeral_storage_limit: Optional[str] = None,
    ) -> "EnvService":
        """
        Create a new environment service (Deployment + Service + optionally PVC).

        Args:
            name: Service name (envName format: name@version)
            service_name: Optional custom service name. If not specified, will be auto-generated as "{envName}-svc-{random}".
                         Must follow Kubernetes DNS naming conventions.
            replicas: Number of replicas (default: 1, must be 1 if storage_size is specified)
            environment_variables: Optional environment variables
            owner: Optional owner of the service
            pvc_name: Optional PVC name (default: envName)
            mount_path: Optional mount path (default: /home/admin/data)
            storage_size: Optional storage size (e.g., "10Gi"). If specified, PVC will be created and replicas must be 1.
                         storageClass is configured in helm values.yaml deployment, not via API.
            port: Optional service port (default: 8080)
            cpu_request: Optional CPU request (default: 1)
            cpu_limit: Optional CPU limit (default: 2)
            memory_request: Optional memory request (default: 2Gi)
            memory_limit: Optional memory limit (default: 4Gi)
            ephemeral_storage_request: Optional ephemeral storage request (default: 5Gi)
            ephemeral_storage_limit: Optional ephemeral storage limit (default: 10Gi)

        Returns:
            Created EnvService

        Raises:
            EnvironmentError: If creation fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        from aenv.core.models import EnvServiceCreateRequest

        logger.info(
            f"Creating environment service: {name}, service_name: {service_name}, replicas: {replicas}, owner: {owner}"
        )
        request = EnvServiceCreateRequest(
            envName=name,
            service_name=service_name,
            replicas=replicas,
            environment_variables=environment_variables,
            owner=owner,
            pvc_name=pvc_name,
            mount_path=mount_path,
            storage_size=storage_size,
            port=port,
            cpu_request=cpu_request,
            cpu_limit=cpu_limit,
            memory_request=memory_request,
            memory_limit=memory_limit,
            ephemeral_storage_request=ephemeral_storage_request,
            ephemeral_storage_limit=ephemeral_storage_limit,
        )

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.post(
                    "/env-service",
                    json=request.model_dump(exclude_none=True),
                )

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        from aenv.core.models import EnvService

                        service = EnvService(**api_response.data)
                        logger.info(f"Environment service created: {service.id}")
                        return service
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(f"Failed to create service: {error_msg}")
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                import random

                if attempt < self.max_retries:
                    wait_time = 2**attempt + random.uniform(0, 1)
                    logger.warning(
                        f"Network error, retrying in {wait_time:.2f}s: {str(e)}"
                    )
                    await asyncio.sleep(wait_time)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def get_env_service(self, service_id: str) -> "EnvService":
        """
        Get environment service by ID.

        Args:
            service_id: Environment service ID

        Returns:
            EnvService details

        Raises:
            EnvironmentError: If service not found
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        logger.debug(f"Querying environment service: {service_id}")
        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.get(f"/env-service/{service_id}")

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        from aenv.core.models import EnvService

                        service = EnvService(**api_response.data)
                        logger.debug(
                            f"Service status: {service.id} -> {service.status}"
                        )
                        return service
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(f"Failed to get service: {error_msg}")
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    wait_time = 2**attempt
                    logger.warning(f"Network error, retrying in {wait_time}s: {str(e)}")
                    await asyncio.sleep(wait_time)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def list_env_services(
        self,
        env_name: Optional[str] = None,
    ) -> List["EnvService"]:
        """
        List environment services.

        Args:
            env_name: Optional environment name filter

        Returns:
            List of EnvService

        Raises:
            EnvironmentError: If listing fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        url = "/env-service/*/list"
        if env_name:
            url = f"/env-service/{env_name}/list"

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.get(url)

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        if isinstance(api_response.data, list):
                            from aenv.core.models import EnvService

                            return [EnvService(**item) for item in api_response.data]
                        return []
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(f"Failed to list services: {error_msg}")
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    await asyncio.sleep(2**attempt)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def delete_env_service(
        self, service_id: str, delete_storage: bool = False
    ) -> bool:
        """
        Delete environment service.

        Args:
            service_id: Environment service ID
            delete_storage: If True, also delete associated storage (PVC). Default False.

        Returns:
            True if deletion successful

        Raises:
            EnvironmentError: If deletion fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        # Build URL with query parameter if delete_storage is True
        url = f"/env-service/{service_id}"
        if delete_storage:
            url += "?deleteStorage=true"

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.delete(url)

                try:
                    api_response = APIResponse(**response.json())
                    return api_response.success
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    await asyncio.sleep(2**attempt)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e

    async def update_env_service(
        self,
        service_id: str,
        replicas: Optional[int] = None,
        image: Optional[str] = None,
        environment_variables: Optional[Dict[str, str]] = None,
    ) -> "EnvService":
        """
        Update environment service.

        Args:
            service_id: Environment service ID
            replicas: Optional number of replicas
            image: Optional container image
            environment_variables: Optional environment variables

        Returns:
            Updated EnvService

        Raises:
            EnvironmentError: If update fails
            NetworkError: If network request fails
        """
        if not self._client:
            raise NetworkError("Client not connected")

        from aenv.core.models import EnvServiceUpdateRequest

        request = EnvServiceUpdateRequest(
            replicas=replicas,
            image=image,
            environment_variables=environment_variables,
        )

        for attempt in range(self.max_retries + 1):
            try:
                response = await self._client.put(
                    f"/env-service/{service_id}",
                    json=request.model_dump(exclude_none=True),
                )

                try:
                    api_response = APIResponse(**response.json())
                    if api_response.success and api_response.data:
                        from aenv.core.models import EnvService

                        service = EnvService(**api_response.data)
                        logger.info(f"Environment service updated: {service.id}")
                        return service
                    else:
                        error_msg = api_response.get_error_message()
                        raise EnvironmentError(f"Failed to update service: {error_msg}")
                except ValueError as e:
                    raise EnvironmentError(
                        f"Invalid server response: {response.status_code} - {response.text[:200]}"
                    ) from e

            except httpx.RequestError as e:
                if attempt < self.max_retries:
                    wait_time = 2**attempt
                    logger.warning(f"Network error, retrying in {wait_time}s: {str(e)}")
                    await asyncio.sleep(wait_time)
                    continue
                raise NetworkError(f"Network error: {str(e)}") from e
