#!/usr/bin/env python3
"""
Training Script - Run by cronjob every 3 hours
Trains YOLO model and saves to shared model directory
"""

import time
import glob
from pathlib import Path
from ultralytics import YOLO
from datetime import datetime
import json
import os

# Configuration
TRAINING_DIR = os.getenv('TRAINING_DIR', '/data/training')
MODEL_DIR = os.getenv('MODEL_DIR', './models')
EPOCHS = int(os.getenv('EPOCHS', '20'))
BATCH_SIZE = int(os.getenv('BATCH_SIZE', '4'))
DEVICE = os.getenv('DEVICE', 'cpu')
DATASET_YAML = '/app/dataset.yaml'  # Built into the image

def train_model():
    """Train YOLOv8-nano on hot dog images"""
    
    # Create model directory if it doesn't exist
    Path(MODEL_DIR).mkdir(parents=True, exist_ok=True)

    # Check if images exist in training directory
    images_dir = f'{TRAINING_DIR}/images'
    images = glob.glob(f'{images_dir}/*')
    if not images:
        print(f"ERROR: No images found in {images_dir}")
        return False

    print("="*60)
    print(f"Starting training at {datetime.now()}")
    print(f"Training directory: {TRAINING_DIR}")
    print(f"Found {len(images)} images")
    print("="*60)

    # Load existing model from shared storage (or use yolov8n.pt as fallback)
    base_model_path = f'{MODEL_DIR}/latest.pt'
    if not Path(base_model_path).exists():
        print(f"WARNING: {base_model_path} not found, using yolov8n.pt")
        base_model_path = 'yolov8n.pt'
    else:
        print(f"Loading base model from: {base_model_path}")

    model = YOLO(base_model_path)
    
    # Train
    start_time = time.time()
    print(f"Using dataset config: {DATASET_YAML}")
    model.train(
        data=DATASET_YAML,
        epochs=EPOCHS,
        imgsz=640,
        batch=BATCH_SIZE,
        device=DEVICE,
        project='training_runs',
        name='edge_training',
        exist_ok=True,
        verbose=True
    )
    
    training_time = time.time() - start_time
    
    # Get the best model
    best_model_path = 'training_runs/edge_training/weights/best.pt'
    
    if not Path(best_model_path).exists():
        print("ERROR: Training failed, no model produced")
        return False
    
    # Generate version timestamp
    version = datetime.now().strftime('%Y%m%d_%H%M%S')
    
    # Copy best model to shared location with version
    versioned_model = f'{MODEL_DIR}/model_{version}.pt'
    latest_model = f'{MODEL_DIR}/latest.pt'
    
    # Save versioned model
    import shutil
    shutil.copy(best_model_path, versioned_model)
    shutil.copy(best_model_path, latest_model)
    
    # Save metadata
    metadata = {
        'version': version,
        'trained_at': datetime.now().isoformat(),
        'num_images': len(images),
        'epochs': EPOCHS,
        'training_time_seconds': training_time,
        'model_path': versioned_model
    }
    
    with open(f'{MODEL_DIR}/latest_metadata.json', 'w') as f:
        json.dump(metadata, f, indent=2)
    
    print("="*60)
    print(f"✓ Training complete!")
    print(f"  Version: {version}")
    print(f"  Training time: {training_time:.2f}s")
    print(f"  Model saved to: {versioned_model}")
    print(f"  Latest model: {latest_model}")
    print("="*60)
    
    return True

def main():
    print("YOLO Training Job")
    print(f"Timestamp: {datetime.now()}")
    
    success = train_model()
    
    if success:
        print("Training job completed successfully")
        exit(0)
    else:
        print("Training job failed")
        exit(1)

if __name__ == "__main__":
    main()