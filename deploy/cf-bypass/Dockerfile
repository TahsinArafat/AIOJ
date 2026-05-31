FROM python:3.11-slim

WORKDIR /app

# Install system dependencies for Chromium
RUN apt-get update && apt-get install -y \
    wget \
    gnupg \
    chromium \
    chromium-driver \
    xvfb \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
RUN pip install --no-cache-dir \
    DrissionPage \
    fastapi \
    uvicorn \
    python-multipart

# Copy server code
COPY server.py .

# Expose port
EXPOSE 8000

# Run with xvfb for headless browser
CMD ["bash", "-c", "Xvfb :99 -screen 0 1280x1024x24 &\nsleep 1\nexport DISPLAY=:99\npython server.py"]
