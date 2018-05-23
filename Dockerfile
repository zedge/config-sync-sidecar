FROM python:3.5

WORKDIR /app
COPY requirements.txt /app/
RUN pip install -r requirements.txt
COPY config_sidecar.py /app/

CMD ["python", "/app/config_sidecar.py"]
