# ⚠️ PROOF OF CONCEPT TESTING MODE ⚠️

## TEMPORARY CHANGES FOR POC

This document tracks **temporary modifications** made for proof-of-concept testing that **MUST BE REMOVED** before production deployment.

---

## 🚨 What's Different from Production Architecture

### TEMPORARY: Edge Model Auto-Copy to Production

**File**: `yolo-sample/train/train.py:107-126`

**What it does**: After training completes, the script automatically copies the newly trained edge model (`edge_trained/edge_*.pt`) to `production.pt`.

**Why this is WRONG for production**:
- In federated learning, edge devices should **NEVER** overwrite `production.pt` directly
- `production.pt` should **ONLY** be updated by the gateway after FedAvg aggregation
- This breaks the federated learning model where the gateway creates the global model

**Why we're doing it for PoC**:
- Allows immediate testing of training improvements
- Lets us see confidence increase without implementing full gateway aggregation
- Proves the training loop actually works

**Code block (train.py:107-126)**:
```python
# ========================================================================
# PROOF OF CONCEPT: Copy edge model to production.pt
# ========================================================================
# THIS IS TEMPORARY FOR POC TESTING ONLY!
#
# In production federated learning:
# - Edge devices should NOT overwrite production.pt
# - production.pt should only be updated by gateway after FedAvg aggregation
# - This logic allows immediate testing of training improvements
#
# TODO: REMOVE THIS BLOCK WHEN IMPLEMENTING REAL FEDERATED LEARNING
# ========================================================================
production_model_path = f'{MODEL_DIR}/production.pt'
shutil.copy(edge_model_path, production_model_path)
print("\n" + "!"*60)
print("⚠️  PoC MODE: Copied edge model to production.pt")
print("⚠️  This is TEMPORARY for testing purposes only!")
print("⚠️  In production, gateway should distribute production.pt")
print("!"*60 + "\n")
# ========================================================================
```

---

## 📋 PoC Testing Procedure

### Goal
Prove that training improves model accuracy on hotdog detection.

### Prerequisites
1. Hotdog dataset downloaded and uploaded to `/mnt/data/yolo/training/`
2. Updated Docker images deployed with PoC logic
3. Baseline hotdog image for testing

### Test Steps

#### 1. Baseline Test (Before Training)
```bash
# Access the web UI
open http://13.217.37.126:6767

# Upload a hotdog image
# Note the confidence score (e.g., 35% or 0.35)
# Example result:
{
  "class_name": "hot dog",
  "confidence": 0.35
}
```

#### 2. Trigger Training
```bash
# SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@13.217.37.126

# Manually trigger training job
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-cronjob manual-training-test -n default

# Watch training progress
sudo /usr/local/bin/k3s kubectl get pods -n default | grep manual-training
sudo /usr/local/bin/k3s kubectl logs -f <manual-training-pod-name> -n default

# Look for the PoC message:
# ⚠️  PoC MODE: Copied edge model to production.pt
# ⚠️  This is TEMPORARY for testing purposes only!
```

#### 3. Verify Model Update
```bash
# Check that production.pt was updated
ls -lah /mnt/data/yolo/models/production.pt

# Should show recent timestamp matching training completion
```

#### 4. Re-test (After Training)
```bash
# Upload the SAME hotdog image again
# Check the new confidence score
# Expected: Significant increase (e.g., 75%+ or 0.75+)

# Example result:
{
  "class_name": "hot dog",
  "confidence": 0.78
}
```

#### 5. Success Criteria
- ✅ Confidence increased by 20%+ points
- ✅ Training completed without errors
- ✅ Edge model was saved to `edge_trained/`
- ✅ Production model was updated (PoC mode)
- ✅ Inference pod automatically picked up new model

---

## 🔄 Correct Production Architecture (Future)

```
EDGE DEVICE
├── Training:
│   └── Saves to: edge_trained/edge_20250130_*.pt
│                 (Never touches production.pt)
│
└── Inference:
    └── Loads from: production.pt (from gateway)

                    ↓ Upload edge model

GATEWAY (18.208.162.218)
├── Collects edge models from all devices
├── Runs FedAvg aggregation
└── Creates production.pt (global model)

                    ↓ Distribute

EDGE DEVICES (all)
└── Download production.pt
    └── Inference automatically uses updated global model
```

---

## 🗑️ How to Restore Production Behavior

When ready to implement real federated learning:

### 1. Remove PoC Block from train.py

**Delete lines 107-126** in `yolo-sample/train/train.py`:
```bash
# Remove the entire PoC block including:
# - The banner comments
# - The shutil.copy() call
# - The warning print statements
```

### 2. Implement Gateway Aggregation

Create gateway service that:
- Receives edge models from devices
- Implements FedAvg algorithm
- Distributes production.pt back to edges

### 3. Update Edge Devices

Deploy updated images without PoC logic:
```bash
# Rebuild images
cd yolo-sample/train
podman build -t edge-training:latest .

# Import to k3s using agent directory
sudo podman save localhost/edge-training:latest -o /var/lib/rancher/k3s/agent/images/edge-training.tar
sudo systemctl restart k3s

# Tag for deployment
sudo /usr/local/bin/k3s ctr images tag localhost/edge-training:latest docker.io/library/edge-training:latest

# Restart deployment
sudo /usr/local/bin/helm upgrade edge-ml ~/edge-ml-chart -n default --kubeconfig /etc/rancher/k3s/k3s.yaml
```

---

## 📝 Checklist Before Production

- [ ] Remove PoC block from train.py (lines 107-126)
- [ ] Implement gateway aggregation service
- [ ] Implement S3/storage for model distribution
- [ ] Test full federated learning loop
- [ ] Verify edge devices don't overwrite production.pt
- [ ] Update BUILD.md and ARCHITECTURE.md
- [ ] Remove this POC_TESTING.md file

---

## 📊 Expected Results

### During PoC Testing

| Metric | Before Training | After Training | Improvement |
|--------|----------------|----------------|-------------|
| Confidence on test image | 30-40% | 70-85% | +40-50% |
| Detection accuracy | Low | High | Significant |
| False positives | Possible | Reduced | Better |

### Limitations of PoC

- Only tests single device (no multi-device aggregation)
- No gateway FedAvg implementation
- Edge model directly becomes production model
- Doesn't test distributed learning benefits

---

## 🎯 Current Status

- ✅ PoC logic added to train.py
- ✅ dataset.yaml configured for hotdog detection
- ✅ Clear warnings in code and logs
- ✅ Documentation created (this file)
- ⏳ Awaiting PoC test execution
- ❌ Gateway aggregation not implemented (future)

**Last Updated**: 2025-01-30
**Remove By**: Before production deployment with real Jetson devices
