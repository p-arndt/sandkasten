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

# Install uv (fast Python package manager)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh && \
    mv /root/.local/bin/uv /usr/local/bin/uv && \
    chmod +x /usr/local/bin/uv

# Pre-install common Python packages to speed up agent work
RUN python3 -m pip install --no-cache-dir \
    requests \
    httpx \
    pandas \
    numpy \
    matplotlib \
    beautifulsoup4 \
    lxml \
    pyyaml \
    python-dotenv \
    && rm -rf /root/.cache/pip

# Set up Python aliases
RUN ln -sf /usr/bin/python3 /usr/local/bin/python

USER sandbox

# Make uv available for non-root user
ENV PATH="/usr/local/bin:${PATH}"

# Create a venv hint file for agents
RUN echo "Use 'uv venv' to create virtual environments" > /workspace/.uv-hint.txt
