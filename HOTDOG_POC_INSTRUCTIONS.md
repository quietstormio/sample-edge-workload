# 🌭 Hotdog Detection PoC Testing Instructions

## ✅ What's Been Done

- ✅ Updated `dataset.yaml` for hotdog-only detection
- ✅ Added PoC logic to `train.py` to copy edge model to production.pt (TEMPORARY)
- ✅ Rebuilt training Docker image with PoC changes
- ✅ Created documentation (POC_TESTING.md)
- ⏳ **Next: Download dataset and run test**

---

## 📥 Step 1: Download Roboflow Hotdog Dataset

### Option A: Using Roboflow CLI (Easiest)

```bash
# SSH to the instance
ssh -i ~/Downloads/bigKey.pem ec2-user@3.91.245.253

# Install roboflow CLI
pip3 install roboflow

# Download the dataset
python3 << 'EOF'
from roboflow import Roboflow
rf = Roboflow(api_key="YOUR_API_KEY_HERE")  # Get from roboflow.com account
project = rf.workspace("workspace-2eqzv").project("hot-dog-detection")
dataset = project.version(2).download("yolov8")
EOF

# Move to correct location
sudo mv hot-dog-detection-2/train/images/* /mnt/data/yolo/training/images/
sudo mv hot-dog-detection-2/train/labels/* /mnt/data/yolo/training/labels/
```

### Option B: Manual Download (No Account Needed)

1. Go to: https://universe.roboflow.com/workspace-2eqzv/hot-dog-detection/dataset/2
2. Click "Download this Dataset"
3. Select format: **YOLOv8**
4. Download the ZIP file
5. Extract and upload to instance:

```bash
# On your local machine
unzip hot-dog-detection.zip
cd hot-dog-detection-2/train

# Upload to instance
scp -i ~/Downloads/bigKey.pem -r images labels ec2-user@3.91.245.253:~/

# SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@3.91.245.253

# Move to correct location
sudo mkdir -p /mnt/data/yolo/training/images
sudo mkdir -p /mnt/data/yolo/training/labels
sudo mv ~/images/* /mnt/data/yolo/training/images/
sudo mv ~/labels/* /mnt/data/yolo/training/labels/
```

### Option C: Minimal Test (Just a Few Images)

If you want to test quickly with just 10-15 images:

```bash
# Download a subset
# Follow Option B but only copy 10-15 images + labels to save time
```

---

## 🧪 Step 2: Verify Dataset is Ready

```bash
ssh -i ~/Downloads/bigKey.pem ec2-user@3.91.245.253

# Check images
ls -lh /mnt/data/yolo/training/images/ | head -20
# Should see: hotdog_001.jpg, hotdog_002.jpg, etc.

# Check labels
ls -lh /mnt/data/yolo/training/labels/ | head -20
# Should see: hotdog_001.txt, hotdog_002.txt, etc.

# Count images
echo "Number of images: $(ls /mnt/data/yolo/training/images/ | wc -l)"
echo "Number of labels: $(ls /mnt/data/yolo/training/labels/ | wc -l)"
# Numbers should match!
```

---

## 🎯 Step 3: Baseline Test (Before Training)

1. **Get a test hotdog image** (save one from the dataset or find online)

2. **Access the web UI**: http://3.91.245.253:6767

3. **Upload the hotdog image**

4. **Note the confidence score**:
   ```json
   {
     "class_name": "hot dog",
     "confidence": 0.35  ← Write this down!
   }
   ```

---

## 🏋️ Step 4: Run Training

```bash
# SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@54.162.60.44

# Manually trigger training job
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-cronjob hotdog-poc-training -n default

# Watch the training job start
sudo /usr/local/bin/k3s kubectl get pods -n default | grep hotdog-poc

# Follow the logs
POD_NAME=$(sudo /usr/local/bin/k3s kubectl get pods -n default | grep hotdog-poc | awk '{print $1}')
sudo /usr/local/bin/k3s kubectl logs -f $POD_NAME -n default
```

### What to Look For in Logs:

```
============================================================
Starting training at 2025-01-30...
Training directory: /data/training
Found 150 images
============================================================
Using dataset config: /app/dataset.yaml

[Training progress...]

============================================================
✓ Training complete!
  Node: <node-name>
  Version: 20250130_123456
  Training time: 324.52s
  Edge model saved to: /data/models/edge_trained/edge_20250130_123456.pt

!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
⚠️  PoC MODE: Copied edge model to production.pt
⚠️  This is TEMPORARY for testing purposes only!
⚠️  In production, gateway should distribute production.pt
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

  PoC: Also copied to production.pt (TEMPORARY)
  (Ready for gateway sync)
============================================================
```

**Training Time Estimate**:
- 15-20 images: ~5-10 minutes on CPU
- 50-100 images: ~15-30 minutes on CPU
- Full dataset: ~45-60 minutes on CPU

---

## ✨ Step 5: Re-test (After Training)

1. **Wait for training job to complete**:
   ```bash
   sudo /usr/local/bin/k3s kubectl get pods -n default | grep hotdog-poc
   # Should show: Completed
   ```

2. **Verify production.pt was updated**:
   ```bash
   ls -lah /mnt/data/yolo/models/production.pt
   # Timestamp should match training completion time
   ```

3. **Upload the SAME hotdog image again** to http://3.91.245.253:6767

4. **Check the new confidence**:
   ```json
   {
     "class_name": "hot dog",
     "confidence": 0.82  ← Should be MUCH higher!
   }
   ```

---

## 📊 Expected Results

| Metric | Before Training | After Training | Success Criteria |
|--------|----------------|----------------|------------------|
| **Confidence** | 30-40% | 70-90% | ✅ +40%+ increase |
| **Detection** | May miss some | Detects reliably | ✅ Consistent detection |
| **False Positives** | Possible | Reduced | ✅ Better accuracy |

### Example Results:

**Before Training**:
```json
{
  "image": "hotdog_test.jpg",
  "detections": [
    {
      "class_name": "hot dog",
      "confidence": 0.37,
      "bbox": {...}
    }
  ],
  "count": 1
}
```

**After Training**:
```json
{
  "image": "hotdog_test.jpg",
  "detections": [
    {
      "class_name": "hot dog",
      "confidence": 0.84,  ← Improved!
      "bbox": {...}
    }
  ],
  "count": 1
}
```

---

## 🐛 Troubleshooting

### Problem: Training job fails

```bash
# Check logs for errors
sudo /usr/local/bin/k3s kubectl logs $POD_NAME -n default

# Common issues:
# - No images found → Check /mnt/data/yolo/training/images/ exists and has files
# - No labels found → Check /mnt/data/yolo/training/labels/ exists and has files
# - Mismatched counts → Each image needs a corresponding .txt label file
```

### Problem: Confidence doesn't improve

- **Try more images**: 15-20 is minimum, 50+ is better
- **Check label quality**: Labels must be correct YOLO format
- **Train longer**: Increase EPOCHS in train-cronjob.yaml (default: 20)
- **Check base model**: Ensure /mnt/data/yolo/models/yolov8n.pt exists

### Problem: Production.pt not updated

```bash
# Check if PoC logic is in the image
sudo /usr/local/bin/k3s kubectl logs $POD_NAME -n default | grep "PoC MODE"
# Should see the warning message

# If not, rebuild training image:
cd ~/yolo-sample/train
sudo podman build --no-cache -t localhost/edge-training:latest .

# Import to k3s
sudo podman save localhost/edge-training:latest -o /var/lib/rancher/k3s/agent/images/edge-training.tar
sudo systemctl restart k3s

# Tag for deployment
sudo /usr/local/bin/k3s ctr images tag localhost/edge-training:latest docker.io/library/edge-training:latest
```

---

## 🎓 What This Proves

✅ **Training works**: Model improves with local data
✅ **Inference works**: Updated model is automatically picked up
✅ **Architecture works**: Training and inference are properly separated
✅ **Ready for federated learning**: Just need to remove PoC logic and add gateway

---

## 🚧 Remember: This is PoC Mode!

The current setup has edge devices directly overwriting `production.pt`. This is **NOT** how federated learning should work in production.

**Production architecture** (future):
- Edge devices save to `edge_trained/` only
- Gateway collects models from all edges
- Gateway runs FedAvg aggregation
- Gateway distributes `production.pt` back to edges

**To restore production behavior**: See `POC_TESTING.md` for removal instructions.

---

## 📝 Quick Command Reference

```bash
# SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@3.91.245.253

# Check dataset
ls -lh /mnt/data/yolo/training/images/ | wc -l

# Trigger training
sudo /usr/local/bin/k3s kubectl create job --from=cronjob/edge-training-cronjob hotdog-test -n default

# Watch training
POD=$(sudo /usr/local/bin/k3s kubectl get pods -n default | grep hotdog-test | awk '{print $1}')
sudo /usr/local/bin/k3s kubectl logs -f $POD -n default

# Check production model
ls -lah /mnt/data/yolo/models/production.pt

# Access UI
echo "http://3.91.245.253:6767"
```

---

## ✅ Success Checklist

- [ ] Dataset downloaded (15+ images minimum)
- [ ] Images in `/mnt/data/yolo/training/images/`
- [ ] Labels in `/mnt/data/yolo/training/labels/`
- [ ] Baseline confidence recorded
- [ ] Training job completed successfully
- [ ] PoC warning message seen in logs
- [ ] production.pt timestamp updated
- [ ] Re-test shows significant confidence increase
- [ ] Screenshot/document results for demo

**Expected Time**: 30-60 minutes total (including training)
