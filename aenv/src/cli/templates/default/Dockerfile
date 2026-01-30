# This is a test file
ARG AENV_BASE_REGISTRY=docker.io
ARG AENV_BASE_IMAGE=aenvironment/base
ARG AENV_BASE_VERSION=0.1.0

FROM ${AENV_BASE_REGISTRY}/${AENV_BASE_IMAGE}:${AENV_BASE_VERSION}

WORKDIR /app
ENV PYTHONPATH=/app/src

COPY . .
RUN python3 -m pip install --no-cache-dir -r /app/requirements.txt
ENTRYPOINT ["/bin/bash", "-c"]
CMD ["python3 -m aenv.main /app/src"]
