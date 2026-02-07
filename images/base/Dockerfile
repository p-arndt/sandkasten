FROM ubuntu:24.04

RUN useradd -m -s /bin/bash sandbox && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        bash coreutils procps ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY runner /usr/local/bin/runner
RUN chmod +x /usr/local/bin/runner

RUN mkdir /workspace && chown sandbox:sandbox /workspace && \
    mkdir -p /run && chmod 777 /run

USER sandbox
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/runner"]
