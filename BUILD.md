# Docker Build Guide

This project supports two different deployment targets with separate Dockerfiles.

## Deployment Targets

| Target | Architecture | GPU Support | Dockerfile | Use Case |
|--------|--------------|-------------|------------|----------|
| **AWS CPU** | x86_64 | CPU only | `Dockerfile` | m7i.xlarge, local development |
| **Jetson** | ARM64 | NVIDIA Ampere | `Dockerfile.jetson` | Jetson Orin Nano Super edge deployment |

## Build Commands

### 1. CPU-only (x86_64) - AWS m7i.xlarge

**For m7i.xlarge or local development:**

```bash
# Inference
cd yolo-sample/infer
podman build -t edge-inference:latest .

# Training
cd yolo-sample/train
podman build -t edge-training:latest .
```

**After building, import to k3s:**
```bash
# Save images to k3s agent directory (k3s will auto-import on restart)
sudo podman save localhost/edge-inference:latest -o /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo podman save localhost/edge-training:latest -o /var/lib/rancher/k3s/agent/images/edge-training.tar

# Restart k3s to import images
sudo systemctl restart k3s

# Verify images imported
sudo /usr/local/bin/k3s ctr images ls | grep edge

# Tag for k3s deployments
sudo /usr/local/bin/k3s ctr images tag localhost/edge-inference:latest docker.io/library/edge-inference:latest
sudo /usr/local/bin/k3s ctr images tag localhost/edge-training:latest docker.io/library/edge-training:latest
```

### 2. GPU (ARM64) - Jetson Orin Nano Super

**Build on the Jetson device itself:**

```bash
# Inference
cd yolo-sample/infer
podman build -f Dockerfile.jetson -t edge-inference:jetson .

# Training
cd yolo-sample/train
podman build -f Dockerfile.jetson -t edge-training:jetson .
```

**After building, import to k3s:**
```bash
# Save images to k3s agent directory (k3s will auto-import on restart)
sudo podman save localhost/edge-inference:jetson -o /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo podman save localhost/edge-training:jetson -o /var/lib/rancher/k3s/agent/images/edge-training.tar

# Restart k3s to import images
sudo systemctl restart k3s

# Verify images imported
sudo /usr/local/bin/k3s ctr images ls | grep edge

# Tag for k3s deployments
sudo /usr/local/bin/k3s ctr images tag localhost/edge-inference:jetson docker.io/library/edge-inference:latest
sudo /usr/local/bin/k3s ctr images tag localhost/edge-training:jetson docker.io/library/edge-training:latest
```

**Update deployment environment variable:**
```yaml
# In templates/infer-deployment.yaml and templates/train-cronjob.yaml
env:
- name: DEVICE
  value: "cuda"  # Change from "cpu"
```

## Key Differences Between Dockerfiles

### Dockerfile (CPU, x86_64)
- Base: `python:3.10-slim`, `golang:1.21-alpine`
- PyTorch: CPU-only from `https://download.pytorch.org/whl/cpu`
- kubectl: AMD64 binary
- **No GPU support**
- Use case: Cloud instances (m7i.xlarge), local dev

### Dockerfile.jetson (CUDA, ARM64)
- Base: `nvcr.io/nvidia/l4t-pytorch:r35.2.1-pth2.0-py3`, `golang:1.21-alpine`
- PyTorch: **Pre-installed with CUDA** in base image
- kubectl: ARM64 binary
- **NVIDIA Ampere GPU support (1024 CUDA cores, 67 TOPS)**
- Requires: JetPack 6.1+, NVIDIA container runtime
- Use case: Jetson Orin Nano Super edge devices

## AWS m7i.xlarge Specs

| Specification | Value |
|--------------|-------|
| **vCPUs** | 4 |
| **RAM** | 16 GB DDR5 |
| **Architecture** | x86_64 (Intel 4th Gen Xeon Scalable) |
| **Network** | Up to 12.5 Gbps |
| **Price** | ~$0.192/hour (~$140/month) |
| **GPU** | None |

## Jetson Orin Nano Super Specs

| Specification | Value |
|--------------|-------|
| **CPU** | 6-core ARM Cortex-A78AE @ 1.7GHz |
| **RAM** | 8GB LPDDR5 (102 GB/s bandwidth) |
| **GPU** | NVIDIA Ampere (1024 CUDA cores, 32 Tensor cores @ 1020MHz) |
| **AI Performance** | 67 TOPS (INT8) |
| **Power** | 25W TDP |
| **Price** | $249 one-time |
| **Break-even vs AWS** | < 2 months |

## Environment Variables

All Dockerfiles use the same environment variables for consistency:

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL_DIR` | `/data/models` | Model storage location |
| `TRAINING_DIR` | `/data/training` | Training data location (training only) |
| `DEVICE` | `cpu` or `cuda` | Inference/training device |
| `EPOCHS` | `20` | Training epochs (training only) |
| `BATCH_SIZE` | `4` | Training batch size (training only) |

**Important:** Set `DEVICE=cuda` in your Kubernetes deployment when using Jetson.

## Performance Comparison

| Platform | Inference Time (estimate) | Architecture | Power | Cost |
|----------|---------------------------|--------------|-------|------|
| **m7i.xlarge (CPU)** | 500-2000ms | x86_64 | ~10W | $0.192/hr ($140/mo) |
| **Jetson Orin Nano (GPU)** | 10-50ms | ARM64 | 25W | $249 one-time |

**Key Insight:** Jetson provides **10-50x faster inference** at a fraction of the power consumption and pays for itself in under 2 months compared to cloud.

## Kubernetes GPU Configuration (Jetson Only)

For Jetson deployment, ensure your deployment has GPU resources:

```yaml
resources:
  limits:
    nvidia.com/gpu: 1  # Request 1 GPU
```

And ensure NVIDIA container runtime is configured in k3s on the Jetson:
```bash
# On the Jetson device
sudo nvidia-ctk runtime configure --runtime=containerd --config=/var/lib/rancher/k3s/agent/etc/containerd/config.toml
sudo systemctl restart k3s
```

## Deployment Strategy

1. **Development/Testing**: Use m7i.xlarge with CPU Dockerfile for development and testing
2. **Production Edge**: Deploy to Jetson Orin Nano with Jetson Dockerfile for production edge inference with GPU acceleration
3. **Gateway/Aggregation**: Use cloud instances (m7i or larger) for model aggregation (FedAvg) at the gateway
