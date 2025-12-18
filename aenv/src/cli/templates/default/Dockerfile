FROM python:3.12-slim

WORKDIR /app
ENV PYTHONPATH=/app/src

COPY . .
RUN python -m pip install --no-cache-dir -r /app/requirements.txt
