FROM criyle/go-judge:latest

RUN apk add --no-cache \
    g++ gcc make musl-dev \
    python3 python3-dev py3-pip \
    openjdk21-jdk \
    rust cargo \
    nodejs npm \
    mono mono-dev \
    ghc cabal \
    fpc \
    bash

RUN pip3 install --break-system-packages --no-cache-dir setuptools
