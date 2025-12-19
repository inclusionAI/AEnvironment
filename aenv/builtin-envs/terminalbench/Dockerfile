FROM python:3.12-slim

WORKDIR /app
ENV PYTHONPATH=/app/src

RUN apt-get update && apt-get install -y --no-install-recommends python3-pip vim git lsof wget && rm -rf /var/lib/apt/lists/*
RUN wget -q https://github.com/containerd/nerdctl/releases/download/v2.1.6/nerdctl-2.1.6-linux-amd64.tar.gz && \
    tar -C /usr/local/bin -xzf nerdctl-2.1.6-linux-amd64.tar.gz && \
    chmod +x /usr/local/bin/nerdctl

COPY . .
RUN python -m pip install --no-cache-dir -r /app/requirements.txt
