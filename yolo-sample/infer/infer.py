#!/usr/bin/env python3
"""
Inference Script - Run inference on a single image and return JSON results
"""

import sys
import json
from pathlib import Path
from ultralytics import YOLO
import os

# Configuration
MODEL_DIR = os.getenv('MODEL_DIR', './models')

def load_model():
    """Load the production model (aggregated from gateway)"""
    # Use production model (good at everything) instead of local trained model
    model_path = f'{MODEL_DIR}/production.pt'

    if not Path(model_path).exists():
        # Fallback to base model if production doesn't exist yet
        model_path = f'{MODEL_DIR}/yolov8n.pt'
        if not Path(model_path).exists():
            return None, f"No model found at {model_path}"

    try:
        model = YOLO(model_path)
        return model, None
    except Exception as e:
        return None, str(e)

def run_inference(model, image_path):
    """Run inference on a single image and return results as JSON"""
    if not Path(image_path).exists():
        return {"error": f"Image not found: {image_path}"}

    try:
        # Run inference with lower confidence threshold for federated model
        results = model(image_path, verbose=False)

        detections = []
        for r in results:
            if len(r.boxes) > 0:
                for box in r.boxes:
                    conf = float(box.conf[0].item())
                    cls = int(box.cls[0].item())
                    cls_name = r.names[cls]

                    # Get bounding box coordinates
                    xyxy = box.xyxy[0].tolist()

                    detections.append({
                        "class_id": cls,
                        "class_name": cls_name,
                        "confidence": round(conf, 3),
                        "bbox": {
                            "x1": round(xyxy[0], 2),
                            "y1": round(xyxy[1], 2),
                            "x2": round(xyxy[2], 2),
                            "y3": round(xyxy[3], 2)
                        }
                    })

        return {
            "image": Path(image_path).name,
            "detections": detections,
            "count": len(detections)
        }
    except Exception as e:
        return {"error": str(e)}

def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "Usage: python infer.py <image_path>"}))
        sys.exit(1)

    image_path = sys.argv[1]

    # Load model
    model, error = load_model()
    if error:
        print(json.dumps({"error": error}))
        sys.exit(1)

    # Run inference
    result = run_inference(model, image_path)
    print(json.dumps(result, indent=2))

if __name__ == "__main__":
    main()
