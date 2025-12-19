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
Multi-task management tool
Manages MCP server and Inspector processes simultaneously
Supports graceful shutdown and signal handling
"""

import asyncio
import logging
import os
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import Any, Dict, List, Optional


class TaskManager:
    """Multi-task manager"""

    def __init__(self, logger: Optional[logging.Logger] = None):
        self.logger = logger or logging.getLogger(__name__)
        self.tasks: Dict[str, subprocess.Popen] = {}
        self.running = False
        self._shutdown_event = threading.Event()
        self._lock = threading.Lock()

    def add_task(
        self, name: str, command: List[str], env: Optional[Dict[str, str]] = None
    ) -> None:
        """Add task"""
        with self._lock:
            if name in self.tasks:
                self.logger.warning(f"Task {name} already exists, will replace")

            task_env = os.environ.copy()
            if env:
                task_env.update(env)

            self.logger.info(f"Starting task: {name} - {' '.join(command)}")
            process = subprocess.Popen(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=task_env,
                text=True,
            )
            self.tasks[name] = process

    def start_mcp_server(self, load_dir) -> None:
        """Start MCP server"""
        load_dir = load_dir if Path(load_dir).exists() else os.getcwd()
        command = ["python", "-m", "aenv.main", load_dir]

        if sys.platform == "darwin":
            command.extend(["--log-dir", "~/.aenv/aenv_test.log"])

        self.add_task("mcp_server", command)

    def start_inspector(self, port: int = 6274) -> None:
        """Start MCP Inspector"""
        command = ["npx", "@modelcontextprotocol/inspector", "--port", str(port)]
        self.add_task("inspector", command)

    def start_both(self, work_dir: str, inspector_port: int = 6274) -> None:
        """Start both MCP server and Inspector simultaneously"""
        self.start_mcp_server(work_dir)
        self.start_inspector(port=inspector_port)

    def is_task_running(self, name: str) -> bool:
        """Check if task is running"""
        with self._lock:
            if name not in self.tasks:
                return False
            return self.tasks[name].poll() is None

    def get_task_status(self) -> Dict[str, Dict[str, Any]]:
        """Get status of all tasks"""
        status = {}
        with self._lock:
            for name, process in self.tasks.items():
                return_code = process.poll()
                status[name] = {
                    "pid": process.pid,
                    "running": return_code is None,
                    "return_code": return_code,
                    "stdout": process.stdout is not None,
                    "stderr": process.stderr is not None,
                }
        return status

    def stop_task(self, name: str, force: bool = False) -> bool:
        """Stop individual task"""
        with self._lock:
            if name not in self.tasks:
                self.logger.warning(f"Task {name} does not exist")
                return False

            process = self.tasks[name]
            if process.poll() is not None:
                self.logger.info(f"Task {name} already stopped")
                return True

            try:
                if force:
                    process.kill()
                else:
                    process.terminate()

                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()

                self.logger.info(f"✅ Task {name} stopped")
                return True

            except Exception as e:
                self.logger.error(f"Failed to stop task {name}: {e}")
                return False

    def stop_all(self, force: bool = True) -> None:
        """Stop all tasks"""
        self.logger.info("🛑 Stopping all tasks...")
        # with self._lock:
        for name in list(self.tasks.keys()):
            self.stop_task(name, force=force)

    def wait_for_tasks(self) -> None:
        """Wait for all tasks to complete"""
        self.logger.info("⏳ Waiting for all tasks to complete...")
        with self._lock:
            for name, process in self.tasks.items():
                if process.poll() is None:
                    process.wait()
        self.logger.info("✅ All tasks completed")

    def monitor_tasks(self) -> None:
        """Monitor task output"""

        def monitor_output(name: str, stream):
            try:
                for line in iter(stream.readline, ""):
                    if line:
                        self.logger.info(f"[{name}] {line.strip()}")
            except Exception:
                pass

        with self._lock:
            for name, process in self.tasks.items():
                if process.stdout:
                    threading.Thread(
                        target=monitor_output, args=(name, process.stdout), daemon=True
                    ).start()
                if process.stderr:
                    threading.Thread(
                        target=monitor_output, args=(name, process.stderr), daemon=True
                    ).start()

    def start(self) -> None:
        """Start task manager"""
        if not self.tasks:
            self.logger.warning("No tasks to start")
            return

        self.running = True
        self.logger.info("🚀 Starting task manager...")

        # Start monitoring threads
        self.monitor_tasks()

        # Wait for signals
        try:
            while self.running and any(
                self.is_task_running(name) for name in self.tasks
            ):
                # self.logger.info("is running")
                time.sleep(1)
        except KeyboardInterrupt:
            self.logger.info("🛑 Received interrupt signal")
        # finally:
        #     self.stop_all()

    def run_until_complete(self) -> None:
        """Run until all tasks complete"""
        self.start()


class AsyncTaskManager:
    """Asynchronous task manager"""

    def __init__(self, logger: Optional[logging.Logger] = None):
        self.logger = logger or logging.getLogger(__name__)
        self.tasks: Dict[str, asyncio.subprocess.Process] = {}
        self.running = False

    async def add_task(
        self, name: str, command: List[str], env: Optional[Dict[str, str]] = None
    ) -> None:
        """Add asynchronous task"""
        task_env = os.environ.copy()
        if env:
            task_env.update(env)

        self.logger.info(f"Starting asynchronous task: {name} - {' '.join(command)}")
        process = await asyncio.create_subprocess_exec(
            *command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            env=task_env,
        )
        self.tasks[name] = process

    async def start_mcp_server_async(
        self, mode: str = "stdio", name: str = "AEnv-MCP-Server"
    ) -> None:
        """Start MCP server asynchronously"""
        command = [sys.executable, "-m", "cli.mcp", mode, "--name", name]
        await self.add_task("mcp_server", command)

    async def start_inspector_async(self, port: int = 6274) -> None:
        """Start MCP Inspector asynchronously"""
        command = ["npx", "@modelcontextprotocol/inspector", "--port", str(port)]
        await self.add_task("inspector", command)

    async def start_both_async(
        self, mcp_name: str = "AEnv-MCP-Server", inspector_port: int = 6274
    ) -> None:
        """Start both MCP server and Inspector asynchronously"""
        await self.start_mcp_server_async(name=mcp_name)
        await self.start_inspector_async(port=inspector_port)

    async def stop_task_async(self, name: str) -> None:
        """Stop task asynchronously"""
        if name not in self.tasks:
            return

        process = self.tasks[name]
        if process.returncode is None:
            self.logger.info(f"Stopping asynchronous task: {name}")
            process.terminate()
            try:
                await asyncio.wait_for(process.wait(), timeout=5)
            except asyncio.TimeoutError:
                process.kill()
                await process.wait()

    async def stop_all_async(self) -> None:
        """Stop all tasks asynchronously"""
        self.logger.info("🛑 Stopping all asynchronous tasks...")
        await asyncio.gather(
            *[self.stop_task_async(name) for name in list(self.tasks.keys())]
        )

    async def monitor_tasks_async(self) -> None:
        """Monitor tasks asynchronously"""

        async def monitor_output(name: str, stream):
            try:
                async for line in stream:
                    if line:
                        self.logger.info(f"[{name}] {line.decode().strip()}")
            except Exception:
                pass

        tasks = []
        for name, process in self.tasks.items():
            if process.stdout:
                tasks.append(monitor_output(name, process.stdout))
            if process.stderr:
                tasks.append(monitor_output(name, process.stderr))

        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

    async def run_async(self) -> None:
        """Run task manager asynchronously"""
        if not self.tasks:
            self.logger.warning("No asynchronous tasks to start")
            return

        self.running = True
        self.logger.info("🚀 Starting asynchronous task manager...")

        try:
            # Start monitoring
            monitor_task = asyncio.create_task(self.monitor_tasks_async())

            # Wait for all tasks to complete
            await asyncio.gather(*[process.wait() for process in self.tasks.values()])

            monitor_task.cancel()

        except asyncio.CancelledError:
            self.logger.info("🛑 Asynchronous task cancelled")
        except KeyboardInterrupt:
            self.logger.info("🛑 Received interrupt signal")
        finally:
            await self.stop_all_async()


class MCPManager:
    """MCP server and Inspector manager"""

    def __init__(self, logger: Optional[logging.Logger] = None):
        self.logger = logger or logging.getLogger(__name__)
        self.task_manager = TaskManager(logger)

    def start(
        self, work_dir: str, inspector_port: int = 6274, quiet: bool = False
    ) -> None:
        """Start MCP server and Inspector"""
        self.logger.debug("Starting MCP server and Inspector tasks...")

        # Set up signal handling
        def signal_handler(signum, frame):
            self.logger.info(
                f"🛑 Received signal {signum}, gracefully shutting down..."
            )
            self.task_manager.stop_all()
            sys.exit(0)

        signal.signal(signal.SIGINT, signal_handler)
        signal.signal(signal.SIGTERM, signal_handler)
        if quiet:
            self.task_manager.start_mcp_server(work_dir)
        else:
            self.task_manager.start_both(
                work_dir=work_dir, inspector_port=inspector_port
            )

        # Run until complete
        self.task_manager.run_until_complete()

    async def start_async(
        self, mcp_name: str = "AEnv-MCP-Server", inspector_port: int = 6274
    ) -> None:
        """Start MCP server and Inspector asynchronously"""
        async_task_manager = AsyncTaskManager(self.logger)

        # Set up signal handling
        def signal_handler(signum, frame):
            self.logger.info(
                f"🛑 Received signal {signum}, gracefully shutting down..."
            )
            asyncio.create_task(async_task_manager.stop_all_async())

        # Set up signal handling in event loop
        loop = asyncio.get_event_loop()
        for sig in [signal.SIGINT, signal.SIGTERM]:
            loop.add_signal_handler(sig, signal_handler)

        try:
            await async_task_manager.start_both_async(
                mcp_name=mcp_name, inspector_port=inspector_port
            )
            await async_task_manager.run_async()
        finally:
            await async_task_manager.stop_all_async()
