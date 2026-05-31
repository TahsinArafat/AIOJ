FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    wget gnupg chromium chromium-driver xvfb procps dbus \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir cloakbrowser fastapi uvicorn

COPY server.py .

ENV DISPLAY=:99

EXPOSE 8002

CMD ["bash", "-c", "rm -f /tmp/.X99-lock && Xvfb :99 -screen 0 1920x1080x24 &\nsleep 2\nexport DISPLAY=:99\npython server.py"]
