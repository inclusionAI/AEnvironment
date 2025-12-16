FROM python:3.12-slim
ENV PYTHONPATH=/app/src

COPY ./ /app/
RUN python3 -m pip install --no-cache-dir -r /app/requirements.txt

WORKDIR /root/workspace
ENTRYPOINT ["python3", "-m", "aenv.main", "/app/src"]
