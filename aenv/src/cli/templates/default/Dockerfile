ARG REGISTRY_PREFIX=
FROM ${REGISTRY_PREFIX}python:3.12-slim

WORKDIR /app
ENV PYTHONPATH=/app/src

COPY . .
RUN python -m pip install --no-cache-dir -r /app/requirements.txt
ENTRYPOINT ["/bin/bash", "-c"]
CMD ["python3 -m aenv.main /app/src"]
