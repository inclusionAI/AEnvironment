FROM python:3.12-slim

COPY ./ /app/
RUN python3 -m pip install --no-cache-dir -r /app/requirements.txt
ENV TAU2_DATA_DIR=/app/src/tau2-bench/data

WORKDIR /root/workspace

ENTRYPOINT ["python3", "-m", "aenv.main", "/app/src"]
