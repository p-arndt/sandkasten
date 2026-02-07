FROM sandbox-runtime:base

USER root
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl && \
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*
USER sandbox
