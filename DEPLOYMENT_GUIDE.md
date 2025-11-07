# Edge ML Application Deployment Guide

## Prerequisites
- EC2 instance with k3s installed
- Podman installed
- SSH access with keypair at `~/Downloads/bigKey.pem`
- Helm installed on the instance

---

## Step 1: Create Required Directories on Target Machine

```bash
# SSH to the instance
ssh -i ~/Downloads/bigKey.pem ec2-user@<INSTANCE_IP>

# Create data directories
sudo mkdir -p /mnt/data/yolo/models
sudo mkdir -p /mnt/data/yolo/models/edge_trained
sudo mkdir -p /mnt/data/yolo/models/edge_trained/metadata
sudo mkdir -p /mnt/data/yolo/training/images
sudo mkdir -p /mnt/data/yolo/training/labels

# Set permissions
sudo chmod -R 777 /mnt/data/yolo

# Verify directories
ls -la /mnt/data/yolo/
```

## Step 2: Upload Base Model (One-time setup)

```bash
# From your local machine, upload the base YOLO model
scp -i ~/Downloads/bigKey.pem /Users/dmarshall94/Code/YOLOStuff/sp-edge-ml-app/yolov8n.pt ec2-user@<INSTANCE_IP>:~/

# SSH to instance and move to models directory
ssh -i ~/Downloads/bigKey.pem ec2-user@<INSTANCE_IP>
sudo mv ~/yolov8n.pt /mnt/data/yolo/models/production.pt
sudo chmod 644 /mnt/data/yolo/models/production.pt
```

## Step 3: Upload Training Data (if needed)

```bash
# From your local machine, upload training dataset
scp -i ~/Downloads/bigKey.pem -r <LOCAL_DATASET_PATH>/images ec2-user@<INSTANCE_IP>:~/
scp -i ~/Downloads/bigKey.pem -r <LOCAL_DATASET_PATH>/labels ec2-user@<INSTANCE_IP>:~/

# SSH to instance and move to training directory
ssh -i ~/Downloads/bigKey.pem ec2-user@<INSTANCE_IP>
sudo mv ~/images/* /mnt/data/yolo/training/images/
sudo mv ~/labels/* /mnt/data/yolo/training/labels/
```

## Step 4: Copy Application Code to Instance

```bash
# From your local machine
cd /Users/dmarshall94/Code/YOLOStuff/sp-edge-ml-app

# Copy yolo-sample directory
scp -i ~/Downloads/bigKey.pem -r yolo-sample ec2-user@<INSTANCE_IP>:~/

# Copy helm chart
scp -i ~/Downloads/bigKey.pem -r templates ec2-user@<INSTANCE_IP>:~/
scp -i ~/Downloads/bigKey.pem -r values.yaml Chart.yaml ec2-user@<INSTANCE_IP>:~/
```

## Step 5: Build Docker Images on Instance

```bash
# SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@<INSTANCE_IP>

# Build inference image
cd ~/yolo-sample/infer
sudo podman build -t localhost/edge-inference:latest .

# Build training image
cd ~/yolo-sample/train
sudo podman build -t localhost/edge-training:latest .
```

## Step 6: Save Images to k3s Agent Directory

```bash
# Create k3s images directory
sudo mkdir -p /var/lib/rancher/k3s/agent/images

# Remove old tars if they exist (important: no duplicates allowed)
sudo rm -f /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo rm -f /var/lib/rancher/k3s/agent/images/edge-training.tar

# Save images as tars
sudo podman save localhost/edge-inference:latest -o /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo podman save localhost/edge-training:latest -o /var/lib/rancher/k3s/agent/images/edge-training.tar

# Verify tars created
ls -lh /var/lib/rancher/k3s/agent/images/
```

**Important:** The `/var/lib/rancher/k3s/agent/images/` directory cannot contain duplicate filenames. Always remove old tar files before saving new ones.

## Step 7: Restart k3s to Import Images

```bash
# Restart k3s (imports images from agent/images directory)
sudo systemctl restart k3s

# Wait for k3s to come back up (30-60 seconds)
sleep 30

# Verify images imported
sudo /usr/local/bin/k3s ctr images ls | grep edge

# Should see:
# localhost/edge-inference:latest
# localhost/edge-training:latest
```

## Step 8: Deploy Helm Chart

```bash
# Create helm chart directory structure
cd ~
mkdir -p edge-ml-chart/templates

# Move files to chart structure
mv templates/* edge-ml-chart/templates/
mv values.yaml Chart.yaml edge-ml-chart/

# Deploy with Helm
sudo /usr/local/bin/helm install edge-ml ~/edge-ml-chart \
  --namespace default \
  --kubeconfig /etc/rancher/k3s/k3s.yaml

# Verify deployment
sudo /usr/local/bin/kubectl get pods --kubeconfig /etc/rancher/k3s/k3s.yaml
sudo /usr/local/bin/kubectl get svc --kubeconfig /etc/rancher/k3s/k3s.yaml
```

## Step 9: Verify Application is Running

```bash
# Check pods are running
sudo /usr/local/bin/kubectl get pods --kubeconfig /etc/rancher/k3s/k3s.yaml | grep edge

# Should see:
# edge-inference-xxxxx        1/1     Running
# edge-heartbeat-xxxxx        1/1     Running
# edge-training-job (suspended cronjob)

# Check logs
sudo /usr/local/bin/kubectl logs -l app=edge-inference --kubeconfig /etc/rancher/k3s/k3s.yaml

# Access the web UI (from your browser)
http://<INSTANCE_IP>:6767
```

---

## Updating the Application

When you make code changes:

```bash
# 1. Copy updated code from local machine
scp -i ~/Downloads/bigKey.pem -r /Users/dmarshall94/Code/YOLOStuff/sp-edge-ml-app/yolo-sample ec2-user@<INSTANCE_IP>:~/

# 2. SSH to instance
ssh -i ~/Downloads/bigKey.pem ec2-user@<INSTANCE_IP>

# 3. Rebuild images
cd ~/yolo-sample/infer
sudo podman build -t localhost/edge-inference:latest .

cd ~/yolo-sample/train
sudo podman build -t localhost/edge-training:latest .

# 4. Remove old tars and save new ones
sudo rm -f /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo rm -f /var/lib/rancher/k3s/agent/images/edge-training.tar

sudo podman save localhost/edge-inference:latest -o /var/lib/rancher/k3s/agent/images/edge-inference.tar
sudo podman save localhost/edge-training:latest -o /var/lib/rancher/k3s/agent/images/edge-training.tar

# 5. Restart k3s
sudo systemctl restart k3s

# 6. Wait for k3s to come back up
sleep 30

# 7. Restart deployments
sudo /usr/local/bin/kubectl rollout restart deployment edge-inference --kubeconfig /etc/rancher/k3s/k3s.yaml
```

---

## Complete Directory Structure

After deployment, your instance should have:

```
/mnt/data/yolo/
├── models/
│   ├── production.pt              # Current inference model
│   ├── yolov8n.pt                 # Base model (optional)
│   └── edge_trained/              # Edge-trained models
│       ├── edge_YYYYMMDD_HHMMSS.pt
│       └── metadata/
│           └── edge_YYYYMMDD_HHMMSS.json
└── training/
    ├── images/                    # Training images
    │   └── *.jpg
    └── labels/                    # Training labels
        └── *.txt

/home/ec2-user/
├── yolo-sample/
│   ├── infer/
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── infer.py
│   └── train/
│       ├── Dockerfile
│       ├── train.py
│       ├── dataset.yaml
│       └── fedavg_aggregate.py
└── edge-ml-chart/
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
        ├── infer-deployment.yaml
        ├── train-cronjob.yaml
        └── heartbeat-deployment.yaml

/var/lib/rancher/k3s/agent/images/
├── edge-inference.tar
└── edge-training.tar
```

---

## Troubleshooting

### Images not found in k3s
```bash
# Check if images exist in containerd
sudo /usr/local/bin/k3s ctr images ls | grep edge

# If missing, verify tars exist
ls -lh /var/lib/rancher/k3s/agent/images/

# Restart k3s to re-import
sudo systemctl restart k3s
```

### Pods stuck in Pending
```bash
# Describe the pod
sudo /usr/local/bin/kubectl describe pod <POD_NAME> --kubeconfig /etc/rancher/k3s/k3s.yaml

# Check PVC status
sudo /usr/local/bin/kubectl get pvc --kubeconfig /etc/rancher/k3s/k3s.yaml

# Check if directories exist
ls -la /mnt/data/yolo/
```

### Cannot access web UI
```bash
# Check service
sudo /usr/local/bin/kubectl get svc --kubeconfig /etc/rancher/k3s/k3s.yaml

# Check if pod is running
sudo /usr/local/bin/kubectl get pods --kubeconfig /etc/rancher/k3s/k3s.yaml | grep edge-inference

# Check logs
sudo /usr/local/bin/kubectl logs -l app=edge-inference --kubeconfig /etc/rancher/k3s/k3s.yaml
```

### Duplicate tar file error
```bash
# Error: "docker-archive doesn't support modifying existing images"
# Solution: Remove old tar file first
sudo rm -f /var/lib/rancher/k3s/agent/images/<IMAGE_NAME>.tar
sudo podman save localhost/<IMAGE_NAME>:latest -o /var/lib/rancher/k3s/agent/images/<IMAGE_NAME>.tar
```

---

## Quick Reference Commands

### Restart deployment only (no rebuild)
```bash
sudo /usr/local/bin/kubectl rollout restart deployment edge-inference --kubeconfig /etc/rancher/k3s/k3s.yaml
```

### View logs
```bash
sudo /usr/local/bin/kubectl logs -l app=edge-inference --kubeconfig /etc/rancher/k3s/k3s.yaml --tail=50 -f
```

### Trigger manual training
```bash
sudo /usr/local/bin/kubectl create job --from=cronjob/edge-training-job manual-training-1 --kubeconfig /etc/rancher/k3s/k3s.yaml
```

### Delete and redeploy
```bash
sudo /usr/local/bin/helm uninstall edge-ml --namespace default --kubeconfig /etc/rancher/k3s/k3s.yaml
sudo /usr/local/bin/helm install edge-ml ~/edge-ml-chart --namespace default --kubeconfig /etc/rancher/k3s/k3s.yaml
```
