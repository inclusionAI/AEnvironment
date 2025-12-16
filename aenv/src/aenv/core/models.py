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
    tags: List[str]
    code_url: str
    status: int
    artifacts: List[Dict[str, str]]
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


class EnvInstanceListResponse(BaseModel):
    """Response for listing environment instances."""

    items: List[EnvInstance]


class APIResponse(BaseModel):
    """Standard API response format."""

    error_code: int = Field(0, alias="errorCode")
    error_message: str = Field("", alias="errorMessage")
    success: bool = True
    data: Optional[Any] = None


class APIError(BaseModel):
    """API error response."""

    code: str
    message: str
    reason: Optional[str] = None
