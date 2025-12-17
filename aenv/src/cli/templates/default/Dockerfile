FROM python:3.12-slim
ENV PYTHONPATH=/app/src
WORKDIR /app
RUN adduser --disabled-password --gecos '' appuser && \
    chown -R appuser:appuser /app
COPY . .

RUN pip install --no-cache-dir -r requirements.txt

USER appuser