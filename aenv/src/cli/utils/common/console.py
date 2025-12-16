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

from typing import Optional

from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table
from rich.theme import Theme

# custom theme
CUSTOM_THEME = Theme(
    {
        "info": "cyan",
        "warning": "yellow",
        "error": "red",
        "success": "green",
        "highlight": "magenta bold",
    }
)

console = Console(theme=CUSTOM_THEME, highlight=False)


class CliConsole:
    """CLI custom console"""

    @staticmethod
    def console():
        return console

    @staticmethod
    def info(message: str, **kwargs) -> None:
        """info message"""
        console.print(f"[info]ℹ️  {message}[/info]", **kwargs)

    @staticmethod
    def success(message: str, **kwargs) -> None:
        """success message"""
        console.print(f"[success]✅ {message}[/success]", **kwargs)

    @staticmethod
    def warning(message: str, **kwargs) -> None:
        """warning message"""
        console.print(f"[warning]⚠️  {message}[/warning]", **kwargs)

    @staticmethod
    def error(message: str, **kwargs) -> None:
        """error message"""
        console.print(f"[error]❌ {message}[/error]", **kwargs)

    @staticmethod
    def print_table(data: list, headers: list, title: Optional[str] = None) -> None:
        """print table data"""
        table = Table(title=title, show_header=True, header_style="bold magenta")
        for header in headers:
            table.add_column(header)

        for row in data:
            table.add_row(*[str(cell) for cell in row])

        console.print(table)

    @staticmethod
    def progress_spinner(description: str = "Processing..."):
        """progress spinner"""
        return Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=console,
        )

    @staticmethod
    def confirm(message: str, default: bool = True) -> bool:
        """confirm message"""
        from rich.prompt import Confirm

        return Confirm.ask(message, default=default, console=console)

    @staticmethod
    def prompt(message: str, default: Optional[str] = None) -> str:
        """prompt message"""
        from rich.prompt import Prompt

        return Prompt.ask(message, default=default, console=console)
