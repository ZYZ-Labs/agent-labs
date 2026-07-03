FROM python:3.11-slim

WORKDIR /app

# Install shared config and lab code.
COPY shared/config /app/shared/config
COPY labs/29-deployment/python /app

RUN pip install --no-cache-dir -r requirements.txt

EXPOSE 8000

ENV PYTHONUNBUFFERED=1
ENV LOG_LEVEL=INFO

CMD ["python", "main.py"]
