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
AEnv models for AScheduler integration.
Based on AScheduler API documentation.
"""

from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


class EnvStatus(str, Enum):
    """Environment instance status."""

    PENDING = "Pending"
    CREATING = "Creating"
    RUNNING = "Running"
    FAILED = "Failed"
    TERMINATED = "Terminated"


class ServiceStatus(str, Enum):
    """Environment service status."""

    PENDING = "Pending"
    CREATING = "Creating"
    RUNNING = "Running"
    UPDATING = "Updating"
    FAILED = "Failed"
    TERMINATED = "Terminated"


class Address(BaseModel):
    """Network address information."""

    ip: str
    port: int
    type: str = "network"
    session_id: Optional[str] = None


class Env(BaseModel):
    """Environment model."""

    id: str
    name: str
    description: str
    version: str
    tags: Optional[List[str]] = None
    code_url: str
    status: int
    artifacts: Optional[List[Dict[str, str]]] = None
    build_config: Optional[Dict] = None
    test_config: Optional[Dict] = None
    deploy_config: Optional[Dict] = None
    created_at: datetime
    updated_at: datetime


class EnvInstance(BaseModel):
    """Environment instance model for AScheduler."""

    id: str = Field(description="Instance id, corresponds to podname")
    env: Optional[Env] = Field(None, description="Environment object")
    status: str = Field(description="Instance status")
    created_at: str = Field(description="Creation time")
    updated_at: str = Field(description="Update time")
    ip: Optional[str] = Field(None, description="Instance IP")


class EnvInstanceCreateRequest(BaseModel):
    """Request to create an environment instance."""

    envName: str = Field(description="Environment name")
    datasource: str = Field(default="", description="Data source")
    ttl: str = Field(default="", description="time_to_live")
    environment_variables: Optional[Dict[str, str]] = (
        Field(None, description="Environment variables"),
    )
    arguments: Optional[List[str]] = (Field(None, description="Startup arguments"),)
    owner: Optional[str] = Field(None, description="Instance owner")


class EnvInstanceListResponse(BaseModel):
    """Response for listing environment instances."""

    items: List[EnvInstance]


class EnvService(BaseModel):
    """Environment service model (Deployment + Service + PVC)."""

    id: str = Field(description="Service id, corresponds to deployment name")
    env: Optional[Env] = Field(None, description="Environment object")
    status: str = Field(description="Service status")
    created_at: str = Field(description="Creation time")
    updated_at: str = Field(description="Update time")
    replicas: int = Field(description="Number of replicas")
    available_replicas: int = Field(description="Number of available replicas")
    service_url: Optional[str] = Field(None, description="Service URL")
    owner: Optional[str] = Field(None, description="Service owner")
    environment_variables: Optional[Dict[str, str]] = Field(
        None, description="Environment variables"
    )
    pvc_name: Optional[str] = Field(None, description="PVC name")


class EnvServiceCreateRequest(BaseModel):
    """Request to create an environment service."""

    envName: str = Field(description="Environment name")
    service_name: Optional[str] = Field(
        None,
        description="Custom service name. If not specified, will be auto-generated as '{envName}-svc-{random}'. Must follow Kubernetes DNS naming conventions.",
    )
    replicas: int = Field(default=1, description="Number of replicas")
    environment_variables: Optional[Dict[str, str]] = Field(
        None, description="Environment variables"
    )
    owner: Optional[str] = Field(None, description="Service owner")

    # Storage configuration
    pvc_name: Optional[str] = Field(None, description="PVC name (default: envName)")
    mount_path: Optional[str] = Field(
        None, description="Mount path (default: /home/admin/data)"
    )
    storage_size: Optional[str] = Field(
        None,
        description="Storage size (e.g., 10Gi). If specified, PVC will be created and replicas must be 1. storageClass is configured in helm deployment.",
    )

    # Service configuration
    port: Optional[int] = Field(None, description="Service port (default: 8080)")

    # Resource limits
    cpu_request: Optional[str] = Field(None, description="CPU request (default: 1)")
    cpu_limit: Optional[str] = Field(None, description="CPU limit (default: 2)")
    memory_request: Optional[str] = Field(
        None, description="Memory request (default: 2Gi)"
    )
    memory_limit: Optional[str] = Field(None, description="Memory limit (default: 4Gi)")
    ephemeral_storage_request: Optional[str] = Field(
        None, description="Ephemeral storage request (default: 5Gi)"
    )
    ephemeral_storage_limit: Optional[str] = Field(
        None, description="Ephemeral storage limit (default: 10Gi)"
    )


class EnvServiceUpdateRequest(BaseModel):
    """Request to update an environment service."""

    replicas: Optional[int] = Field(None, description="Number of replicas")
    image: Optional[str] = Field(None, description="Container image")
    environment_variables: Optional[Dict[str, str]] = Field(
        None, description="Environment variables"
    )


class EnvServiceListResponse(BaseModel):
    """Response for listing environment services."""

    items: List[EnvService]


class APIResponse(BaseModel):
    """Standard API response format."""

    success: bool = True
    code: Optional[int] = Field(None, description="Response code")
    message: Optional[str] = Field(None, description="Response message")
    data: Optional[Any] = None

    # Legacy fields for backwards compatibility
    error_code: Optional[int] = Field(None, alias="errorCode")
    error_message: Optional[str] = Field(None, alias="errorMessage")

    def get_error_message(self) -> str:
        """Get error message from either message or error_message field."""
        return self.message or self.error_message or "Unknown error"


class APIError(BaseModel):
    """API error response."""

    code: str
    message: str
    reason: Optional[str] = None
