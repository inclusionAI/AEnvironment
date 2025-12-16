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
AEnv Hub client - Complete HTTP API wrapper

Provides CRUD operations and advanced features for AEnv Hub
"""

import json
import logging
import os
from enum import Enum
from typing import Any, Dict, List, Optional
from urllib.parse import urljoin

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

from cli.utils.cli_config import get_config_manager


class EnvStatus(Enum):
    PENDING = "pending"
    COMPLETED = "completed"
    UNKNOWN = "unknown"

    @classmethod
    def parse_state(cls, raw_state: str):
        try:
            return EnvStatus(raw_state)
        except ValueError:
            return EnvStatus.UNKNOWN

    def running(self):
        return self == EnvStatus.PENDING


class AEnvHubError(Exception):
    """AEnv Hub client exception base class"""

    pass


class AuthenticationError(AEnvHubError):
    """Authentication error"""

    pass


class NotFoundError(AEnvHubError):
    """Resource not found error"""

    pass


class ValidationError(AEnvHubError):
    """Data validation error"""

    pass


class RateLimitError(AEnvHubError):
    """Rate limit error"""

    pass


class AEnvHubClient:
    """
    AEnv Hub client

    Provides complete CRUD operations and advanced features including:
    - Environment variable CRUD operations
    - Batch operations
    - Version control
    - Permission management
    - Audit logs
    """

    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: int = 30,
        max_retries: int = 3,
        verify_ssl: bool = True,
        user_agent: str = "AEnv-Hub-Client/1.0.0",
    ):
        """
        Initialize client

        Args:
            base_url: AEnv Hub base URL
            api_key: API key for authentication
            timeout: Request timeout in seconds
            max_retries: Maximum retry attempts
            verify_ssl: Whether to verify SSL certificates
            user_agent: User agent string
        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self.user_agent = user_agent

        # Configure session
        self.session = requests.Session()
        self.session.headers.update(
            {
                "User-Agent": user_agent,
                "Content-Type": "application/json",
                "Accept": "application/json",
            }
        )

        if api_key:
            self.session.headers["Authorization"] = f"Bearer {api_key}"

        # Configure retry strategy
        retry_strategy = Retry(
            total=max_retries,
            status_forcelist=[429, 500, 502, 503, 504],
            backoff_factor=1,
        )
        adapter = HTTPAdapter(max_retries=retry_strategy)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)

        # Configure SSL verification
        self.session.verify = verify_ssl

        # Configure logging
        self.logger = logging.getLogger(__name__)

    def _make_request(
        self,
        method: str,
        endpoint: str,
        data: Optional[Dict[str, Any]] = None,
        params: Optional[Dict[str, Any]] = None,
        files: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Send HTTP request

        Args:
            method: HTTP method
            endpoint: API endpoint
            data: Request data
            params: URL parameters
            files: Uploaded files

        Returns:
            API response data

        Raises:
            AEnvHubError: Various API errors
        """
        url = urljoin(self.base_url, endpoint.lstrip("/"))

        try:
            if files:
                response = self.session.request(
                    method=method,
                    url=url,
                    data=data,
                    params=params,
                    files=files,
                    timeout=self.timeout,
                )
            else:
                response = self.session.request(
                    method=method,
                    url=url,
                    json=data,
                    params=params,
                    timeout=self.timeout,
                )

            # Handle response
            if response.status_code == 204:
                return {}

            try:
                json_data = response.json()
            except json.JSONDecodeError:
                json_data = {"message": response.text}

            # Error handling
            if response.status_code >= 400:
                self._handle_error_response(response.status_code, json_data)

            status = json_data.get("success", False)
            if not status:
                raise AEnvHubError(json_data)
            return json_data.get("data", {})

        except requests.exceptions.Timeout:
            raise AEnvHubError(f"Request timeout: {self.timeout} seconds")
        except requests.exceptions.ConnectionError as e:
            raise AEnvHubError(f"Connection error: {e}")
        except requests.exceptions.RequestException as e:
            raise AEnvHubError(f"Request error: {e}")

    def _handle_error_response(self, status_code: int, response_data: Dict[str, Any]):
        """Handle error response"""
        message = response_data.get("message", "Unknown error")

        if status_code == 401:
            raise AuthenticationError(message)
        elif status_code == 404:
            raise NotFoundError(message)
        elif status_code == 422:
            raise ValidationError(message)
        elif status_code == 429:
            raise RateLimitError(message)
        else:
            raise AEnvHubError(f"HTTP {status_code}: {message}")

    # ===== Environment Management CRUD Operations =====

    def check_env(self, name: str, version: str):
        response = self._make_request("GET", f"/env/{name}/{version}/exists")
        return response.get("exists", False)

    def list_environments(self, limit: int = 100, offset: int = 0):
        """List all environments"""
        params = {"limit": limit, "offset": offset}
        response = self._make_request("GET", "env", params=params)
        return response

    def get_environment(self, name: str, version: str) -> Dict[str, Any]:
        """Get specified environment details"""
        response = self._make_request("GET", f"/env/{name}/{version}")
        return response

    def state_environment(self, name: str, version: str):
        response = self._make_request("GET", f"/env/{name}/{version}/status")
        return response

    def create_environment(self, meta_data) -> Dict[str, Any]:
        """Create new environment
        meta_data = {
            "name": "envName",
            "version": "0.0.1",
            "tags": ["swe", "python", "linux"],
            "buildConfig": {"dockerfile": "./Dockerfile"},
            "testConfig": {"script": "pytest xxx"},
            "deployConfig": {"cpu": "1C", "memory": "2G", "os": "linux"},
            "status": "Ready",
            "codeUrl": "oss://xxx",  # Code file
        }
        """
        return self._make_request("POST", "/env", data=meta_data)

    def update_environment(self, meta_data):
        name = meta_data.get("name")
        version = meta_data.get("version")
        return self._make_request("PUT", f"/env/{name}/{version}", data=meta_data)

    def release_environment(self, name: str, version: str):
        response = self._make_request("POST", f"/env/{name}/{version}/release")
        return response

    def delete_environment(self, name: str) -> bool:
        """Delete environment"""
        self._make_request("DELETE", f"/env/{name}")
        return True

    # ===== Advanced Features =====
    def clone_environment(
        self, source_name: str, target_name: str, description: str = ""
    ) -> Dict[str, Any]:
        """Clone environment"""
        data = {
            "source_name": source_name,
            "target_name": target_name,
            "description": description,
        }
        return self._make_request("POST", "/api/v1/environments/clone", data=data)

    def search_environments(
        self, query: str, tags: Optional[List[str]] = None
    ) -> List[Dict[str, Any]]:
        """Search environments"""
        params = {"q": query}
        if tags:
            params["tags"] = ",".join(tags)
        response = self._make_request(
            "GET", "/api/v1/environments/search", params=params
        )
        return response.get("environments", [])

    def apply_sign_url(self, name, version, style=None):
        suffix = f"?style={style}" if style else ""
        endpoint = f"/env/{name}/{version}/sign{suffix}"
        response = self._make_request("POST", endpoint)
        return response

    _singleton_instance = None
    _singleton_lock = False

    @classmethod
    def load_client(cls, base_url: str = None, api_key: str = None):
        """
        Singleton method to create and return AEnvHubClient instance.

        This method implements the singleton pattern to ensure only one instance
        of AEnvHubClient is created and reused across the application.

        Configuration loading precedence:
        1. Custom values provided as arguments
        2. Global config file values
        3. Environment variables
        4. Default values

        Args:
            base_url: Optional custom base URL
            api_key: Optional custom API key

        Returns:
            Singleton AEnvHubClient instance
        """
        # Return existing singleton if already created and no custom args provided
        if cls._singleton_instance is not None and base_url is None and api_key is None:
            return cls._singleton_instance

        # If custom args provided, create new instance (bypass singleton for custom config)
        if base_url is not None or api_key is not None:
            return cls._create_client_instance(base_url, api_key)

        # Prevent recursive calls during singleton creation
        if cls._singleton_lock:
            raise RuntimeError(
                "Recursive call detected during AEnvHubClient singleton creation"
            )

        cls._singleton_lock = True
        try:
            if cls._singleton_instance is None:
                cls._singleton_instance = cls._create_client_instance()
            return cls._singleton_instance
        finally:
            cls._singleton_lock = False

    @classmethod
    def _create_client_instance(cls, base_url: str = None, api_key: str = None):
        """
        Internal method to create a new AEnvHubClient instance.

        Args:
            base_url: Optional custom base URL
            api_key: Optional custom API key

        Returns:
            New AEnvHubClient instance
        """
        # Use provided values or load from configuration
        if base_url is None or api_key is None:
            hub_config = get_config_manager().get_hub_config()

            if base_url is None:
                hub_backend = os.getenv("HUB_BACKEND")
                base_url = hub_backend if hub_backend else hub_config.get("hub_backend")

            if api_key is None:
                api_key = hub_config.get("api_key")

        return AEnvHubClient(
            base_url=base_url,
            api_key=api_key,
            timeout=hub_config.get("timeout", 30) if "hub_config" in locals() else 30,
        )

    @classmethod
    def reset_singleton(cls):
        """
        Reset the singleton instance (mainly for testing purposes).

        This method allows clearing the singleton instance, useful for testing
        scenarios where you need to create a fresh instance.
        """
        cls._singleton_instance = None
        cls._singleton_lock = False

    @classmethod
    def get_instance(cls):
        """
        Get the current singleton instance without creating a new one.

        Returns:
            Current singleton instance or None if not created
        """
        return cls._singleton_instance
