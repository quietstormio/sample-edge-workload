FROM python:3.10-slim

WORKDIR /app

# Install system dependencies and Python packages with aggressive optimization
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/* \
    # Install PyTorch CPU-only (saves ~500MB vs CUDA)
    && pip install --no-cache-dir \
        torch==2.1.0 torchvision==0.16.0 --index-url https://download.pytorch.org/whl/cpu \
    # Install opencv-python-headless first
    && pip install --no-cache-dir opencv-python-headless \
    # Install ultralytics without dependencies to avoid opencv-python
    && pip install --no-cache-dir --no-deps ultralytics \
    # Manually install only required ultralytics dependencies
    && pip install --no-cache-dir \
        ultralytics-thop>=2.0.0 \
        matplotlib>=3.3.0 \
        pyyaml>=5.3.1 \
        tqdm>=4.64.0 \
        psutil \
        py-cpuinfo \
        pandas>=1.1.4 \
        seaborn>=0.11.0 \
    # Force install numpy<2 LAST to ensure compatibility
    && pip install --no-cache-dir --force-reinstall "numpy<2" \
    # Remove opencv-python if it was installed
    && pip uninstall -y opencv-python || true \
    # Aggressive cleanup
    && rm -rf /root/.cache/pip \
    && find /usr/local -type d -name '__pycache__' -exec rm -rf {} + 2>/dev/null || true \
    && find /usr/local -type f -name '*.pyc' -delete \
    && find /usr/local -type f -name '*.pyo' -delete \
    && find /usr/local -type d -name 'tests' -exec rm -rf {} + 2>/dev/null || true \
    && find /usr/local -type d -name 'test' -exec rm -rf {} + 2>/dev/null || true

# Copy training script and dataset config
COPY train.py /app/
COPY dataset.yaml /app/

ENV PYTHONUNBUFFERED=1

# Default config (override with env vars)
ENV TRAINING_DIR=/data/training
ENV MODEL_DIR=/data/models
ENV EPOCHS=20
ENV BATCH_SIZE=4
ENV DEVICE=cpu

CMD ["python", "train.py"]