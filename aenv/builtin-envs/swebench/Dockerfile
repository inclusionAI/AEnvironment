FROM python:3.12-slim

WORKDIR /app
ENV PYTHONPATH=/app/src


# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends wget && \
    rm -rf /var/lib/apt/lists/*

RUN wget -q https://github.com/containerd/nerdctl/releases/download/v2.1.6/nerdctl-2.1.6-linux-amd64.tar.gz && \
    tar -C /usr/local/bin -xzf nerdctl-2.1.6-linux-amd64.tar.gz && \
    chmod +x /usr/local/bin/nerdctl && \
    rm -rf nerdctl-2.1.6-linux-amd64.tar.gz


RUN mkdir -p /app/data && wget -q -O /app/data/swe-verify-details.json https://www.modelscope.cn/datasets/kangoal/swebench-scripts/resolve/master/swe-verify-details.json

COPY . .
RUN python -m pip install --no-cache-dir -r /app/requirements.txt
