FROM docker.1ms.run/python:3.12-slim

SHELL ["bash", "-c"]

ENV PYTHONPATH=/app/src
WORKDIR /app

COPY . .
RUN python -m pip install -r /app/requirements.txt

CMD ["python3", "-m", "aenv.main", "/app/src"]