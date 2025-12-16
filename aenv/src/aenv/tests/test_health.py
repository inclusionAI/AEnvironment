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

import time
from datetime import datetime

import requests

URL = "http://localhost:8081/health"
TIMEOUT = 0.5
INTERVAL = 0.1

while True:
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")[:-3]
    try:
        resp = requests.get(URL, timeout=TIMEOUT)
        if resp.status_code == 200:
            print(f"{now} [INFO] health check passed")
        else:
            print(f"{now} [WARN] health returned {resp.status_code}")
    except requests.RequestException as e:
        print(f"{now} [ERROR] {e}")
    time.sleep(INTERVAL)
