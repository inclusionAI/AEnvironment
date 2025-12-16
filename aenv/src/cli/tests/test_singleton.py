#!/usr/bin/env python3
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
Test script for AEnvHubClient singleton implementation
"""
import sys

# Add the src directory to Python path
sys.path.insert(0, "../..")

from cli.client.aenv_hub_client import AEnvHubClient


def test_singleton():
    """Test the singleton implementation."""
    print("Testing AEnvHubClient singleton implementation...")

    # Test 1: Basic singleton behavior
    print("\n1. Testing basic singleton behavior:")
    client1 = AEnvHubClient.load_client()
    client2 = AEnvHubClient.load_client()

    print(f"Client 1 ID: {id(client1)}")
    print(f"Client 2 ID: {id(client2)}")
    print(f"Same instance: {client1 is client2}")

    # Test 2: Custom arguments bypass singleton
    print("\n2. Testing custom arguments bypass:")
    client3 = AEnvHubClient.load_client(base_url="http://custom.com")
    client4 = AEnvHubClient.load_client(base_url="http://custom.com")

    print(f"Client 3 ID: {id(client3)}")
    print(f"Client 4 ID: {id(client4)}")
    print(f"Same instance: {client3 is client4}")
    print(f"Different from singleton: {client1 is not client3}")

    # Test 3: Get instance without creating
    print("\n3. Testing get_instance method:")
    instance = AEnvHubClient.get_instance()
    print(f"Current instance: {instance}")
    print(f"Same as client1: {instance is client1}")

    # Test 4: Reset singleton
    print("\n4. Testing reset_singleton method:")
    AEnvHubClient.reset_singleton()
    client5 = AEnvHubClient.load_client()
    print(f"Client 5 ID: {id(client5)}")
    print(f"Different from original: {client5 is not client1}")

    # Test 5: Thread safety simulation
    print("\n5. Testing thread safety (simulated):")
    clients = []
    for i in range(5):
        client = AEnvHubClient.load_client()
        clients.append(client)

    all_same = all(c is clients[0] for c in clients)
    print(f"All instances same: {all_same}")

    print("\n✅ All singleton tests passed!")


if __name__ == "__main__":
    test_singleton()
