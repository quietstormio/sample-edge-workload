# 🌸 Federated Learning with FedAvg

This guide explains how to use the manual FedAvg (Federated Averaging) implementation for federated learning across edge devices.

## 📚 What is Federated Learning?

**Federated Learning** allows multiple edge devices to collaboratively train a machine learning model without sharing raw data. Each device:
1. Trains on its local data
2. Sends only model parameters (not data) to a central server
3. Server aggregates parameters using FedAvg algorithm
4. Distributes updated global model back to devices

**Benefits**:
- ✅ Privacy: Raw data never leaves edge devices
- ✅ Bandwidth: Only model parameters transferred, not datasets
- ✅ Specialization: Each edge learns from its unique environment
- ✅ Robustness: Combined model learns from diverse data sources

## 🔄 How FedAvg Works

The **Federated Averaging** algorithm (McMahan et al., 2016):

```
For each training round:
  1. Server sends current global model to edge devices
  2. Each edge device k:
     - Trains model on local data (n_k samples)
     - Sends updated parameters back to server
  3. Server aggregates using weighted average:

     w_global = Σ(n_k / n_total) * w_k

     where w_k are parameters from device k

  4. Server distributes new global model
```

**Key insight**: Devices with more training data contribute more to the global model (weighted by `n_k / n_total`).

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     EDGE DEVICE 1                           │
│  ┌────────────────────────────────────────────────────┐    │
│  │ Training Job (CronJob)                             │    │
│  │ - Trains on local hotdog images                    │    │
│  │ - Saves to: edge_trained/edge_20250130_120000.pt  │    │
│  │ - Saves metadata: num_images, node_id, etc.       │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ (Model + metadata saved to /mnt/data/yolo/models/edge_trained/)
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    AGGREGATION SERVER                        │
│  (Run manually on instance for PoC)                         │
│                                                              │
│  $ python fedavg_aggregate.py                               │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │ FedAvg Algorithm:                                  │    │
│  │ 1. Load all edge models from edge_trained/        │    │
│  │ 2. Read metadata (num_images for weighting)       │    │
│  │ 3. Weighted average: Σ(n_k/n_total) * w_k        │    │
│  │ 4. Save to: aggregated_global.pt                  │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ (Manual copy to production.pt)
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     INFERENCE SERVICE                        │
│  ┌────────────────────────────────────────────────────┐    │
│  │ Uses: production.pt (global aggregated model)      │    │
│  │ Serves predictions on http://IP:6767               │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## 🚀 Quick Start Guide

### Prerequisites

1. ✅ Training Docker image rebuilt with updated `train.py`
2. ✅ Hotdog dataset uploaded to `/mnt/data/yolo/training/`
3. ✅ Base model `yolov8n.pt` at `/mnt/data/yolo/models/`

### Step 1: Simulate Multiple Edge Devices

For this PoC, we'll simulate multiple edge devices by running training multiple times with different data subsets.

**On your instance**:

```bash
ssh -i ~/Downloads/bigKey.pem ec2-user@54.162.60.44

# Option A: Use different data subsets (realistic simulation)
# Split your dataset into 2-3 subsets representing different "edge devices"

# Create device directories
sudo mkdir -p /mnt/data/yolo/training_device1
sudo mkdir -p /mnt/data/yolo/training_device2

# Split dataset (example: 100 images each)
sudo cp /mnt/data/yolo/training/images/{1..100}.jpg /mnt/data/yolo/training_device1/images/
sudo cp /mnt/data/yolo/training/labels/{1..100}.txt /mnt/data/yolo/training_device1/labels/

sudo cp /mnt/data/yolo/training/images/{101..200}.jpg /mnt/data/yolo/training_device2/images/
sudo cp /mnt/data/yolo/training/labels/{101..200}.txt /mnt/data/yolo/training_device2/labels/
```

**Option B**: Just run training twice on the same data (simpler, less realistic):

```bash
# Run training job 1
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-job edge-device1 -n default

# Wait for completion (check logs)
POD1=$(sudo /usr/local/bin/k3s kubectl get pods -n default | grep edge-device1 | awk '{print $1}')
sudo /usr/local/bin/k3s kubectl logs -f $POD1 -n default

# Wait a few minutes, then run training job 2
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-job edge-device2 -n default

POD2=$(sudo /usr/local/bin/k3s kubectl get pods -n default | grep edge-device2 | awk '{print $1}')
sudo /usr/local/bin/k3s kubectl logs -f $POD2 -n default
```

### Step 2: Verify Edge Models Saved

```bash
# Check edge_trained directory
ls -lah /mnt/data/yolo/models/edge_trained/

# Should see something like:
# edge_20250130_120000.pt
# edge_20250130_120530.pt

# Check metadata
ls -lah /mnt/data/yolo/models/edge_trained/metadata/

# Should see:
# edge_20250130_120000.json
# edge_20250130_120530.json

# View metadata
cat /mnt/data/yolo/models/edge_trained/metadata/edge_*.json
```

### Step 3: Run FedAvg Aggregation

The aggregation script is already in the training container. Run it:

```bash
# Option A: Run inside training container
sudo /usr/local/bin/k3s kubectl run fedavg-aggregation \
  --image=localhost/edge-training:latest \
  --restart=Never \
  --rm \
  -it \
  -n default \
  --overrides='
{
  "spec": {
    "containers": [{
      "name": "fedavg",
      "image": "localhost/edge-training:latest",
      "command": ["python", "/app/fedavg_aggregate.py"],
      "volumeMounts": [{
        "name": "shared-data",
        "mountPath": "/data"
      }]
    }],
    "volumes": [{
      "name": "shared-data",
      "persistentVolumeClaim": {
        "claimName": "edge-ml-shared-pvc"
      }
    }]
  }
}' \
-- python /app/fedavg_aggregate.py

# Option B: Copy script to instance and run with Python
# (Requires PyTorch installed on instance)
```

**Expected Output**:

```
============================================================
FedAvg Federated Learning Aggregation
============================================================
Models directory: /data/models/edge_trained
Output path: /data/models/aggregated_global.pt

Found 2 edge models
============================================================
Edge Model 1:
  Node: ip-172-31-47-219
  Version: 20250130_120000
  Training samples: 100
  Epochs: 20
  Path: /data/models/edge_trained/edge_20250130_120000.pt

Edge Model 2:
  Node: ip-172-31-47-219
  Version: 20250130_120530
  Training samples: 100
  Epochs: 20
  Path: /data/models/edge_trained/edge_20250130_120530.pt

============================================================
Starting FedAvg Aggregation
Total training samples across all edges: 200
============================================================

Processing ip-172-31-47-219:
  Samples: 100
  Weight: 0.5000 (50.0%)
  ✓ Aggregated

Processing ip-172-31-47-219:
  Samples: 100
  Weight: 0.5000 (50.0%)
  ✓ Aggregated

============================================================
✓ Federated Aggregation Complete!
  Algorithm: FedAvg (Federated Averaging)
  Edge models combined: 2
  Total training samples: 200
  Output model: /data/models/aggregated_global.pt
  Metadata: /data/models/aggregated_global_metadata.json
============================================================

📝 Next steps:
1. Copy aggregated model to production:
   cp /data/models/aggregated_global.pt /data/models/production.pt
2. Inference deployment will automatically use new model
3. Test with hotdog image to verify improvement
```

### Step 4: Deploy Aggregated Model

```bash
# Copy aggregated model to production
sudo cp /mnt/data/yolo/models/aggregated_global.pt /mnt/data/yolo/models/production.pt

# Check timestamp updated
ls -lah /mnt/data/yolo/models/production.pt

# Restart inference deployment to pick up new model (optional, auto-reloads)
sudo /usr/local/bin/k3s kubectl rollout restart deployment edge-inference -n default
```

### Step 5: Test Improvement

```bash
# Access web UI
echo "http://54.162.60.44:6767"

# Upload the same hotdog image you tested before training
# Compare confidence scores
```

**Expected Results**:

| Stage | Confidence | Model |
|-------|-----------|-------|
| Before any training | 83% | Base yolov8n.pt (COCO 80-class) |
| After single device training | 68% | Single edge model (1-class, undertrained) |
| **After FedAvg aggregation** | **75-85%** | **Global model (federated, better)** |

---

## 📊 Understanding the Results

### Why FedAvg Works Better

**Single Device Training**:
- Limited data (e.g., 100 images)
- May overfit to local distribution
- Less robust

**Federated Learning (FedAvg)**:
- Combined data knowledge (e.g., 200+ images across devices)
- Averaged parameters reduce overfitting
- More generalized model

### Weights in Action

If Device 1 has 150 images and Device 2 has 50 images:

```
Device 1 weight: 150 / (150+50) = 0.75 (75%)
Device 2 weight:  50 / (150+50) = 0.25 (25%)

Global parameters = 0.75 * Device1_params + 0.25 * Device2_params
```

Device 1 contributes more because it trained on more data!

---

## 🔬 Advanced: Multi-Round Federated Learning

For production, you'd iterate:

```
Round 1:
  - Devices start with yolov8n.pt
  - Train locally
  - Aggregate → production_round1.pt

Round 2:
  - Devices download production_round1.pt
  - Train more on new data
  - Aggregate → production_round2.pt

Round 3:
  - ... continue improving
```

**To simulate**:

```bash
# Round 1
# (Do training + aggregation as above)

# Round 2: Use Round 1's output as base model
sudo cp /mnt/data/yolo/models/aggregated_global.pt /mnt/data/yolo/models/production.pt

# Clear old edge models
sudo rm -rf /mnt/data/yolo/models/edge_trained/*

# Run training again (will use production.pt as base)
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-job edge-round2-device1 -n default

# ... aggregate again
```

---

## 🛠️ Troubleshooting

### No Edge Models Found

```bash
# Check directory exists
ls -la /mnt/data/yolo/models/edge_trained/

# Check metadata directory
ls -la /mnt/data/yolo/models/edge_trained/metadata/

# If empty, training jobs didn't complete
# Check pod logs
sudo /usr/local/bin/k3s kubectl logs -l app=edge-training-job -n default
```

### Aggregation Fails

```bash
# Check PyTorch can load models
python3 << 'EOF'
import torch
model = torch.load('/mnt/data/yolo/models/edge_trained/edge_*.pt')
print(type(model))
EOF

# If error, models may be corrupted
# Re-run training
```

### Confidence Doesn't Improve

**Possible causes**:
1. **Too few edge devices**: Need 2+ for aggregation benefits
2. **Identical data**: If all devices train on same data, no diversity benefit
3. **Wrong base model**: Make sure starting from yolov8n.pt or previous aggregated model
4. **Class mismatch**: Using 1 class vs 80 classes (see earlier discussion)

**Solutions**:
- Train more rounds
- Use different data subsets per device
- Increase epochs per device
- Consider switching to 80-class COCO dataset

---

## 📝 Files Reference

| File | Purpose |
|------|---------|
| `train.py` | Edge device training (saves to edge_trained/) |
| `fedavg_aggregate.py` | FedAvg aggregation script |
| `edge_trained/edge_*.pt` | Edge-trained models |
| `edge_trained/metadata/edge_*.json` | Training metadata (num_images, etc.) |
| `aggregated_global.pt` | Output of FedAvg aggregation |
| `aggregated_global_metadata.json` | Aggregation metadata |
| `production.pt` | Global model used by inference |

---

## 🎓 Next Steps

1. **Automate aggregation**: Create a gateway service that runs FedAvg automatically
2. **Multi-device deployment**: Deploy to actual Jetson devices
3. **Secure communication**: Add encryption for model parameter transfer
4. **Differential privacy**: Add noise to parameters before sending
5. **Asynchronous updates**: Handle devices joining/leaving dynamically

---

## 📚 References

- **FedAvg Paper**: McMahan et al., "Communication-Efficient Learning of Deep Networks from Decentralized Data" (2016) - https://arxiv.org/abs/1602.05629
- **Flower Framework**: https://flower.ai/
- **Federated Learning Book**: https://www.federated-learning.org/

---

## ✅ Quick Command Reference

```bash
# Run training (simulate 2 devices)
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-job edge-device1 -n default
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-job edge-device2 -n default

# Check edge models
ls -lah /mnt/data/yolo/models/edge_trained/
cat /mnt/data/yolo/models/edge_trained/metadata/*.json

# Run aggregation (inside container)
sudo /usr/local/bin/k3s kubectl run fedavg-temp --image=localhost/edge-training:latest --restart=Never --rm -it -n default --overrides='{"spec":{"containers":[{"name":"fedavg","image":"localhost/edge-training:latest","command":["python","/app/fedavg_aggregate.py"],"volumeMounts":[{"name":"data","mountPath":"/data"}]}],"volumes":[{"name":"data","persistentVolumeClaim":{"claimName":"edge-ml-shared-pvc"}}]}}'

# Deploy aggregated model
sudo cp /mnt/data/yolo/models/aggregated_global.pt /mnt/data/yolo/models/production.pt

# Test
echo "http://54.162.60.44:6767"
```
