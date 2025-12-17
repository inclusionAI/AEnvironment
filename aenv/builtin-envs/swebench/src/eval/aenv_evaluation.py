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

import json
import logging
import os
import platform
import sys
from argparse import ArgumentParser
from pathlib import Path
from typing import Any

from constants import KEY_INSTANCE_ID, FAIL_TO_PASS, PASS_TO_PASS, EvalType, FAIL_ONLY_REPOS, \
    ResolvedStatus, \
    APPLY_PATCH_FAIL, APPLY_PATCH_PASS
from grading import get_eval_tests_report, get_resolution_status, get_logs_eval
from shell_runner import ShellExecutor
from swe_model import SweRun

root_tmp_dir = "/dev/shm" if 'linux' == platform.system().lower() else "/tmp"

class EvaluationError(Exception):
    def __init__(self, message: str):
        self.message = message


def setup_logger(log_file: Path, mode="w", add_stdout: bool = False):
    """
    This logger is used for logging the build process of images and containers.
    It writes logs to the log file.

    If `add_stdout` is True, logs will also be sent to stdout, which can be used for
    streaming ephemeral output from Modal containers.
    """
    log_file.parent.mkdir(parents=True, exist_ok=True)
    logger = logging.getLogger(f"{log_file.name}")
    handler = logging.FileHandler(log_file, mode=mode, encoding="utf-8")
    formatter = logging.Formatter("%(asctime)s - %(levelname)s - %(message)s")
    handler.setFormatter(formatter)
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    logger.propagate = False
    setattr(logger, "log_file", log_file)
    if add_stdout:
        handler = logging.StreamHandler(sys.stdout)
        formatter = logging.Formatter(
            f"%(asctime)s - {instance_id} - %(levelname)s - %(message)s"
        )
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    return logger


working_dir = Path(root_tmp_dir) / f"swe-sandbox"
working_dir.mkdir(parents=True, exist_ok=True)
str_working_dir = f"{root_tmp_dir}/swe-sandbox"

LOG_INSTANCE = "run_instance.log"
run_log_file = working_dir / LOG_INSTANCE

logger = None

executor = ShellExecutor(
    timeout=3600,
    env={"CUSTOM_VAR": "value"}
)


def run_instance_aenv(test_spec: SweRun):
    if not test_spec:
        return {"run_instance_err": "input swe data is invalid!"}

    global logger
    logger = setup_logger(run_log_file)
    executor.logger = logger

    instance_id = test_spec.instance_id
    logger.info(f"Running instance {instance_id}")

    patch_diff = test_spec.model_patch
    patch_file = f"{str_working_dir}/patch.diff"
    with open(patch_file, "w") as pf:
        if patch_diff and not patch_diff.endswith('\n'):
            patch_diff += '\n'
        pf.write(patch_diff)

    git_apply = f"cd /{test_spec.local_code_space} && git apply -v {patch_file}"
    apply_resp = executor.execute(git_apply)
    if not apply_resp.ok():
        logger.info("Failed to apply patch to container, trying again...")
        git_apply_special = f"cd /{test_spec.local_code_space} && patch --batch --fuzz=5 -p1 -i {patch_file}"
        special_resp = executor.execute(git_apply_special)

        if not special_resp.ok():
            logger.info(f"{APPLY_PATCH_FAIL}:\n{special_resp.stderr}")
            raise EvaluationError(f"Failed to apply patch to container, try again.")
        else:
            logger.info(f"{APPLY_PATCH_PASS}:\n{special_resp.stdout}")
    else:
        logger.info(f"{APPLY_PATCH_PASS}:\n{apply_resp.stdout}\n")

    # Get git diff before running eval script
    before_diff = f"cd /{test_spec.local_code_space} && git diff"
    before_diff_resp = executor.execute(before_diff)
    logger.info(
        f"Git diff before command status:{before_diff_resp.ok()},"
        f"output:{before_diff_resp.stdout},error:{before_diff_resp.stderr}")

    # eval_script support repository embed code file
    eval_file = test_spec.eval_file
    eval_script = test_spec.eval_script
    if os.path.exists(eval_script):
        eval_file = eval_script
    else:
        # django hack
        eval_script = eval_script.replace("locale-gen", "locale-gen en_US.UTF-8")
        with open(eval_file, "w") as ef:
            ef.write(eval_script)

    # run eval script and write stdout
    run_command = f"/bin/bash {eval_file}"
    test_output_path = working_dir / "test_output.txt"
    run_resp = executor.execute(run_command)
    logger.info(f"run eval script result:{run_resp}")
    with open(test_output_path, "w") as test_out:
        test_out.write(run_resp.stdout)

    # Get git diff after running eval script
    after_diff_resp = executor.execute(f"cd {test_spec.local_code_space} && git diff")
    # Check if git diff changed after running eval script
    logger.info(f"Git diff after:\n{after_diff_resp.stdout}")
    if after_diff_resp.stdout != before_diff_resp.stdout:
        logger.info("Git diff changed after running eval script")

    # Get report from test output
    logger.info(f"Grading answer for {instance_id}...")
    report = get_eval_report(
        test_spec=test_spec,
        test_log_path=str(test_output_path),
        include_tests_status=True,
    )
    logger.info(
        f"report: {report}\n"
        f"Result for {instance_id}: resolved: {report[instance_id]['resolved']}"
    )
    return report


def get_eval_report(
        test_spec: SweRun,
        test_log_path: str,
        include_tests_status: bool,
) -> dict[str, Any]:
    """
    Generate a report of model evaluation results from a prediction, task instance,
    and evaluation log.

    Args:
        test_log_path: path to evaluation log
        test_spec (dict): test spec containing keys "instance_id", "FAIL_TO_PASS", and "PASS_TO_PASS"
        include_tests_status (bool): whether to include the status of each test in the returned report
    Returns:
        report (dict): report of metrics
    """
    report_map = {}
    instance_id = test_spec.instance_id
    report_map[instance_id] = {
        "patch_is_None": False,
        "patch_exists": False,
        "patch_successfully_applied": False,
        "resolved": False,
    }

    # Check if the model patch exists
    if test_spec.model_patch is None:
        report_map[instance_id]["patch_is_None"] = True
        return report_map
    # if prediction[KEY_PREDICTION] is None:
    #     report_map[instance_id]["patch_is_None"] = True
    #     return report_map
    report_map[instance_id]["patch_exists"] = True

    # Get evaluation logs
    eval_status_map, found = get_logs_eval(test_spec, test_log_path)

    if not found:
        return report_map
    report_map[instance_id]["patch_successfully_applied"] = True

    eval_ref = {
        KEY_INSTANCE_ID: test_spec.instance_id,
        FAIL_TO_PASS: test_spec.FAIL_TO_PASS,
        PASS_TO_PASS: test_spec.PASS_TO_PASS,
    }

    eval_type = EvalType.FAIL_ONLY if test_spec.repo in FAIL_ONLY_REPOS \
        else EvalType.PASS_AND_FAIL

    report = get_eval_tests_report(
        eval_status_map, eval_ref, eval_type=eval_type
    )
    if get_resolution_status(report) == ResolvedStatus.FULL.value:
        report_map[instance_id]["resolved"] = True

    if include_tests_status:
        report_map[instance_id]["tests_status"] = report  # type: ignore

    return report_map


def main(details):
    with open(details, "r") as swe:
        details = json.loads(swe.read())
    instance = details["instance_id"]
    run = SweRun(instance, details.get("repo"),
                 details.get("version"), details.get("model_patch"),
                 details.get("script"), details.get("test_cmd"))
    run.PASS_TO_PASS = json.loads(details.get("PASS_TO_PASS"))
    run.FAIL_TO_PASS = json.loads(details.get("FAIL_TO_PASS"))
    report = run_instance_aenv(run)
    print(json.dumps(report, indent=2))
    status = report.get(instance, {}).get("resolved")
    if not status:
        sys.exit(1)


if __name__ == "__main__":
    parser = ArgumentParser()
    parser.add_argument(
        "--details",
        type=str,
        default="/shared/details.json",
        help="",
    )
    args = parser.parse_args()
    main(**vars(args))
