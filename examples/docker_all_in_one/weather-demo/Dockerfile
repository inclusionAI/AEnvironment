FROM python:3.12-slim

WORKDIR /app
ENV PYTHONPATH=/app/src

COPY requirements.txt .
RUN python -m pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["python", "-m", "aenv.main", "src"]
