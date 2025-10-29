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

## Prerequisites

- k3s cluster running
- kubectl configured to access your cluster
- Helm 3.x installed
- Docker or containerd for building images

## Configuration

Before deploying, update `values.yaml`:

```yaml
monitor:
  heartbeatIp: "192.168.1.1"  # Change to your gateway IP
```

## Deployment Steps

### 1. Build Docker Images

Build the training and inference images:

```bash
# Build training image
cd yolo-sample/train
docker build -t edge-trainer:latest .

# Build inference image
cd ../infer
docker build -t edge-inference:latest .
```

**For k3s**: Import the images into k3s:

```bash
docker save edge-trainer:latest | sudo k3s ctr images import -
docker save edge-inference:latest | sudo k3s ctr images import -
```

### 2. Prepare Storage

Ensure the host path exists on all nodes:

```bash
sudo mkdir -p /mnt/data/yolo/models
sudo mkdir -p /mnt/data/yolo/images
sudo chmod -R 777 /mnt/data/yolo
```

### 3. Add Sample Images (Optional)

Add some images for inference to process:

```bash
# Download sample images to the shared volume
sudo cp /path/to/your/images/* /mnt/data/yolo/images/
```

### 4. Deploy the Helm Chart

```bash
# From the chart root directory
helm install yolo-app . --namespace default
```

Or specify a custom namespace:

```bash
kubectl create namespace yolo
helm install yolo-app . --namespace yolo
```

### 5. Verify Deployment

Check that all components are running:

```bash
# Check monitor daemonset
kubectl get daemonset edge-ml-monitor-daemonset

# Check node labels
kubectl get nodes --show-labels | grep network-status

# Check training cronjob
kubectl get cronjob edge-training-job

# Check inference deployment
kubectl get deployment edge-inference

# Check PV/PVC
kubectl get pv,pvc
```

### 6. Monitor Logs

**Monitor DaemonSet** (check gateway connectivity):
```bash
kubectl logs -l component=monitor -f
```

**Training Jobs** (when they run):
```bash
kubectl logs -l app=edge-training-job -f
```

**Inference Deployment**:
```bash
kubectl logs -l app=edge-inference -f
```

## How It Works

### Gateway Connectivity Monitoring

The monitor daemonset runs on each node and:
1. Pings the configured gateway IP (`heartbeatIp`) every 15 seconds
2. Labels the node with `myapp.com/network-status=online` or `offline`

### Training Workflow

The training cronjob:
1. **Only runs on nodes labeled `myapp.com/network-status=online`**
2. Runs every 3 hours (schedule: `0 */3 * * *`)
3. Trains YOLO model on images in `/data/images`
4. Saves models to `/data/models/` as:
   - `latest.pt` - Always the newest model
   - `model_YYYYMMDD_HHMMSS.pt` - Versioned models
   - `latest_metadata.json` - Model metadata

### Inference Workflow

The inference deployment:
1. Continuously runs in a loop
2. Checks for new models every 30 seconds
3. Automatically reloads when a new model is detected
4. Runs inference on all images in `/data/images/`
5. Prints detection results to stdout (logs)

## Accessing Inference Externally

### Current State

The inference deployment currently:
- Processes images from the shared volume (`/data/images`)
- Outputs results to stdout (visible in logs)
- **Does not expose an HTTP endpoint**

### Making Inference Available via HTTP

To enable external access to inference, you need to add an HTTP server to `yolo-sample/infer/infer.py`.

The Service and Ingress are already configured and ready to use once you add an HTTP endpoint.

**Example: Add Flask HTTP endpoint**

1. Update `yolo-sample/infer/Dockerfile`:
```dockerfile
RUN pip install --no-cache-dir \
    ultralytics \
    opencv-python-headless \
    flask
```

2. Modify `yolo-sample/infer/infer.py` to add Flask endpoint:
```python
from flask import Flask, request, jsonify
import base64
import numpy as np
import cv2

app = Flask(__name__)
watcher = None

@app.route('/infer', methods=['POST'])
def infer():
    # Receive base64 encoded image
    image_data = request.json['image']
    img_bytes = base64.b64decode(image_data)
    nparr = np.frombuffer(img_bytes, np.uint8)
    img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)

    # Run inference
    results = watcher.model(img, verbose=False)

    # Extract detections
    detections = []
    for r in results:
        for box in r.boxes:
            detections.append({
                'class': int(box.cls[0].item()),
                'confidence': float(box.conf[0].item())
            })

    return jsonify({'detections': detections})

if __name__ == "__main__":
    # Initialize watcher globally
    watcher = ModelWatcher(MODEL_DIR)
    watcher.load_model()

    # Start Flask server
    app.run(host='0.0.0.0', port=8080)
```

3. Update `templates/infer-deployment.yaml` to add container port:
```yaml
containers:
- name: inference
  image: edge-inference:latest
  ports:
  - containerPort: 8080
    protocol: TCP
```

4. Rebuild and redeploy:
```bash
cd yolo-sample/infer
docker build -t edge-inference:latest .
docker save edge-inference:latest | sudo k3s ctr images import -
kubectl rollout restart deployment edge-inference
```

5. Access externally:
```bash
# Get the ingress IP
kubectl get ingress edge-inference-ingress

# Test inference
curl -X POST http://<INGRESS_IP>/infer \
  -H "Content-Type: application/json" \
  -d '{"image": "<base64-encoded-image>"}'
```

## Configuration Reference

### values.yaml

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `monitor.image.repository` | Monitor container image | `bitnami/kubectl` |
| `monitor.image.tag` | Monitor image tag | `latest` |
| `monitor.checkIntervalSeconds` | Gateway ping interval | `15` |
| `monitor.heartbeatIp` | Gateway IP to ping | `192.168.1.1` |
| `monitor.nodeLabelKey` | Node label key for status | `myapp.com/network-status` |

### Environment Variables

**Training:**
- `IMAGES_DIR`: `/data/images`
- `MODEL_DIR`: `/data/models`
- `EPOCHS`: `20`
- `BATCH_SIZE`: `4`
- `DEVICE`: `cpu`

**Inference:**
- `IMAGES_DIR`: `/data/images`
- `MODEL_DIR`: `/data/models`
- `INFERENCE_INTERVAL`: `2` (seconds between inferences)
- `CHECK_MODEL_INTERVAL`: `30` (seconds between model checks)

## Troubleshooting

### Training jobs not running

1. Check node labels:
```bash
kubectl get nodes --show-labels | grep network-status
```

2. If no nodes are labeled "online", check monitor logs:
```bash
kubectl logs -l component=monitor
```

3. Verify gateway IP is correct in `values.yaml`

### PVC not binding

1. Check PV/PVC status:
```bash
kubectl get pv,pvc
```

2. Verify host path exists:
```bash
sudo ls -la /mnt/data/yolo
```

### Inference not detecting new models

1. Check that training has completed:
```bash
sudo ls -la /mnt/data/yolo/models/
```

2. Check inference logs:
```bash
kubectl logs -l app=edge-inference -f
```

### Images not building

Make sure you're in the correct directory and have the required files:
```bash
# Training
ls yolo-sample/train/
# Should show: Dockerfile, train.py

# Inference
ls yolo-sample/infer/
# Should show: Dockerfile, infer.py
```

## Cleanup

Remove the deployment:

```bash
helm uninstall yolo-app --namespace default
```

Clean up storage (optional):

```bash
sudo rm -rf /mnt/data/yolo
```

## Next Steps

1. Add HTTP endpoint to inference service (see "Making Inference Available via HTTP")
2. Add authentication to the ingress
3. Configure persistent storage (NFS, Ceph, etc.) instead of hostPath
4. Add monitoring/metrics (Prometheus)
5. Configure resource limits based on your hardware
