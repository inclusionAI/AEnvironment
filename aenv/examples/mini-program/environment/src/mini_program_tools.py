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
MCP tools for mini program development IDE.
Provides file system management and code execution capabilities.
"""

import io
import re
import sys
import traceback
from typing import Any, Dict

from aenv import register_tool

# Import vfs module - MCP server adds src/ to sys.path, so direct import works
try:
    from vfs import get_vfs
except ImportError:
    # Fallback: try relative import
    try:
        from .vfs import get_vfs
    except ImportError:
        # Last resort: import from same directory using file path
        import importlib.util
        from pathlib import Path

        vfs_path = Path(__file__).parent / "vfs.py"
        if vfs_path.exists():
            spec = importlib.util.spec_from_file_location("vfs", vfs_path)
            if spec and spec.loader:
                vfs_module = importlib.util.module_from_spec(spec)
                spec.loader.exec_module(vfs_module)
                get_vfs = vfs_module.get_vfs
            else:
                raise ImportError(f"Failed to load vfs module from {vfs_path}")
        else:
            raise ImportError(f"vfs.py not found at {vfs_path}")


@register_tool
def read_file(file_path: str) -> Dict[str, Any]:
    """
    Read a file from the virtual file system.

    Args:
        file_path: Path to the file (e.g., "app.js", "pages/index/index.js")

    Returns:
        Dictionary with file content and metadata:
        {
            "success": bool,
            "content": str,
            "file_path": str
        }
    """
    try:
        # Normalize file path - remove leading slashes and path components
        normalized_path = file_path.lstrip("/").split("/")[-1]

        vfs = get_vfs()
        content = vfs.read_file(normalized_path)
        return {
            "success": True,
            "content": content,
            "file_path": normalized_path,
            "size": len(content),
        }
    except FileNotFoundError:
        normalized_path = file_path.lstrip("/").split("/")[-1]
        return {
            "success": False,
            "error": f"File not found: {normalized_path}. Use list_files to see available files.",
            "file_path": file_path,
        }
    except Exception as e:
        return {
            "success": False,
            "error": f"Error reading file: {str(e)}",
            "file_path": file_path,
        }


@register_tool
def write_file(file_path: str, content: str) -> Dict[str, Any]:
    """
    Write content to a file in the virtual file system.

    Args:
        file_path: Path to the file (e.g., "index.html", "style.css", "game.js")
                  IMPORTANT: Use simple filenames without paths. The entry point MUST be "index.html".
        content: Content to write to the file

    Returns:
        Dictionary with operation result:
        {
            "success": bool,
            "file_path": str,
            "message": str,
            "size": int
        }
    """
    try:
        # Normalize file path - remove leading slashes and path components
        # Only allow simple filenames in root directory
        normalized_path = file_path.lstrip("/").split("/")[-1]

        # Warn if path was modified
        if normalized_path != file_path:
            import warnings

            warnings.warn(
                f"File path normalized from '{file_path}' to '{normalized_path}'. Use simple filenames only."
            )

        vfs = get_vfs()
        vfs.write_file(normalized_path, content)

        # Debug: Log VFS state after write
        import logging

        logger = logging.getLogger(__name__)
        logger.debug(
            f"VFS instance ID: {id(vfs)}, Files after write: {vfs.list_files()}"
        )

        # Validate HTML if it's index.html
        if normalized_path == "index.html":
            # Basic HTML validation
            if "<!DOCTYPE html>" not in content and "<html" not in content:
                return {
                    "success": True,
                    "file_path": normalized_path,
                    "message": f"File written: {normalized_path}. WARNING: Missing HTML5 doctype or html tag.",
                    "size": len(content),
                    "warning": "HTML structure may be incomplete",
                }

        return {
            "success": True,
            "file_path": normalized_path,
            "message": f"File written successfully: {normalized_path}",
            "size": len(content),
        }
    except Exception as e:
        return {
            "success": False,
            "error": f"Error writing file: {str(e)}",
            "file_path": file_path,
        }


@register_tool
def list_files(directory: str = "/") -> Dict[str, Any]:
    """
    List all files in the virtual file system.

    Args:
        directory: Directory path to list (default: "/" for root)

    Returns:
        Dictionary with list of files:
        {
            "success": bool,
            "files": List[str],
            "directory": str
        }
    """
    try:
        vfs = get_vfs()
        # Debug: Log VFS state
        import logging

        logger = logging.getLogger(__name__)
        logger.debug(
            f"VFS instance ID: {id(vfs)}, All files in VFS: {list(vfs.files.keys())}"
        )

        files = vfs.list_files(directory)
        logger.debug(f"list_files({directory}) returned: {files}")

        return {
            "success": True,
            "files": sorted(files),  # Sort for consistent output
            "directory": directory,
            "count": len(files),
        }
    except Exception as e:
        return {
            "success": False,
            "error": f"Error listing files: {str(e)}",
            "directory": directory,
        }


@register_tool
def execute_python_code(code: str) -> Dict[str, Any]:
    """
    Execute Python code and return the result.

    Args:
        code: Python code to execute

    Returns:
        Dictionary with execution result:
        {
            "success": bool,
            "output": str,
            "error": str (if any),
            "result": Any (if code returns a value)
        }
    """
    # Capture stdout and stderr
    old_stdout = sys.stdout
    old_stderr = sys.stderr
    stdout_capture = io.StringIO()
    stderr_capture = io.StringIO()

    try:
        sys.stdout = stdout_capture
        sys.stderr = stderr_capture

        # Execute code in a restricted environment
        local_vars = {}
        exec(code, {"__builtins__": __builtins__}, local_vars)

        output = stdout_capture.getvalue()
        error = stderr_capture.getvalue()

        # Try to get return value if exists
        result = local_vars.get("result", None)

        return {
            "success": True,
            "output": output,
            "error": error if error else None,
            "result": str(result) if result is not None else None,
        }
    except Exception:
        error_msg = traceback.format_exc()
        return {
            "success": False,
            "output": stdout_capture.getvalue(),
            "error": error_msg,
            "result": None,
        }
    finally:
        sys.stdout = old_stdout
        sys.stderr = old_stderr


@register_tool
def validate_html(html_content: str) -> Dict[str, Any]:
    """
    Validate HTML structure and syntax.

    Args:
        html_content: HTML content to validate

    Returns:
        Dictionary with validation result:
        {
            "success": bool,
            "valid": bool,
            "errors": List[str],
            "warnings": List[str],
            "suggestions": List[str]
        }
    """
    errors = []
    warnings = []
    suggestions = []

    # Check for HTML5 doctype
    if "<!DOCTYPE html>" not in html_content and "<!doctype html>" not in html_content:
        warnings.append("Missing HTML5 doctype declaration")
        suggestions.append("Add <!DOCTYPE html> at the beginning of the file")

    # Check for html tag
    if "<html" not in html_content.lower():
        errors.append("Missing <html> tag")

    # Check for head tag
    if "<head" not in html_content.lower():
        warnings.append("Missing <head> tag")

    # Check for body tag
    if "<body" not in html_content.lower():
        errors.append("Missing <body> tag")

    # Check for viewport meta tag (important for responsive design)
    if "viewport" not in html_content.lower():
        warnings.append("Missing viewport meta tag for responsive design")
        suggestions.append(
            'Add <meta name="viewport" content="width=device-width, initial-scale=1.0"> in <head>'
        )

    # Basic tag balance check (simple approach)
    open_tags = html_content.count("<")
    close_tags = html_content.count("</")
    self_closing_tags = html_content.count("/>")

    # Rough estimate - not perfect but catches obvious issues
    if open_tags - close_tags - self_closing_tags > 5:
        warnings.append("Possible unclosed tags detected")

    # Check for common issues
    if "position:fixed" in html_content and "overflow:hidden" not in html_content:
        suggestions.append(
            "Consider adding overflow:hidden to body when using position:fixed"
        )

    valid = len(errors) == 0

    return {
        "success": True,
        "valid": valid,
        "errors": errors,
        "warnings": warnings,
        "suggestions": suggestions,
    }


@register_tool
def check_responsive_design(
    html_content: str, css_content: str = None
) -> Dict[str, Any]:
    """
    Check if the design is responsive and follows best practices.

    Args:
        html_content: HTML content to check
        css_content: Optional CSS content to check (can be inline or separate)

    Returns:
        Dictionary with check results:
        {
            "success": bool,
            "is_responsive": bool,
            "issues": List[str],
            "fixed_pixels": List[str],  # Found fixed pixel values
            "responsive_units_used": bool,
            "suggestions": List[str]
        }
    """
    issues = []
    fixed_pixels = []
    responsive_units_used = False
    suggestions = []

    # Combine HTML and CSS for analysis
    content_to_check = html_content
    if css_content:
        content_to_check += "\n" + css_content

    # Check for viewport meta tag
    if "viewport" not in content_to_check.lower():
        issues.append("Missing viewport meta tag")
        suggestions.append(
            'Add <meta name="viewport" content="width=device-width, initial-scale=1.0">'
        )

    # Check for fixed pixel values in style attributes
    # Find fixed pixel values (simple regex)
    pixel_pattern = r"(\d+)px"
    matches = re.findall(pixel_pattern, content_to_check)

    # Filter out common acceptable uses (borders, small details)
    # Focus on layout-related pixels
    layout_keywords = [
        "width",
        "height",
        "margin",
        "padding",
        "top",
        "left",
        "right",
        "bottom",
        "font-size",
    ]
    for match in matches:
        # Check if it's near a layout keyword
        context_start = max(0, content_to_check.find(match) - 20)
        context_end = min(len(content_to_check), content_to_check.find(match) + 30)
        context = content_to_check[context_start:context_end].lower()

        if any(keyword in context for keyword in layout_keywords):
            if match not in fixed_pixels:
                fixed_pixels.append(match)

    if fixed_pixels:
        issues.append(
            f"Found {len(fixed_pixels)} fixed pixel value(s) that might affect responsiveness"
        )
        suggestions.append(
            "Consider using viewport units (vw, vh), percentages (%), or clamp() instead of fixed pixels"
        )

    # Check for responsive units
    responsive_patterns = ["vw", "vh", "%", "rem", "em", "clamp"]
    if any(pattern in content_to_check for pattern in responsive_patterns):
        responsive_units_used = True

    # Check for overflow hidden on body
    if "overflow:hidden" not in content_to_check.replace(" ", "").lower():
        suggestions.append(
            "Consider adding overflow:hidden to body to prevent scrolling"
        )

    # Check for flexbox or grid usage (good for responsive design)
    if (
        "display:flex" not in content_to_check.lower()
        and "display:grid" not in content_to_check.lower()
    ):
        suggestions.append("Consider using flexbox or grid for responsive layouts")

    is_responsive = (
        responsive_units_used
        and "viewport" in content_to_check.lower()
        and len(issues) == 0
    )

    return {
        "success": True,
        "is_responsive": is_responsive,
        "issues": issues,
        "fixed_pixels": fixed_pixels[:10],  # Limit to first 10
        "responsive_units_used": responsive_units_used,
        "suggestions": suggestions,
    }


@register_tool
def log_browser_console() -> Dict[str, Any]:
    """
    Get browser console logs from the preview iframe.
    Note: This requires the preview to be loaded and have console logging enabled.

    Returns:
        Dictionary with console logs:
        {
            "success": bool,
            "logs": List[Dict[str, str]],  # [{"type": "error|warn|log", "message": "..."}]
            "error_count": int,
            "warning_count": int
        }
    """
    # This would need to be implemented with browser automation or client-side cooperation
    # For now, return a placeholder that indicates this needs client-side support
    return {
        "success": True,
        "logs": [],
        "error_count": 0,
        "warning_count": 0,
        "message": "Browser console logging requires client-side implementation. Please check browser developer console manually.",
        "note": "To implement: Use postMessage API between iframe and parent window to capture console logs",
    }
