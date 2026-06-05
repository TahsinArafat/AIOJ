FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    xvfb procps dbus ca-certificates \
    libglib2.0-0 libgobject-2.0-0 libnspr4 libnss3 \
    libatk1.0-0 libatk-bridge2.0-0 libcups2 libxkbcommon0 \
    libatspi2.0-0 libxcomposite1 libxdamage1 libxfixes3 \
    libcairo2 libpango-1.0-0 libasound2 \
    fonts-noto-color-emoji fonts-freefont-ttf fonts-unifont \
    fonts-ipafont-gothic fonts-wqy-zenhei fonts-tlwg-loma-otf \
    && fc-cache -f \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir cloakbrowser[geoip] fastapi uvicorn requests

RUN python -m cloakbrowser install

COPY server.py .

ENV DISPLAY=:99

EXPOSE 8002

CMD ["bash", "-c", "rm -f /tmp/.X99-lock && Xvfb :99 -screen 0 1920x1080x24 -ac +extension GLX +render -noreset & sleep 2 && export DISPLAY=:99 && python server.py"]
