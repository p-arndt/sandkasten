FROM sandbox-runtime:base

USER root
# Install Python, uv, and common development tools
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

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/
# Set up Python aliases
RUN ln -sf /usr/bin/python3 /usr/local/bin/python

USER sandbox

# Make uv available for non-root user
ENV PATH="/usr/local/bin:${PATH}"

RUN uv venv --python 3.11

ENV PATH="/app/.venv/bin:$PATH"
# Pre-install common Python packages to speed up agent work
RUN uv pip install \
    requests \
    httpx \
    pandas \
    numpy \
    matplotlib \
    beautifulsoup4 \
    lxml \
    pyyaml \
    python-dotenv
