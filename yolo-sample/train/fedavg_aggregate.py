#!/usr/bin/env python3
"""
FedAvg Aggregation Script - Manual Federated Learning Aggregation

This script implements the Federated Averaging (FedAvg) algorithm to combine
multiple edge-trained models into a single global model.

Usage:
    python fedavg_aggregate.py [--models-dir PATH] [--output PATH]

The script will:
1. Find all edge models in edge_trained/
2. Load metadatåa to get training sample counts
3. Perform weighted averaging (FedAvg algorithm)
4. Save aggregated model

Reference: https://arxiv.org/abs/1602.05629 (McMahan et al., 2016)
"""

import argparse
import json
import glob
from pathlib import Path
from collections import OrderedDict
import torch
from datetime import datetime


def load_edge_models(models_dir):
    """
    Load all edge models and their metadata from the edge_trained directory.

    Returns:
        List of tuples: (model_path, metadata_dict)
    """
    models = []
    metadata_dir = Path(models_dir) / 'metadata'

    # Find all metadata files
    metadata_files = glob.glob(str(metadata_dir / 'edge_*.json'))

    if not metadata_files:
        raise ValueError(f"No edge model metadata found in {metadata_dir}")

    print(f"Found {len(metadata_files)} edge models")
    print("="*60)

    for metadata_file in sorted(metadata_files):
        with open(metadata_file, 'r') as f:
            metadata = json.load(f)

        model_path = metadata['model_path']

        if not Path(model_path).exists():
            print(f"⚠️  Warning: Model file not found: {model_path}")
            continue

        models.append((model_path, metadata))

        print(f"Edge Model {len(models)}:")
        print(f"  Node: {metadata['node_id']}")
        print(f"  Version: {metadata['version']}")
        print(f"  Training samples: {metadata['num_images']}")
        print(f"  Epochs: {metadata['epochs']}")
        print(f"  Path: {model_path}")
        print()

    return models


def fedavg_aggregate(models_data):
    """
    Implement FedAvg algorithm: weighted average of model parameters.

    FedAvg formula:
        w_global = Σ(n_k / n_total) * w_k

    where:
        w_k = parameters from edge device k
        n_k = number of training samples on device k
        n_total = total training samples across all devices

    Args:
        models_data: List of (model_path, metadata) tuples

    Returns:
        OrderedDict containing aggregated model state_dict
    """
    # Calculate total samples for weighting
    total_samples = sum(metadata['num_images'] for _, metadata in models_data)

    print("="*60)
    print("Starting FedAvg Aggregation")
    print(f"Total training samples across all edges: {total_samples}")
    print("="*60)

    # Initialize aggregated state dict
    aggregated_state_dict = None

    for model_path, metadata in models_data:
        num_samples = metadata['num_images']
        weight = num_samples / total_samples

        print(f"\nProcessing {metadata['node_id']}:")
        print(f"  Samples: {num_samples}")
        print(f"  Weight: {weight:.4f} ({weight*100:.1f}%)")

        # Load model state dict
        try:
            checkpoint = torch.load(model_path, map_location='cpu', weights_only=False)

            # Handle different checkpoint formats
            if isinstance(checkpoint, dict) and 'model' in checkpoint:
                state_dict = checkpoint['model'].float().state_dict()
            elif hasattr(checkpoint, 'state_dict'):
                state_dict = checkpoint.state_dict()
            else:
                state_dict = checkpoint

        except Exception as e:
            print(f"  ⚠️  Error loading model: {e}")
            continue

        # Initialize or aggregate
        if aggregated_state_dict is None:
            # First model: initialize with weighted parameters
            aggregated_state_dict = OrderedDict()
            for key, value in state_dict.items():
                aggregated_state_dict[key] = value.clone() * weight
        else:
            # Subsequent models: add weighted parameters
            for key, value in state_dict.items():
                if key in aggregated_state_dict:
                    aggregated_state_dict[key] += value.clone() * weight
                else:
                    print(f"  ⚠️  Warning: Key {key} not found in previous models")

        print(f"  ✓ Aggregated")

    if aggregated_state_dict is None:
        raise ValueError("No models were successfully loaded for aggregation")

    return aggregated_state_dict


def save_aggregated_model(state_dict, output_path, models_data):
    """
    Save the aggregated model with metadata in proper YOLO checkpoint format.
    """
    # Create output directory if needed
    output_path = Path(output_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Prepare metadata
    metadata = {
        'aggregation_method': 'FedAvg',
        'aggregated_at': datetime.now().isoformat(),
        'num_edge_models': len(models_data),
        'total_training_samples': sum(m['num_images'] for _, m in models_data),
        'edge_models': [
            {
                'node_id': m['node_id'],
                'version': m['version'],
                'num_images': m['num_images'],
                'weight': m['num_images'] / sum(md['num_images'] for _, md in models_data)
            }
            for _, m in models_data
        ]
    }

    # Load a base checkpoint to get proper YOLO structure
    base_model_path = models_data[0][0]  # Use first model as base
    base_checkpoint = torch.load(base_model_path, map_location='cpu', weights_only=False)

    # Load the aggregated state_dict into the checkpoint's model
    base_checkpoint['model'].load_state_dict(state_dict)

    # Save the full checkpoint (this will be compressed by PyTorch)
    torch.save(base_checkpoint, output_path)

    # Save metadata alongside
    metadata_path = output_path.parent / f"{output_path.stem}_metadata.json"
    with open(metadata_path, 'w') as f:
        json.dump(metadata, f, indent=2)

    print("\n" + "="*60)
    print("✓ Federated Aggregation Complete!")
    print(f"  Algorithm: FedAvg (Federated Averaging)")
    print(f"  Edge models combined: {len(models_data)}")
    print(f"  Total training samples: {metadata['total_training_samples']}")
    print(f"  Output model: {output_path}")
    print(f"  Metadata: {metadata_path}")
    print("="*60)

    return metadata


def main():
    parser = argparse.ArgumentParser(
        description='Aggregate edge-trained models using FedAvg algorithm'
    )
    parser.add_argument(
        '--models-dir',
        type=str,
        default='/data/models/edge_trained',
        help='Directory containing edge-trained models (default: /data/models/edge_trained)'
    )
    parser.add_argument(
        '--output',
        type=str,
        default='/data/models/aggregated_global.pt',
        help='Output path for aggregated model (default: /data/models/aggregated_global.pt)'
    )

    args = parser.parse_args()

    print("="*60)
    print("FedAvg Federated Learning Aggregation")
    print("="*60)
    print(f"Models directory: {args.models_dir}")
    print(f"Output path: {args.output}")
    print()

    try:
        # Load edge models and metadata
        models_data = load_edge_models(args.models_dir)

        if len(models_data) == 0:
            print("❌ No valid edge models found for aggregation")
            return 1

        # Perform FedAvg aggregation
        aggregated_state_dict = fedavg_aggregate(models_data)

        # Save aggregated model
        save_aggregated_model(aggregated_state_dict, args.output, models_data)

        print("\n📝 Next steps:")
        print("1. Copy aggregated model to production:")
        print(f"   cp {args.output} /data/models/production.pt")
        print("2. Inference deployment will automatically use new model")
        print("3. Test with hotdog image to verify improvement")

        return 0

    except Exception as e:
        print(f"\n❌ Error during aggregation: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == "__main__":
    exit(main())
