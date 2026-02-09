#!/usr/bin/env python3
# Copyright 2026 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import json
import sys
import os
import math
from datetime import datetime

try:
    import matplotlib.pyplot as plt
    import matplotlib.dates as mdates
    from matplotlib.backends.backend_pdf import PdfPages
except ImportError:
    print("Error: matplotlib is not installed. Please install it using 'pip install matplotlib'")
    sys.exit(1)

# Configuration
SUBPLOTS_PER_PAGE = 8
COLS = 2
ROWS = 4

def parse_args():
    parser = argparse.ArgumentParser(description='Visualize Karmada performance metrics comparison')
    parser.add_argument('--baseline', required=True, help='Path to the baseline metrics JSON file')
    parser.add_argument('--target', required=True, help='Path to the target metrics JSON file')
    parser.add_argument('--output-dir', default='performance-comparison', help='Directory to write all output images into (created if not exists)')
    parser.add_argument('--output', default='metrics-report', help='Prefix for output images (e.g., metrics-report -> metrics-report-01.png)')
    return parser.parse_args()

def load_json(filepath):
    try:
        with open(filepath, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"Error loading {filepath}: {e}")
        sys.exit(1)

def extract_timeseries(data, metric_path):
    """
    Extract time series data from the nested JSON structure.
    metric_path is a list of keys.
    Returns a list of (timestamp, value) tuples.
    """
    tgt = data
    for key in metric_path:
        if isinstance(tgt, dict) and key in tgt:
            tgt = tgt[key]
        else:
            return []
    
    if isinstance(tgt, list):
        result = []
        for item in tgt:
            if len(item) == 2:
                try:
                    ts = float(item[0])
                    val = float(item[1])
                    result.append((ts, val))
                except ValueError:
                    continue
        return result
    return []

def normalize_time(timeseries):
    """
    Normalize timestamps to start from 0.
    Returns (relative_timestamps, values)
    """
    if not timeseries:
        return [], []
    
    start_time = timeseries[0][0]
    relative_times = [t[0] - start_time for t in timeseries]
    values = [t[1] for t in timeseries]
    return relative_times, values

class Plotter:
    def __init__(self, baseline_data, target_data, output_prefix):
        self.baseline = baseline_data
        self.target = target_data
        self.output_prefix = output_prefix
        self.page_count = 1
        self.target_subplots = [] # List of plot functions

    def add_plot(self, title, ylabel, plot_func):
        self.target_subplots.append((title, ylabel, plot_func))
        if len(self.target_subplots) >= SUBPLOTS_PER_PAGE:
            self.flush_page()

    def flush_page(self):
        if not self.target_subplots:
            return

        fig, axes = plt.subplots(ROWS, COLS, figsize=(16, 4 * ROWS))
        axes = axes.flatten()
        
        for i, (title, ylabel, plot_func) in enumerate(self.target_subplots):
            ax = axes[i]
            plot_func(ax)
            ax.set_title(title, fontsize=10)
            ax.set_xlabel('Time (s)', fontsize=8)
            ax.set_ylabel(ylabel, fontsize=8)
            ax.grid(True, linestyle=':', alpha=0.6)
            # Only add legend if there are labels
            handles, labels = ax.get_legend_handles_labels()
            if labels:
                ax.legend(fontsize=8)

        # Hide unused subplots
        for i in range(len(self.target_subplots), len(axes)):
            fig.delaxes(axes[i])
            
        plt.tight_layout()
        output_file = f"{self.output_prefix}-{self.page_count:02d}.png"
        plt.savefig(output_file)
        print(f"Saved {output_file}")
        plt.close(fig)
        
        self.target_subplots = []
        self.page_count += 1

    def finish(self):
        self.flush_page()

def plot_latency_group(ax, baseline, target, base_path, title_suffix="Latency"):
    """
    Plots P50, P90, P99 for target, and P50, P90, P99 for baseline.
    """
    # Target
    p50 = extract_timeseries(target, base_path + ['p50_seconds'])
    p90 = extract_timeseries(target, base_path + ['p90_seconds'])
    p99 = extract_timeseries(target, base_path + ['p99_seconds'])
    
    # Baseline
    b_p50 = extract_timeseries(baseline, base_path + ['p50_seconds'])
    b_p90 = extract_timeseries(baseline, base_path + ['p90_seconds'])
    b_p99 = extract_timeseries(baseline, base_path + ['p99_seconds'])
    
    t_p50, v_p50 = normalize_time(p50)
    t_p90, v_p90 = normalize_time(p90)
    t_p99, v_p99 = normalize_time(p99)
    
    t_b50, v_b50 = normalize_time(b_p50)
    t_b90, v_b90 = normalize_time(b_p90)
    t_b99, v_b99 = normalize_time(b_p99)
    
    # Plot Baseline with dashed lines
    if t_b50: ax.plot(t_b50, v_b50, '--', color='lightgreen', alpha=0.6, label='Base P50')
    if t_b90: ax.plot(t_b90, v_b90, '--', color='moccasin', alpha=0.6, label='Base P90')
    if t_b99: ax.plot(t_b99, v_b99, '--', color='lightcoral', alpha=0.6, label='Base P99')

    # Plot Target with solid lines
    if t_p50: ax.plot(t_p50, v_p50, color='green', alpha=0.8, label='Target P50')
    if t_p90: ax.plot(t_p90, v_p90, color='orange', alpha=0.8, label='Target P90')
    if t_p99: ax.plot(t_p99, v_p99, color='red', alpha=0.8, label='Target P99')

def plot_rate_group(ax, baseline, target, base_path, success_key='success', error_key='error'):
    """
    Plots Success Rate and Error Rate.
    """
    # Target
    succ = extract_timeseries(target, base_path + [success_key, 'throughput', 'rate_per_second'])
    err = extract_timeseries(target, base_path + [error_key, 'throughput', 'rate_per_second'])
    
    # Baseline
    b_succ = extract_timeseries(baseline, base_path + [success_key, 'throughput', 'rate_per_second'])
    b_err = extract_timeseries(baseline, base_path + [error_key, 'throughput', 'rate_per_second'])
    
    t_succ, v_succ = normalize_time(succ)
    t_err, v_err = normalize_time(err)
    t_b_succ, v_b_succ = normalize_time(b_succ)
    t_b_err, v_b_err = normalize_time(b_err)
    
    if t_b_succ: ax.plot(t_b_succ, v_b_succ, '--', color='lightgreen', alpha=0.6, label='Base Success')
    if t_b_err: ax.plot(t_b_err, v_b_err, '--', color='pink', alpha=0.6, label='Base Error')
    if t_succ: ax.plot(t_succ, v_succ, color='green', label='Target Success')
    if t_err: ax.plot(t_err, v_err, color='red', label='Target Error')

def plot_count_group(ax, baseline, target, base_path, success_key='success', error_key='error'):
    """
    Plots Total Count (Accumulated).
    """
    # Helper to build path properly whether it's nested or flat
    def build_path(base, key):
        if key == 'throughput': # Special handling for controller metrics structure which is flatter
             return base + ['throughput', 'total_count']
        if key == 'error_count': # Special handling for controller metrics error count
             return base + ['throughput', 'error_count']
        return base + [key, 'throughput', 'total_count']

    # Target
    succ_path = build_path(base_path, success_key)
    err_path = build_path(base_path, error_key)
    
    succ = extract_timeseries(target, succ_path)
    err = extract_timeseries(target, err_path)
    
    # Baseline
    b_succ = extract_timeseries(baseline, succ_path)
    b_err = extract_timeseries(baseline, err_path)
    
    t_succ, v_succ = normalize_time(succ)
    t_err, v_err = normalize_time(err)
    t_b_succ, v_b_succ = normalize_time(b_succ)
    t_b_err, v_b_err = normalize_time(b_err)
    
    if t_b_succ: ax.plot(t_b_succ, v_b_succ, '--', color='lightgreen', alpha=0.6, label='Base Total')
    if t_b_err: ax.plot(t_b_err, v_b_err, '--', color='pink', alpha=0.6, label='Base Error')
    if t_succ: ax.plot(t_succ, v_succ, color='green', label='Target Total')
    if t_err: ax.plot(t_err, v_err, color='red', label='Target Error')

def plot_depth_group(ax, baseline, target, base_path):
    """
    Plots Queue Depth (Max and Avg).
    """
    # Target
    c_max = extract_timeseries(target, base_path + ['depth', 'max'])
    c_avg = extract_timeseries(target, base_path + ['depth', 'avg'])
    
    # Baseline
    b_max = extract_timeseries(baseline, base_path + ['depth', 'max'])
    b_avg = extract_timeseries(baseline, base_path + ['depth', 'avg'])
    
    t_c_max, v_c_max = normalize_time(c_max)
    t_c_avg, v_c_avg = normalize_time(c_avg)
    t_b_max, v_b_max = normalize_time(b_max)
    t_b_avg, v_b_avg = normalize_time(b_avg)
    
    # Plot Baseline with dashed lines
    if t_b_max: ax.plot(t_b_max, v_b_max, '--', color='lightcoral', alpha=0.5, label='Base Max')
    if t_b_avg: ax.plot(t_b_avg, v_b_avg, '--', color='lightblue', alpha=0.5, label='Base Avg')

    # Plot Target with solid lines
    if t_c_max: ax.plot(t_c_max, v_c_max, color='red', alpha=0.8, label='Target Max')
    if t_c_avg: ax.plot(t_c_avg, v_c_avg, color='blue', alpha=0.8, label='Target Avg')

def plot_workqueue_count_group(ax, baseline, target, base_path):
    """
    Plots Workqueue Counts (Total Adds and Total Retries).
    """
    # Target
    adds = extract_timeseries(target, base_path + ['throughput', 'total_adds'])
    retries = extract_timeseries(target, base_path + ['throughput', 'total_retries'])
    
    # Baseline
    b_adds = extract_timeseries(baseline, base_path + ['throughput', 'total_adds'])
    b_retries = extract_timeseries(baseline, base_path + ['throughput', 'total_retries'])
    
    t_adds, v_adds = normalize_time(adds)
    t_retries, v_retries = normalize_time(retries)
    t_b_adds, v_b_adds = normalize_time(b_adds)
    t_b_retries, v_b_retries = normalize_time(b_retries)
    
    if t_b_adds: ax.plot(t_b_adds, v_b_adds, '--', color='lightgreen', alpha=0.6, label='Base Adds')
    if t_b_retries: ax.plot(t_b_retries, v_b_retries, '--', color='pink', alpha=0.6, label='Base Retries')
    if t_adds: ax.plot(t_adds, v_adds, color='green', label='Target Adds')
    if t_retries: ax.plot(t_retries, v_retries, color='red', label='Target Retries')

def plot_simple_metric(ax, baseline, target, path, label):
    tgt = extract_timeseries(target, path)
    base = extract_timeseries(baseline, path)
    
    t_c, v_c = normalize_time(tgt)
    t_b, v_b = normalize_time(base)
    
    if t_b: ax.plot(t_b, v_b, '--', color='gray', alpha=0.5, label=f'Base {label}')
    if t_c: ax.plot(t_c, v_c, label=f'Target {label}')

def plot_rest_client_rate(ax, baseline, target, base_path):
    """
    Plots REST Client Rates (Total, 2xx, 4xx).
    """
    # Target
    c_total = extract_timeseries(target, base_path + ['total', 'rate_per_second'])
    c_2xx = extract_timeseries(target, base_path + ['2xx', 'rate_per_second'])
    c_4xx = extract_timeseries(target, base_path + ['4xx', 'rate_per_second'])
    
    # Baseline
    b_total = extract_timeseries(baseline, base_path + ['total', 'rate_per_second'])
    
    t_c_tot, v_c_tot = normalize_time(c_total)
    t_c_2xx, v_c_2xx = normalize_time(c_2xx)
    t_c_4xx, v_c_4xx = normalize_time(c_4xx)
    t_b_tot, v_b_tot = normalize_time(b_total)
    
    if t_b_tot: ax.plot(t_b_tot, v_b_tot, '--', color='gray', alpha=0.5, label='Base Total')
    if t_c_tot: ax.plot(t_c_tot, v_c_tot, color='blue', label='Target Total')
    if t_c_2xx: ax.plot(t_c_2xx, v_c_2xx, color='green', label='Target 2xx')
    if t_c_4xx: ax.plot(t_c_4xx, v_c_4xx, color='red', label='Target 4xx')

def plot_rest_client_count(ax, baseline, target, base_path):
    """
    Plots REST Client Counts (Total, 2xx, 4xx).
    """
    # Target
    c_total = extract_timeseries(target, base_path + ['total', 'count'])
    c_2xx = extract_timeseries(target, base_path + ['2xx', 'count'])
    c_4xx = extract_timeseries(target, base_path + ['4xx', 'count'])
    
    # Baseline
    b_total = extract_timeseries(baseline, base_path + ['total', 'count'])
    
    t_c_tot, v_c_tot = normalize_time(c_total)
    t_c_2xx, v_c_2xx = normalize_time(c_2xx)
    t_c_4xx, v_c_4xx = normalize_time(c_4xx)
    t_b_tot, v_b_tot = normalize_time(b_total)
    
    if t_b_tot: ax.plot(t_b_tot, v_b_tot, '--', color='gray', alpha=0.5, label='Base Total')
    if t_c_tot: ax.plot(t_c_tot, v_c_tot, color='blue', label='Target Total')
    if t_c_2xx: ax.plot(t_c_2xx, v_c_2xx, color='green', label='Target 2xx')
    if t_c_4xx: ax.plot(t_c_4xx, v_c_4xx, color='red', label='Target 4xx')

def plot_resource_usage(ax, baseline, target, base_path, resource_type, unit_label, scale_factor=1.0):
    """
    Plots Resource Usage (CPU or Memory).
    """
    # Target
    if resource_type == 'cpu':
        metric_key = 'usage_cores_per_second'
    else:
        metric_key = 'working_set_bytes'
        
    c_data = extract_timeseries(target, base_path + [resource_type, metric_key])
    b_data = extract_timeseries(baseline, base_path + [resource_type, metric_key])
    
    t_c, v_c = normalize_time(c_data)
    t_b, v_b = normalize_time(b_data)
    
    # Apply scaling (e.g., Bytes -> MB)
    if scale_factor != 1.0:
        v_c = [v * scale_factor for v in v_c]
        v_b = [v * scale_factor for v in v_b]
    
    if t_b: ax.plot(t_b, v_b, '--', color='gray', alpha=0.5, label=f'Base {resource_type.upper()}')
    if t_c: ax.plot(t_c, v_c, label=f'Target {resource_type.upper()}')

def main():
    args = parse_args()
    os.makedirs(args.output_dir, exist_ok=True)
    baseline = load_json(args.baseline)
    target = load_json(args.target)
    
    output_prefix = os.path.join(args.output_dir, args.output)
    plotter = Plotter(baseline, target, output_prefix)
    
    # --- Policy Match ---
    plotter.add_plot("Policy Match Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'policy_match', 'latency']))
    plotter.add_plot("Policy Match Rate", "Ops/s", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'policy_match', 'throughput', 'rate_per_second'], "Rate"))
    plotter.add_plot("Policy Match Count", "Total", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'policy_match', 'throughput', 'total_count'], "Count"))

    # --- Policy Apply ---
    plotter.add_plot("Policy Apply Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'policy_apply', 'success', 'latency']))
    plotter.add_plot("Policy Apply Rate", "Ops/s", 
                     lambda ax: plot_rate_group(ax, baseline, target, ['metrics', 'policy_apply']))
    plotter.add_plot("Policy Apply Count", "Total", 
                     lambda ax: plot_count_group(ax, baseline, target, ['metrics', 'policy_apply']))

    # --- Binding Sync ---
    plotter.add_plot("Binding Sync Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'binding_sync', 'success', 'latency']))
    plotter.add_plot("Binding Sync Rate", "Ops/s", 
                     lambda ax: plot_rate_group(ax, baseline, target, ['metrics', 'binding_sync']))
    plotter.add_plot("Binding Sync Count", "Total", 
                     lambda ax: plot_count_group(ax, baseline, target, ['metrics', 'binding_sync']))

    # --- Work Sync ---
    plotter.add_plot("Work Sync Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'work_sync', 'success', 'latency']))
    plotter.add_plot("Work Sync Rate", "Ops/s", 
                     lambda ax: plot_rate_group(ax, baseline, target, ['metrics', 'work_sync']))
    plotter.add_plot("Work Sync Count", "Total", 
                     lambda ax: plot_count_group(ax, baseline, target, ['metrics', 'work_sync']))

    # --- Cluster Sync ---
    plotter.add_plot("Cluster Sync Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'cluster_sync', 'latency']))
    plotter.add_plot("Cluster Sync Rate", "Ops/s", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'cluster_sync', 'throughput', 'rate_per_second'], "Rate"))
    plotter.add_plot("Cluster Sync Count", "Total", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'cluster_sync', 'throughput', 'total_count'], "Count"))

    # --- Scheduler ---
    plotter.add_plot("Scheduler E2E Success Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'scheduler', 'e2e_scheduling', 'scheduled', 'latency']))
    plotter.add_plot("Scheduler E2E Error Latency", "Seconds", 
                     lambda ax: plot_latency_group(ax, baseline, target, ['metrics', 'scheduler', 'e2e_scheduling', 'error', 'latency']))
    plotter.add_plot("Scheduler Attempts Rate", "Ops/s", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'scheduler', 'scheduling_attempts', 'throughput', 'rate_per_second'], "Rate"))
    plotter.add_plot("Scheduler Attempts Count", "Total", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'scheduler', 'scheduling_attempts', 'throughput', 'total_count'], "Count"))
    plotter.add_plot("Scheduler Queue Incoming Rate", "Ops/s", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'scheduler', 'queue', 'incoming_bindings', 'rate_per_second'], "Rate"))
    plotter.add_plot("Scheduler Queue Incoming Count", "Total", 
                     lambda ax: plot_simple_metric(ax, baseline, target, ['metrics', 'scheduler', 'queue', 'incoming_bindings', 'total_count'], "Count"))

    # --- Controller Runtime (Dynamic) ---
    # Iterate over controllers found in the metrics
    controllers = target.get('metrics', {}).get('controller_runtime', {}).keys()
    for ctrl in sorted(controllers):
        base_path = ['metrics', 'controller_runtime', ctrl, 'reconciliation']
        
        plotter.add_plot(f"Controller: {ctrl} Latency", "Seconds", 
                         lambda ax, p=base_path: plot_latency_group(ax, baseline, target, p + ['latency']))
        
        plotter.add_plot(f"Controller: {ctrl} Rate", "Ops/s", 
                         lambda ax, p=base_path: plot_simple_metric(ax, baseline, target, p + ['throughput', 'total_rate_per_second'], "Rate"))
        
        plotter.add_plot(f"Controller: {ctrl} Count", "Total", 
                         lambda ax, p=base_path: plot_count_group(ax, baseline, target, p, 'throughput', 'error_count'))

    # --- Workqueue (Dynamic) ---
    queues = target.get('metrics', {}).get('workqueue', {}).keys()
    # Filter out _no_queues
    queues = [q for q in queues if q != "_no_instances" and q != "_no_queues"]
    
    for q in sorted(queues):
        base_path = ['metrics', 'workqueue', q]
        
        plotter.add_plot(f"Queue: {q} Depth", "Count", 
                         lambda ax, p=base_path: plot_depth_group(ax, baseline, target, p))
        
        plotter.add_plot(f"Queue: {q} Queue Latency", "Seconds", 
                         lambda ax, p=base_path: plot_latency_group(ax, baseline, target, p + ['queue_latency']))
        
        plotter.add_plot(f"Queue: {q} Work Latency", "Seconds", 
                         lambda ax, p=base_path: plot_latency_group(ax, baseline, target, p + ['work_latency']))
        
        plotter.add_plot(f"Queue: {q} Add Rate", "Ops/s", 
                         lambda ax, p=base_path: plot_simple_metric(ax, baseline, target, p + ['throughput', 'adds_per_second'], "Adds"))
        
        plotter.add_plot(f"Queue: {q} Counts", "Total", 
                         lambda ax, p=base_path: plot_workqueue_count_group(ax, baseline, target, p))

    # --- REST Client (Dynamic) ---
    hosts = target.get('metrics', {}).get('rest_client', {}).keys()
    # Filter out special keys like _no_instances if they exist
    hosts = [h for h in hosts if h != "_no_instances"]
    
    for h in sorted(hosts):
        base_path = ['metrics', 'rest_client', h]
        
        plotter.add_plot(f"REST: {h} Rate", "Ops/s", 
                         lambda ax, p=base_path: plot_rest_client_rate(ax, baseline, target, p))
        
        plotter.add_plot(f"REST: {h} Count", "Total", 
                         lambda ax, p=base_path: plot_rest_client_count(ax, baseline, target, p))

    # --- Component Resources ---
    # etcd
    plotter.add_plot("Etcd CPU Usage", "Cores", 
                     lambda ax: plot_resource_usage(ax, baseline, target, ['metrics', 'component', 'etcd'], 'cpu', 'Cores'))
    plotter.add_plot("Etcd Memory Usage", "MB", 
                     lambda ax: plot_resource_usage(ax, baseline, target, ['metrics', 'component', 'etcd'], 'memory', 'MB', scale_factor=1.0/(1024*1024)))

    plotter.add_plot("Karmada Apiserver CPU Usage", "Cores", 
                     lambda ax: plot_resource_usage(ax, baseline, target, ['metrics', 'component', 'karmada_apiserver'], 'cpu', 'Cores'))
    plotter.add_plot("Karmada Apiserver Memory Usage", "MB", 
                     lambda ax: plot_resource_usage(ax, baseline, target, ['metrics', 'component', 'karmada_apiserver'], 'memory', 'MB', scale_factor=1.0/(1024*1024)))

    plotter.finish()
    print("Visualization completed.")

if __name__ == "__main__":
    main()
