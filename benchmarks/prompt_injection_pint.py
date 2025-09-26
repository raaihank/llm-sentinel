#!/usr/bin/env python3
"""
PINT Benchmark Runner for LLM-Sentinel
Tests prompt injection detection against Lakera's PINT dataset

Based on the official Lakera PINT benchmark methodology:
https://github.com/lakera-ai/pint-benchmark
"""

import json
import requests
import time
import pandas as pd
from pathlib import Path
from typing import Dict, Callable
import argparse
from ruamel.yaml import YAML
import tqdm


def evaluate_llm_sentinel(prompt: str, sentinel_url: str = "http://localhost:8080") -> tuple[bool, float]:
    """
    Evaluate LLM-Sentinel on a single prompt and return boolean indicating if flagged + latency.
    
    Args:
        prompt: The prompt to test
        sentinel_url: URL of the LLM-Sentinel instance
        
    Returns:
        tuple: (flagged: bool, latency_ms: float)
    """
    start_time = time.perf_counter()
    
    try:
        response = requests.post(
            f"{sentinel_url}/ollama/api/generate",
            json={
                "model": "test-model",  # Doesn't matter for blocking test
                "prompt": prompt,
                "stream": False
            },
            timeout=10
        )
        
        end_time = time.perf_counter()
        latency_ms = (end_time - start_time) * 1000
        
        # Request blocked = flagged as malicious
        return response.status_code == 403, latency_ms
        
    except Exception as e:
        end_time = time.perf_counter()
        latency_ms = (end_time - start_time) * 1000
        print(f"Error evaluating prompt: {e}")
        return False, latency_ms


def evaluate_dataset(df: pd.DataFrame, eval_function: Callable) -> tuple[pd.DataFrame, dict]:
    """
    Iterate through dataframe and call evaluation function on each input.
    
    Returns:
        Tuple of (accuracy_results, latency_stats)
    """
    df = df.copy()
    df["prediction"] = None
    df["latency_ms"] = None
    
    latencies = []
    
    # Evaluate each prompt
    for i, row in tqdm.tqdm(df.iterrows(), total=len(df), desc="Evaluating"):
        prediction, latency = eval_function(prompt=str(row["text"]))
        df.at[i, "prediction"] = prediction
        df.at[i, "latency_ms"] = latency
        latencies.append(latency)
        
        # Small delay to avoid overwhelming server
        time.sleep(0.05)
    
    df["correct"] = df["prediction"] == df["label"]
    
    # Calculate latency statistics
    latency_stats = {
        "mean_ms": float(pd.Series(latencies).mean()),
        "median_ms": float(pd.Series(latencies).median()),
        "p95_ms": float(pd.Series(latencies).quantile(0.95)),
        "p99_ms": float(pd.Series(latencies).quantile(0.99)),
        "min_ms": float(pd.Series(latencies).min()),
        "max_ms": float(pd.Series(latencies).max()),
        "std_ms": float(pd.Series(latencies).std()),
    }
    
    accuracy_results = (
        df.groupby(["category", "label"]).agg({"correct": ["mean", "sum", "count"]})
        .droplevel(0, axis=1)
        .rename(columns={"mean": "accuracy", "sum": "correct", "count": "total"})
    )
    
    return accuracy_results, latency_stats


def pint_benchmark(
    df: pd.DataFrame,
    model_name: str,
    eval_function: Callable[[str], tuple[bool, float]],
    quiet: bool = False,
    weight: str = "balanced",
) -> tuple[str, float, pd.DataFrame, dict]:
    """
    Evaluate a model on dataset and return benchmark results with latency.
    
    Args:
        df: DataFrame with 'text', 'category', 'label' columns
        model_name: Name of model being evaluated
        eval_function: Function that takes prompt and returns (prediction, latency_ms)
        quiet: If True, suppress printing results
        weight: 'balanced' or 'imbalanced' scoring method
        
    Returns:
        Tuple with (model_name, score, results_dataframe, latency_stats)
    """
    benchmark, latency_stats = evaluate_dataset(df=df, eval_function=eval_function)
    
    if weight == "imbalanced":
        score = benchmark["correct"].sum() / benchmark["total"].sum()
    else:
        # Balanced accuracy - mean of per-label accuracies
        score = float(
            benchmark.groupby("label")
            .agg({"total": "sum", "correct": "sum"})
            .assign(accuracy=lambda x: x["correct"] / x["total"])["accuracy"]
            .mean()
        )
    
    if not quiet:
        print("PINT Benchmark")
        print("=" * 50)
        print(f"Model: {model_name}")
        print(f"Score ({weight}): {round(score * 100, 4)}%")
        print("=" * 50)
        print("\n⚡ LATENCY METRICS:")
        print(f"Mean Latency:     {latency_stats['mean_ms']:.1f}ms")
        print(f"Median Latency:   {latency_stats['median_ms']:.1f}ms")
        print(f"95th Percentile:  {latency_stats['p95_ms']:.1f}ms")
        print(f"99th Percentile:  {latency_stats['p99_ms']:.1f}ms")
        print(f"Min Latency:      {latency_stats['min_ms']:.1f}ms")
        print(f"Max Latency:      {latency_stats['max_ms']:.1f}ms")
        print(f"Std Deviation:    {latency_stats['std_ms']:.1f}ms")
        print("=" * 50)
        print(benchmark)
        print("=" * 50)
        print(f"Date: {pd.to_datetime('today').strftime('%Y-%m-%d')}")
        print("=" * 50)
    
    return (model_name, score, benchmark, latency_stats)


def main():
    parser = argparse.ArgumentParser(description='Run PINT benchmark against LLM-Sentinel')
    parser.add_argument('--dataset', required=True, help='Path to PINT dataset file (.yaml or .jsonl)')
    parser.add_argument('--url', default='http://localhost:8080', help='LLM-Sentinel URL')
    parser.add_argument('--max-tests', type=int, help='Maximum number of tests to run')
    parser.add_argument('--format', choices=['yaml', 'jsonl'], default='yaml', help='Dataset format')
    parser.add_argument('--quiet', action='store_true', help='Suppress detailed output')
    
    args = parser.parse_args()
    
    # Load dataset based on format
    if args.format == 'yaml':
        yaml_data = YAML().load(Path(args.dataset))
        df = pd.DataFrame.from_records(yaml_data)
    else:  # jsonl
        with open(args.dataset, 'r') as f:
            data = [json.loads(line) for line in f]
        df = pd.DataFrame(data)
        
        # Convert JSONL format to PINT format if needed
        if 'text' not in df.columns and 'prompt' in df.columns:
            df['text'] = df['prompt']
        if 'category' not in df.columns:
            df['category'] = 'unknown'
        if 'label' not in df.columns and 'is_injection' in df.columns:
            df['label'] = df['is_injection']
    
    # Limit dataset size if requested
    if args.max_tests:
        df = df.head(args.max_tests)
    
    print(f"📊 Dataset loaded: {len(df)} samples")
    print(f"📋 Categories: {df['category'].value_counts().to_dict()}")
    print(f"🏷️  Labels: {df['label'].value_counts().to_dict()}")
    print()
    
    # Create evaluation function with URL
    def eval_func(prompt: str) -> tuple[bool, float]:
        return evaluate_llm_sentinel(prompt, args.url)
    
    # Run benchmark
    model_name, score, results, latency_stats = pint_benchmark(
        df=df,
        model_name="LLM-Sentinel",
        eval_function=eval_func,
        quiet=args.quiet,
        weight="balanced"
    )
    
    # Save results
    results_file = f"pint_results_{pd.to_datetime('today').strftime('%Y%m%d_%H%M%S')}.csv"
    results.to_csv(results_file)
    print(f"💾 Results saved to: {results_file}")


if __name__ == "__main__":
    main()
