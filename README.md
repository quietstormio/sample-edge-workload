# YOLO Edge ML Application - Helm Chart

A proof of concept for deploying YOLO training and inference on k3s with gateway-aware scheduling.

## Overview

This Helm chart deploys:
- **Training CronJob**: Runs every 3 hours on nodes with gateway connectivity
- **Inference Deployment**: Continuously runs inference and monitors for new models
- **Monitor DaemonSet**: Checks gateway connectivity and labels nodes accordingly
- **Shared Storage**: PV/PVC for sharing models and images between components

## Architecture

```
┌─────────────────┐
│ Monitor         │ ← Checks gateway connectivity
│ DaemonSet       │   Labels nodes: online/offline
└─────────────────┘
        │
        ├─ Labels nodes with: myapp.com/network-status=online|offline
        │
        ▼
┌─────────────────┐
│ Training        │ ← Only runs on nodes labeled "online"
│ CronJob         │   Runs every 3 hours
│ (every 3h)      │   Saves models to /data/models
└─────────────────┘
        │
        │ Writes models to shared volume
        ▼
┌─────────────────┐
│ Shared PV/PVC   │ ← 3Gi storage at /mnt/data/yolo
│ /data/models    │   Contains:
│ /data/images    │   - Models (latest.pt, versioned models)
└─────────────────┘   - Training images
        │
        │ Reads models from shared volume
        ▼
┌─────────────────┐
│ Inference       │ ← Continuously monitors for new models
│ Deployment      │   Runs inference on images in /data/images
└─────────────────┘
```
