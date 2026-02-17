# Python sandbox image with uv. Follows uv Docker guide:
# https://docs.astral.sh/uv/guides/integration/docker/
FROM sandbox-runtime:base

USER root

# System dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3 \
        python3-pip \
        python3-venv \
        curl \
        git \
        wget \
        jq \
        && \
    rm -rf /var/lib/apt/lists/*

# Pin uv version for reproducible builds (docs: pin tag or SHA256)
# Note: ARG cannot be used in COPY --from; change the tag here to update.
COPY --from=ghcr.io/astral-sh/uv:0.10 /uv /uvx /bin/

RUN ln -sf /usr/bin/python3 /usr/local/bin/python

# Venv under /app so it is not hidden by the /workspace volume at runtime
RUN mkdir -p /app && chown sandbox:sandbox /app

# Create venv (docs: Using the pip interface → virtual environment)
ENV UV_LINK_MODE=copy
RUN --mount=type=cache,target=/root/.cache/uv \
    uv venv /app/.venv --python 3.11

# Use the venv so uv pip install targets it (omit --system and --python per docs)
ENV VIRTUAL_ENV=/app/.venv
ENV PATH="/app/.venv/bin:$PATH"

# Pre-install common packages with cache mount (docs: Caching)
# Optional: bytecode compilation for faster startup (docs: Compiling bytecode)
ENV UV_COMPILE_BYTECODE=1
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install \
    requests \
    httpx \
    pandas \
    numpy \
    matplotlib \
    beautifulsoup4 \
    lxml \
    pyyaml \
    python-dotenv

RUN chown -R sandbox:sandbox /app

USER sandbox

# PATH and VIRTUAL_ENV already set above; sandbox inherits them.

# Use workspace for cache and venv (writable volume; /tmp is small tmpfs and fills up)
ENV UV_CACHE_DIR="/workspace/.uv-cache"

# At runtime root fs is read-only: copy venv to workspace once so pip install works
RUN echo 'if [ ! -d /workspace/.venv/bin ]; then cp -a /app/.venv /workspace/.venv; fi && . /workspace/.venv/bin/activate' >> ~/.bashrc
