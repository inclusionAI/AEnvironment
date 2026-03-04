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

import logging
import sys
from typing import Literal, Optional

import colorlog

LOG_FORMAT = "%(asctime)s.%(msecs)03d %(name)s %(levelname)s: %(message)s"
DATE_FORMAT = "%Y%m%d-%H:%M:%S"
LOGLEVEL = logging.INFO

_STYLES = {
    "plain": {
        "DEBUG": "white",
        "INFO": "white",
        "WARNING": "yellow",
        "ERROR": "red",
        "CRITICAL": "bold_white,bg_red",
    },
    "colored": {
        "DEBUG": "blue",
        "INFO": "light_purple",
        "WARNING": "yellow",
        "ERROR": "red",
        "CRITICAL": "bold_white,bg_red",
    },
    "system": {
        "DEBUG": "blue",
        "INFO": "light_green",
        "WARNING": "yellow",
        "ERROR": "red",
        "CRITICAL": "bold_white,bg_red",
    },
    "benchmark": {
        "DEBUG": "light_black",
        "INFO": "light_cyan",
        "WARNING": "yellow",
        "ERROR": "red",
        "CRITICAL": "bold_white,bg_red",
    },
}


def _make_handler(style: str, level=LOGLEVEL) -> logging.Handler:
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)
    handler.setFormatter(
        colorlog.ColoredFormatter(
            fmt="%(log_color)s" + LOG_FORMAT,
            datefmt=DATE_FORMAT,
            log_colors=_STYLES[style],
        )
    )
    return handler


# The single parent logger for all SDK loggers.
# Upper layers can intercept ALL SDK logging by replacing this logger's handlers.
_root_logger = logging.getLogger("aenv")
_root_logger.setLevel(LOGLEVEL)
_root_logger.propagate = True

# Default: colored output to stdout (only added once)
if not _root_logger.handlers:
    _root_logger.addHandler(_make_handler("colored"))


def getLogger(
    name: Optional[str] = None,
    type_: Optional[Literal["plain", "benchmark", "colored", "system"]] = None,
) -> logging.Logger:
    """Get a named logger under the ``aenv`` namespace.

    All returned loggers are children of ``logging.getLogger("aenv")``,
    so replacing the handlers on that single logger is enough to redirect
    all SDK log output.

    When called standalone (no external logging setup), SDK logs go to
    stdout with colored output — same behavior as before.
    """
    if name is None:
        name = "aenv"
    elif not name.startswith("aenv"):
        name = f"aenv.{name}"

    logger = logging.getLogger(name)
    # Children propagate to the "aenv" parent — no per-logger handlers needed.
    # This is the key: upper layers only need to touch _root_logger.
    logger.propagate = True
    return logger


def setup_logging(
    handler: Optional[logging.Handler] = None,
    level: int = logging.DEBUG,
):
    """Replace SDK log handlers — the one-liner for upper-layer integration.

    Call this to redirect all aenv SDK logs to your own handler.
    After calling, the default colored-stdout handler is removed.

    Usage with loguru::

        from aenv.core.logging import setup_logging

        class InterceptHandler(logging.Handler):
            def emit(self, record):
                logger.opt(depth=6, exception=record.exc_info).log(
                    record.levelname, record.getMessage()
                )

        setup_logging(InterceptHandler())

    Usage with a file handler::

        setup_logging(logging.FileHandler("aenv.log"))

    Call with no arguments to disable SDK log output::

        setup_logging()
    """
    _root_logger.handlers.clear()
    _root_logger.setLevel(level)
    if handler is not None:
        _root_logger.addHandler(handler)
