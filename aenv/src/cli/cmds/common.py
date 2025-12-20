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

import functools
import sys
import traceback
from typing import Any, Dict, NoReturn, Optional

import click
from rich.console import Group
from rich.panel import Panel
from rich.text import Text

from cli.utils.common.aenv_logger import get_logger
from cli.utils.common.console import CliConsole


class Config:
    """global config class"""

    def __init__(self):
        self.verbose = False
        self.quiet = True
        self.log_file = None
        self.json_logs = False
        self.console = CliConsole()
        self.cli_config = None
        self.debug = False


pass_config = click.make_pass_decorator(Config, ensure=True)

logger = get_logger(__name__)


class CLIError(Exception):
    """CLI error"""

    exit_code = 1

    def __init__(self, message: str):
        self.message = message
        super().__init__(message)


class ConfigError(CLIError):
    """Config error"""

    exit_code = 3


def exit_with_error(message: str, exit_code: int = 1) -> NoReturn:
    """exit with error"""
    click.secho(f"❌ Error: {message}", fg="red", err=True)
    sys.exit(exit_code)


def global_error_handler(func):
    """global error handler decorator"""

    @functools.wraps(func)
    def wrapper(*args, **kwargs):
        verbose = "--verbose" in sys.argv
        try:
            return func(*args, **kwargs)
        except ConfigError as e:
            logger.error(f"Config error: {e.message}")
            exit_with_error(e.message, e.exit_code)
        except KeyboardInterrupt:
            logger.info("Operation interrupted by user")
            click.secho("\n⚠️ Operation cancelled", fg="yellow", err=True)
            sys.exit(130)
        except click.exceptions.Exit:
            # Exit exceptions are normal (e.g., --help), let Click handle them
            raise
        except click.ClickException as e:
            _handle_click_error(e)
            sys.exit(e.exit_code)
        except ValidationError as e:
            ErrorHandler.handle_error(e)
            sys.exit(1)
        except Exception as e:
            if verbose:
                logger.exception("Unexpected error")
            exit_with_error(f"Unexpected error: {e}")

    return wrapper


class ValidationError(Exception):
    """Base exception for validation process"""

    def __init__(
        self,
        message: str,
        error_code: str = "VALIDATION_ERROR",
        details: Optional[Dict[str, Any]] = None,
        suggestion: Optional[str] = None,
    ):
        super().__init__(message)
        self.message = message
        self.error_code = error_code
        self.details = details or {}
        self.suggestion = suggestion
        self.work_dir = None
        self.inspector_port = None


class EnvironmentSetupError(ValidationError):
    """Environment setup error"""

    def __init__(self, message: str, **kwargs):
        super().__init__(message, error_code="ENV_SETUP_ERROR", **kwargs)


class DependencyError(ValidationError):
    """Dependency check error"""

    def __init__(self, message: str, **kwargs):
        super().__init__(message, error_code="DEPENDENCY_ERROR", **kwargs)


class MCPServerError(ValidationError):
    """MCP server error"""

    def __init__(self, message: str, **kwargs):
        super().__init__(message, error_code="MCP_SERVER_ERROR", **kwargs)


class ErrorHandler:
    """Unified error handler"""

    ERROR_SUGGESTIONS = {
        "ENV_SETUP_ERROR": {
            "title": "Environment Setup Issue",
            "suggestions": [
                "Check if the working directory exists and is accessible",
                "Ensure current user has permission to access the directory",
                "Try using absolute path for working directory",
            ],
        },
        "DEPENDENCY_ERROR": {
            "title": "Dependency Check Failed",
            "suggestions": [
                "Ensure Node.js (>=14.x) and npm are installed",
                "Run 'npm install -g @modelcontextprotocol/inspector' to manually install MCP Inspector",
                "Check network connectivity",
            ],
        },
        "MCP_SERVER_ERROR": {
            "title": "MCP Server Startup Failed",
            "suggestions": [
                "Check if valid aenv project configuration exists in working directory",
                "Ensure port {port} is not in use",
                "Check detailed logs for specific error reasons",
            ],
        },
        "VALIDATION_ERROR": {
            "title": "Validation Process Error",
            "suggestions": [
                "Check project configuration files are complete",
                "Ensure all required dependencies are installed",
                "Check detailed error information for more details",
            ],
        },
    }

    @classmethod
    def handle_error(cls, error: Exception) -> None:
        """Unified error handling with beautiful error output"""
        # Create error panel
        if isinstance(error, ValidationError):
            cls._handle_validation_error(error)
        else:
            cls._handle_unexpected_error(error)

    @classmethod
    def _handle_validation_error(cls, error: ValidationError) -> None:
        """Handle validation errors"""
        error_info = cls.ERROR_SUGGESTIONS.get(
            error.error_code, cls.ERROR_SUGGESTIONS["VALIDATION_ERROR"]
        )
        work_dir = error.work_dir
        inspector_port = error.inspector_port
        # Build error details
        details = []
        if error.details:
            for key, value in error.details.items():
                details.append(f"• {key}: {value}")

        # Build suggestions
        suggestions = error_info["suggestions"].copy()
        if error.suggestion:
            suggestions.insert(0, error.suggestion)

        # Format suggestions with variables
        formatted_suggestions = []
        for suggestion in suggestions:
            formatted_suggestions.append(suggestion.format(port=inspector_port))

        # Create error panel content
        content = Group(
            Text(f"Error Code: {error.error_code}", style="bold red"),
            Text(f"Error Description: {error.message}", style="red"),
            Text(""),
            Text("Context Information:", style="bold yellow"),
            Text(f"• Working Directory: {work_dir}"),
            Text(f"• Inspector Port: {inspector_port}"),
            Text(f"• Current Time: {cls._get_current_time()}"),
        )

        if details:
            content.renderables.append(Text(""))
            content.renderables.append(
                Text("Detailed Information:", style="bold yellow")
            )
            content.renderables.extend([Text(detail) for detail in details])

        content.renderables.append(Text(""))
        content.renderables.append(Text("Resolution Suggestions:", style="bold green"))
        content.renderables.extend(
            [Text(f"  {i + 1}. {s}") for i, s in enumerate(formatted_suggestions)]
        )

        # Display error panel
        panel = Panel(
            content,
            title=f"[bold red]❌ {error_info['title']}",
            border_style="red",
            expand=False,
        )

        CliConsole.console().print(panel)

    @classmethod
    def _handle_unexpected_error(cls, error: Exception) -> None:
        """Handle unexpected errors"""
        # Get stack trace information
        tb_str = "".join(
            traceback.format_exception(type(error), error, error.__traceback__)
        )

        content = Group(
            Text("Unexpected error occurred", style="bold red"),
            Text(f"Error Type: {type(error).__name__}", style="red"),
            Text(f"Error Message: {str(error)}", style="red"),
            Text(""),
            Text("Context Information:", style="bold yellow"),
            Text(f"• Python Version: {sys.version}"),
            Text(f"• Current Time: {cls._get_current_time()}"),
            Text(""),
            Text("Stack Trace:", style="bold yellow"),
            Text(tb_str, style="dim"),
        )

        panel = Panel(
            content, title="[bold red]❌ System Error", border_style="red", expand=False
        )

        CliConsole.console().print(panel)

        # Provide general suggestions
        CliConsole.info("💡 Suggestions:")
        CliConsole.console().print("  1. Check log files for more information")
        CliConsole.console().print(
            "  2. Ensure all dependencies are correctly installed"
        )
        CliConsole.console().print("  3. Try running the command again")
        CliConsole.console().print(
            "  4. If the issue persists, provide complete error information to technical support"
        )

    @staticmethod
    def _get_current_time() -> str:
        """Get current time string"""
        from datetime import datetime

        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def _handle_click_error(e: click.ClickException):
    """Handle Click Error"""
    if isinstance(e, click.UsageError):
        CliConsole.console().print(
            Panel(f"{e}", title="CLI error", border_style="yellow")
        )
    else:
        # Other Click Error
        CliConsole.console().print(
            Panel(f"[red]Error:[/red] {e}", title="Error", border_style="red")
        )
