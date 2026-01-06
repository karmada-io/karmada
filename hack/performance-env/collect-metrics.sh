#!/usr/bin/env bash
# Copyright 2026 The Karmada Authors.
#
# Karmada performance metrics collection script
# Function: Collect performance metrics of Karmada system components
# Including: controller-manager controller, workqueue, REST client, etc.

set -o errexit
set -o nounset
set -o pipefail

# configuration parameters
PROMETHEUS_ENDPOINT="${PROMETHEUS_ENDPOINT:-http://localhost:31801}"
OUTPUT_FILE="${OUTPUT_FILE:-./karmada-metrics.json}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# make sure the output directory exists
mkdir -p "$(dirname "${OUTPUT_FILE}")"

# Time range configuration for metrics collection
# Default: collect data for the last 5 minutes with 15s step
END_TIME="${END_TIME:-$(date +%s)}"
START_TIME="${START_TIME:-$(($END_TIME - 300))}"
STEP="${STEP:-15s}"
# Rate/range window for rate() and *_over_time() in PromQL (default 2m)
RATE_INTERVAL="${RATE_INTERVAL:-2m}"

# ============================================
# General query function
# Function: Query Prometheus metrics range, return array of [timestamp, value]
# Parameters: $1 - PromQL query statement
# Return: JSON array of values, e.g. [[1678000000,"10"],[1678000015,"20"]], if failed return []
# ============================================
function query_prometheus() {
    local query="$1"

    # execute query_range and extract the values array of the first result
    # use -c for compact output
    local result=$(curl -s -G "${PROMETHEUS_ENDPOINT}/api/v1/query_range" \
        --data-urlencode "query=${query}" \
        --data-urlencode "start=${START_TIME}" \
        --data-urlencode "end=${END_TIME}" \
        --data-urlencode "step=${STEP}" 2>/dev/null | \
        jq -c '.data.result[0].values // []' 2>/dev/null)

    # handle empty result
    if [ -z "${result}" ] || [ "${result}" == "null" ]; then
        result="[]"
    fi

    echo "${result}"
}

# ============================================
# General grouped metrics collection function
# Function: Collect metrics by specified label, support dynamic discovery of label values
# Parameters:
#   $1 - count metric name (used to discover active instances, also used as metric name)
#   $2 - label name (used to group)
#   $3 - metric collection function name (function to handle single instance)
# Return: JSON formatted metric data
# ============================================
function collect_grouped_metrics() {
    local metric_name="$1"
    local label_name="$2"
    local collect_func="$3"
    local exclude_pattern="${4:-}"

    # get all active instances at END_TIME
    local instances=$(curl -s -G "${PROMETHEUS_ENDPOINT}/api/v1/query" \
        --data-urlencode "query=count by (${label_name}) (${metric_name})" \
        --data-urlencode "time=${END_TIME}" 2>/dev/null | \
        jq -r ".data.result[].metric.${label_name}" 2>/dev/null | sort -u)

    if [ -z "${instances}" ]; then
        echo "⚠️  Warning: No ${metric_name} metrics found" >&2
        instances=""
    fi

    # collect metrics for each instance
    local by_instance_json=""
    local first=true
    for instance in ${instances}; do
        # Apply exclude pattern if provided
        if [ -n "${exclude_pattern}" ] && [[ "${instance}" =~ ${exclude_pattern} ]]; then
            continue
        fi

        if [ "${first}" = true ]; then
            first=false
        else
            by_instance_json+=","
        fi
        # call the specified metric collection function
        by_instance_json+=$(${collect_func} "${instance}")
    done

    # if no instances are found, set empty object
    if [ -z "${by_instance_json}" ]; then
        by_instance_json='"_no_instances": {}'
    fi

    cat <<EOF
{
    ${by_instance_json}
}
EOF
}

function collect_policy_match_metrics() {
    # P50 latency
    local match_p50=$(query_prometheus \
        'histogram_quantile(0.50, rate(resource_match_policy_duration_seconds_bucket['"${RATE_INTERVAL}"']))')
    
    # P90 latency
    local match_p90=$(query_prometheus \
        'histogram_quantile(0.90, rate(resource_match_policy_duration_seconds_bucket['"${RATE_INTERVAL}"']))')
    
    # P99 latency
    local match_p99=$(query_prometheus \
        'histogram_quantile(0.99, rate(resource_match_policy_duration_seconds_bucket['"${RATE_INTERVAL}"']))')

    # matching rate (per second)
    local match_rate=$(query_prometheus \
        'rate(resource_match_policy_duration_seconds_count['"${RATE_INTERVAL}"'])')
    
    # total matching count (aggregate all instances)
    local match_total=$(query_prometheus \
        'sum(resource_match_policy_duration_seconds_count)')
    
    cat <<EOF
{
    "policy_match": {
        "latency": {
            "p50_seconds": ${match_p50},
            "p90_seconds": ${match_p90},
            "p99_seconds": ${match_p99}
        },
        "throughput": {
            "rate_per_second": ${match_rate},
            "total_count": ${match_total}
        }
    }
}
EOF
}

function collect_policy_apply_metrics() {
    # successful P50 latency
    local apply_success_p50=$(query_prometheus \
        'histogram_quantile(0.50, rate(resource_apply_policy_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P90 latency
    local apply_success_p90=$(query_prometheus \
        'histogram_quantile(0.90, rate(resource_apply_policy_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P99 latency
    local apply_success_p99=$(query_prometheus \
        'histogram_quantile(0.99, rate(resource_apply_policy_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')

    # successful rate
    local apply_success_rate=$(query_prometheus \
        'rate(resource_apply_policy_duration_seconds_count{result="success"}['"${RATE_INTERVAL}"'])')
    
    # failed rate
    local apply_error_rate=$(query_prometheus \
        'rate(resource_apply_policy_duration_seconds_count{result="error"}['"${RATE_INTERVAL}"'])')
    
    # total apply count (success, aggregate all instances)
    local apply_success_total=$(query_prometheus \
        'sum(resource_apply_policy_duration_seconds_count{result="success"})')
    
    # total apply count (failed, aggregate all instances)
    local apply_error_total=$(query_prometheus \
        'sum(resource_apply_policy_duration_seconds_count{result="error"})')
    
    cat <<EOF
{
    "policy_apply": {
        "success": {
            "latency": {
                "p50_seconds": ${apply_success_p50},
                "p90_seconds": ${apply_success_p90},
                "p99_seconds": ${apply_success_p99}
            },
            "throughput": {
                "rate_per_second": ${apply_success_rate},
                "total_count": ${apply_success_total}
            }
        },
        "error": {
            "throughput": {
                "rate_per_second": ${apply_error_rate},
                "total_count": ${apply_error_total}
            }
        }
    }
}
EOF
}

function collect_binding_sync_metrics() {
    # successful P50 latency
    local binding_p50=$(query_prometheus \
        'histogram_quantile(0.50, rate(binding_sync_work_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P90 latency
    local binding_p90=$(query_prometheus \
        'histogram_quantile(0.90, rate(binding_sync_work_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P99 latency
    local binding_p99=$(query_prometheus \
        'histogram_quantile(0.99, rate(binding_sync_work_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')

    # successful rate
    local binding_success_rate=$(query_prometheus \
        'rate(binding_sync_work_duration_seconds_count{result="success"}['"${RATE_INTERVAL}"'])')
    
    # failed rate
    local binding_error_rate=$(query_prometheus \
        'rate(binding_sync_work_duration_seconds_count{result="error"}['"${RATE_INTERVAL}"'])')
    
    # successful total count
    local binding_success_total=$(query_prometheus \
        'sum(binding_sync_work_duration_seconds_count{result="success"})')
    # failed total count
    local binding_error_total=$(query_prometheus \
        'sum(binding_sync_work_duration_seconds_count{result="error"})')
    
    cat <<EOF
{
    "binding_sync": {
        "success": {
            "latency": {
                "p50_seconds": ${binding_p50},
                "p90_seconds": ${binding_p90},
                "p99_seconds": ${binding_p99}
            },
            "throughput": {
                "rate_per_second": ${binding_success_rate},
                "total_count": ${binding_success_total}
            }
        },
        "error": {
            "throughput": {
                "rate_per_second": ${binding_error_rate},
                "total_count": ${binding_error_total}
            }
        }
    }
}
EOF
}

function collect_work_sync_metrics() {
    # successful P50 latency
    local work_p50=$(query_prometheus \
        'histogram_quantile(0.50, rate(work_sync_workload_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P90 latency
    local work_p90=$(query_prometheus \
        'histogram_quantile(0.90, rate(work_sync_workload_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')
    
    # successful P99 latency
    local work_p99=$(query_prometheus \
        'histogram_quantile(0.99, rate(work_sync_workload_duration_seconds_bucket{result="success"}['"${RATE_INTERVAL}"']))')

    # successful rate
    local work_success_rate=$(query_prometheus \
        'rate(work_sync_workload_duration_seconds_count{result="success"}['"${RATE_INTERVAL}"'])')
    
    # failed rate
    local work_error_rate=$(query_prometheus \
        'rate(work_sync_workload_duration_seconds_count{result="error"}['"${RATE_INTERVAL}"'])')
    
    local work_success_total=$(query_prometheus \
        'sum(work_sync_workload_duration_seconds_count{result="success"})')
    
    local work_error_total=$(query_prometheus \
        'sum(work_sync_workload_duration_seconds_count{result="error"})')
    
    cat <<EOF
{
    "work_sync": {
        "success": {
            "latency": {
                "p50_seconds": ${work_p50},
                "p90_seconds": ${work_p90},
                "p99_seconds": ${work_p99}
            },
            "throughput": {
                "rate_per_second": ${work_success_rate},
                "total_count": ${work_success_total}
            }
        },
        "error": {
            "throughput": {
                "rate_per_second": ${work_error_rate},
                "total_count": ${work_error_total}
            }
        }
    }
}
EOF
}

function collect_cluster_sync_metrics() {
    # P50 latency
    local cluster_p50=$(query_prometheus \
        'histogram_quantile(0.50, sum(rate(cluster_sync_status_duration_seconds_bucket['"${RATE_INTERVAL}"'])) by (le))')
    
    # P90 latency
    local cluster_p90=$(query_prometheus \
        'histogram_quantile(0.90, sum(rate(cluster_sync_status_duration_seconds_bucket['"${RATE_INTERVAL}"'])) by (le))')
    
    # P99 latency
    local cluster_p99=$(query_prometheus \
        'histogram_quantile(0.99, sum(rate(cluster_sync_status_duration_seconds_bucket['"${RATE_INTERVAL}"'])) by (le))')

    # sync rate
    local cluster_rate=$(query_prometheus \
        'sum(rate(cluster_sync_status_duration_seconds_count['"${RATE_INTERVAL}"']))')
    
    # total sync count
    local cluster_total=$(query_prometheus \
        'sum(cluster_sync_status_duration_seconds_count)')
    
    cat <<EOF
{
    "cluster_sync": {
        "latency": {
            "p50_seconds": ${cluster_p50},
            "p90_seconds": ${cluster_p90},
            "p99_seconds": ${cluster_p99}
        },
        "throughput": {
            "rate_per_second": ${cluster_rate},
            "total_count": ${cluster_total}
        }
    }
}
EOF
}

function collect_single_controller_metrics() {
    local controller_name="$1"
    
    # Reconciliation P50 latency
    local runtime_p50=$(query_prometheus \
        "histogram_quantile(0.50, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller=\"${controller_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # Reconciliation P90 latency
    local runtime_p90=$(query_prometheus \
        "histogram_quantile(0.90, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller=\"${controller_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # Reconciliation P99 latency
    local runtime_p99=$(query_prometheus \
        "histogram_quantile(0.99, sum(rate(controller_runtime_reconcile_time_seconds_bucket{controller=\"${controller_name}\"}['"${RATE_INTERVAL}"'])) by (le))")

    # Reconciliation total rate
    local reconcile_rate=$(query_prometheus \
        "sum(rate(controller_runtime_reconcile_total{controller=\"${controller_name}\"}['"${RATE_INTERVAL}"']))")
    
    # Reconciliation total count
    local reconcile_total=$(query_prometheus \
        "sum(controller_runtime_reconcile_total{controller=\"${controller_name}\"})")
    
    # failed total count
    local error_total=$(query_prometheus \
        "sum(controller_runtime_reconcile_errors_total{controller=\"${controller_name}\"})")
    
    cat <<EOF
    "${controller_name}": {
        "reconciliation": {
            "latency": {
                "p50_seconds": ${runtime_p50},
                "p90_seconds": ${runtime_p90},
                "p99_seconds": ${runtime_p99}
            },
            "throughput": {
                "total_rate_per_second": ${reconcile_rate},
                "total_count": ${reconcile_total},
                "error_count": ${error_total}
            }
        }
    }
EOF
}

function collect_controller_runtime_metrics() {
    local controller_names=(dependencies-distributor binding-controller resource-binding-status-controller execution-controller work-status-controller)
    local first=true
    local by_controller_json=""

    for controller in "${controller_names[@]}"; do
        if [ "${first}" = true ]; then
            first=false
        else
            by_controller_json+=","
        fi

        by_controller_json+=$(collect_single_controller_metrics "${controller}")
    done

    local result=$(cat <<EOF
{
    ${by_controller_json}
}
EOF
)
    cat <<EOF
{
    "controller_runtime": ${result}
}
EOF
}

function collect_scheduler_metrics() {
    # successful scheduling (result="scheduled", aggregate all schedule_type)
    local e2e_scheduled_p50=$(query_prometheus \
        'histogram_quantile(0.50, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="scheduled"}['"${RATE_INTERVAL}"'])) by (le))')
    local e2e_scheduled_p90=$(query_prometheus \
        'histogram_quantile(0.90, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="scheduled"}['"${RATE_INTERVAL}"'])) by (le))')
    local e2e_scheduled_p99=$(query_prometheus \
        'histogram_quantile(0.99, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="scheduled"}['"${RATE_INTERVAL}"'])) by (le))')

    # scheduling failed (result="error", aggregate all schedule_type)
    local e2e_error_p50=$(query_prometheus \
        'histogram_quantile(0.50, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="error"}['"${RATE_INTERVAL}"'])) by (le))')
    local e2e_error_p90=$(query_prometheus \
        'histogram_quantile(0.90, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="error"}['"${RATE_INTERVAL}"'])) by (le))')
    local e2e_error_p99=$(query_prometheus \
        'histogram_quantile(0.99, sum(rate(karmada_scheduler_e2e_scheduling_duration_seconds_bucket{result="error"}['"${RATE_INTERVAL}"'])) by (le))')

    # queue incoming binding rate
    local queue_rate=$(query_prometheus \
        'sum(rate(karmada_scheduler_queue_incoming_bindings_total['"${RATE_INTERVAL}"']))')
    local queue_total=$(query_prometheus \
        'sum(karmada_scheduler_queue_incoming_bindings_total)')

    # scheduling attempt rate
    local attempts_rate=$(query_prometheus \
        'sum(rate(karmada_scheduler_schedule_attempts_total['"${RATE_INTERVAL}"']))')
    local attempts_total=$(query_prometheus \
        'sum(karmada_scheduler_schedule_attempts_total)')

    cat <<EOF
{
    "scheduler": {
        "e2e_scheduling": {
            "scheduled": {
                "latency": {
                    "p50_seconds": ${e2e_scheduled_p50},
                    "p90_seconds": ${e2e_scheduled_p90},
                    "p99_seconds": ${e2e_scheduled_p99}
                }
            },
            "error": {
                "latency": {
                    "p50_seconds": ${e2e_error_p50},
                    "p90_seconds": ${e2e_error_p90},
                    "p99_seconds": ${e2e_error_p99}
                }
            }
        },
        "queue": {
            "incoming_bindings": {
                "rate_per_second": ${queue_rate},
                "total_count": ${queue_total}
            }
        },
        "scheduling_attempts": {
            "throughput": {
                "rate_per_second": ${attempts_rate},
                "total_count": ${attempts_total}
            }
        }
    }
}
EOF
}

function collect_single_workqueue_metrics() {
    local queue_name="$1"
    
    # queue latency P50 (the time the project waits in the queue)
    local queue_p50=$(query_prometheus \
        "histogram_quantile(0.50, sum(rate(workqueue_queue_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # queue latency P90
    local queue_p90=$(query_prometheus \
        "histogram_quantile(0.90, sum(rate(workqueue_queue_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # queue latency P99
    local queue_p99=$(query_prometheus \
        "histogram_quantile(0.99, sum(rate(workqueue_queue_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")

    # processing latency P50 (the time to process a single project)
    local work_p50=$(query_prometheus \
        "histogram_quantile(0.50, sum(rate(workqueue_work_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # processing latency P90
    local work_p90=$(query_prometheus \
        "histogram_quantile(0.90, sum(rate(workqueue_work_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")
    
    # processing latency P99
    local work_p99=$(query_prometheus \
        "histogram_quantile(0.99, sum(rate(workqueue_work_duration_seconds_bucket{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])) by (le))")

    # maximum queue depth (last 5 minutes)
    local depth_max=$(query_prometheus \
        "max_over_time(workqueue_depth{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])")
    
    # average queue depth (last 5 minutes)
    local depth_avg=$(query_prometheus \
        "avg_over_time(workqueue_depth{name=\"${queue_name}\"}['"${RATE_INTERVAL}"'])")
    
    # add rate (throughput)
    local adds_rate=$(query_prometheus \
        "sum(rate(workqueue_adds_total{name=\"${queue_name}\"}['"${RATE_INTERVAL}"']))")
    
    # total add count
    local adds_total=$(query_prometheus \
        "sum(workqueue_adds_total{name=\"${queue_name}\"})")
    
    # total retry count
    local retries_total=$(query_prometheus \
        "sum(workqueue_retries_total{name=\"${queue_name}\"})")
    
    cat <<EOF
    "${queue_name}": {
        "queue_latency": {
            "p50_seconds": ${queue_p50},
            "p90_seconds": ${queue_p90},
            "p99_seconds": ${queue_p99}
        },
        "work_latency": {
            "p50_seconds": ${work_p50},
            "p90_seconds": ${work_p90},
            "p99_seconds": ${work_p99}
        },
        "depth": {
            "max": ${depth_max},
            "avg": ${depth_avg}
        },
        "throughput": {
            "adds_per_second": ${adds_rate},
            "total_adds": ${adds_total},
            "total_retries": ${retries_total}
        }
    }
EOF
}

function collect_workqueue_metrics() {
    local queue_names=("resource detector" "propagationPolicy reconciler" dependencies-distributor binding-controller resource-binding-status-controller execution-controller work-status work-status-controller)
    local first=true
    local by_queue_json=""

    for queue_name in "${queue_names[@]}"; do
        if [ "${first}" = true ]; then
            first=false
        else
            by_queue_json+=","$'\n'
        fi

        by_queue_json+=$(collect_single_workqueue_metrics "${queue_name}")
    done

    if [ -z "${by_queue_json}" ]; then
        by_queue_json='"_no_queues": {}'
    fi

    local result=$(cat <<EOF
{
${by_queue_json}
}
EOF
)
    cat <<EOF
{
    "workqueue": ${result}
}
EOF
}

function collect_single_rest_client_metrics() {
    local host_name="$1"
    
    # total request rate (all status codes)
    local total_rate=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=\"${host_name}\"}['"${RATE_INTERVAL}"']))")
    
    # total request count
    local total_count=$(query_prometheus \
        "sum(rest_client_requests_total{host=\"${host_name}\"})")
    
    # 2xx request rate (success)
    local rate_2xx=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=\"${host_name}\", code=~\"2..\"}['"${RATE_INTERVAL}"']))")
    
    # 4xx request rate (client error)
    local rate_4xx=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=\"${host_name}\", code=~\"4..\"}['"${RATE_INTERVAL}"']))")
    
    # 2xx total request count
    local count_2xx=$(query_prometheus \
        "sum(rest_client_requests_total{host=\"${host_name}\", code=~\"2..\"})")
    
    # 4xx total request count
    local count_4xx=$(query_prometheus \
        "sum(rest_client_requests_total{host=\"${host_name}\", code=~\"4..\"})")
    
    cat <<EOF
    "${host_name}": {
        "total": {
            "rate_per_second": ${total_rate},
            "count": ${total_count}
        },
        "2xx": {
            "rate_per_second": ${rate_2xx},
            "count": ${count_2xx}
        },
        "4xx": {
            "rate_per_second": ${rate_4xx},
            "count": ${count_4xx}
        }
    }
EOF
}

function collect_rest_client_metrics_by_pattern() {
    local key_name="$1"
    local pattern="$2"

    # total request rate (all status codes)
    local total_rate=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=~\"${pattern}\"}['"${RATE_INTERVAL}"']))")

    # total request count
    local total_count=$(query_prometheus \
        "sum(rest_client_requests_total{host=~\"${pattern}\"})")

    # 2xx request rate (success)
    local rate_2xx=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=~\"${pattern}\", code=~\"2..\"}['"${RATE_INTERVAL}"']))")

    # 4xx request rate (client error)
    local rate_4xx=$(query_prometheus \
        "sum(rate(rest_client_requests_total{host=~\"${pattern}\", code=~\"4..\"}['"${RATE_INTERVAL}"']))")

    # 2xx total request count
    local count_2xx=$(query_prometheus \
        "sum(rest_client_requests_total{host=~\"${pattern}\", code=~\"2..\"})")

    # 4xx total request count
    local count_4xx=$(query_prometheus \
        "sum(rest_client_requests_total{host=~\"${pattern}\", code=~\"4..\"})")

    cat <<EOF
    "${key_name}": {
        "total": {
            "rate_per_second": ${total_rate},
            "count": ${total_count}
        },
        "2xx": {
            "rate_per_second": ${rate_2xx},
            "count": ${count_2xx}
        },
        "4xx": {
            "rate_per_second": ${rate_4xx},
            "count": ${count_4xx}
        }
    }
EOF
}

function collect_rest_client_metrics() {
    local ipv4_port_pattern="[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+:[0-9]+"
    local ipv6_pattern="\[.*\]"

    # 1. Get individual metrics (excluding member clusters and ipv6)
    local exclude="${ipv4_port_pattern}|${ipv6_pattern}"
    local individual_json=$(collect_grouped_metrics "rest_client_requests_total" "host" "collect_single_rest_client_metrics" "${exclude}")

    # 2. Get aggregated metrics for member clusters
    local aggregated_json=$(collect_rest_client_metrics_by_pattern "member_clusters_aggregated" "${ipv4_port_pattern}")

    # Wrap aggregated_json in {} to make it a valid object for jq
    local aggregated_obj="{ ${aggregated_json} }"

    # 3. Merge JSONs
    local merged_json=$(echo "${individual_json} ${aggregated_obj}" | jq -s '.[0] + .[1]')

    cat <<EOF
{
    "rest_client": ${merged_json}
}
EOF
}

function collect_single_component_resources() {
    local component_name="$1"
    local namespace="${2:-karmada-system}"
    local container_name="${3:-$component_name}"

    # CPU usage (cores/second)
    local cpu_rate=$(query_prometheus \
        "sum(rate(container_cpu_usage_seconds_total{namespace=\"${namespace}\", pod=~\"${component_name}-.+\", container=\"${container_name}\"}['"${RATE_INTERVAL}"']))")

    # memory working set usage (bytes)
    local memory_bytes=$(query_prometheus \
        "sum(container_memory_working_set_bytes{namespace=\"${namespace}\", pod=~\"${component_name}-.+\", container=\"${container_name}\"})")

    cat <<EOF
{
    "cpu": {
        "usage_cores_per_second": ${cpu_rate}
    },
    "memory": {
        "working_set_bytes": ${memory_bytes}
    }
}
EOF
}

function collect_component_metrics() {
    local etcd_metrics=$(collect_single_component_resources "etcd" "karmada-system" "etcd")
    local apiserver_metrics=$(collect_single_component_resources "karmada-apiserver" "karmada-system" "karmada-apiserver")

    cat <<EOF
{
    "component": {
        "etcd": ${etcd_metrics},
        "karmada_apiserver": ${apiserver_metrics}
    }
}
EOF
}

function main() {
    echo "start to collect Karmada performance metrics..."
    
    # check Prometheus connection
    if ! curl -s "${PROMETHEUS_ENDPOINT}/-/healthy" > /dev/null 2>&1; then
        echo "❌ Error: Unable to connect to Prometheus: ${PROMETHEUS_ENDPOINT}"
        echo "Please ensure Prometheus is running and accessible"
        kubectl --context="${HOST_CLUSTER_NAME}" get pod -n monitor
        kubectl --context="${HOST_CLUSTER_NAME}" get pod -n karmada-system
        exit 1
    fi
    echo "✅ Prometheus connection is successful"
    
    echo "start to collect policy match metrics..."
    local match_metrics=$(collect_policy_match_metrics)

    echo "start to collect policy apply metrics..."
    local apply_metrics=$(collect_policy_apply_metrics)

    echo "start to collect binding sync metrics..."
    local binding_metrics=$(collect_binding_sync_metrics)

    echo "start to collect work sync metrics..."
    local work_metrics=$(collect_work_sync_metrics)

    echo "start to collect cluster sync metrics..."
    local cluster_metrics=$(collect_cluster_sync_metrics)

    echo "start to collect controller runtime metrics..."
    local runtime_metrics=$(collect_controller_runtime_metrics)

    echo "start to collect scheduler metrics..."
    local scheduler_metrics=$(collect_scheduler_metrics)

    echo "start to collect workqueue metrics..."
    local workqueue_metrics=$(collect_workqueue_metrics)

    echo "start to collect rest client metrics..."
    local rest_client_metrics=$(collect_rest_client_metrics)

    echo "start to collect component metrics..."
    local component_metrics=$(collect_component_metrics)    

    export TIMESTAMP
    export PROMETHEUS_ENDPOINT
    export START_TIME
    export END_TIME
    export STEP
    export RATE_INTERVAL
    
    # combine into a complete JSON (using jq to ensure correct format)
    echo "${match_metrics}" "${apply_metrics}" "${binding_metrics}" "${work_metrics}" "${cluster_metrics}" "${runtime_metrics}" "${scheduler_metrics}" "${workqueue_metrics}" "${rest_client_metrics}" "${component_metrics}" | \
    jq -s --arg ts "${TIMESTAMP}" --arg endpoint "${PROMETHEUS_ENDPOINT}" \
          --arg start "${START_TIME}" --arg end_time "${END_TIME}" --arg step "${STEP}" '{
        metadata: {
            timestamp: $ts,
            collection_time: (now | todate),
            prometheus_endpoint: $endpoint,
            query_range: {
                start: $start,
                "end": $end_time,
                step: $step
            },
            controllers: ["detector", "binding", "work", "cluster"],
            includes_controller_runtime: true,
            includes_scheduler: true,
            includes_workqueue: true,
            includes_rest_client: true,
            includes_component: true
        },
        metrics: {
            policy_match: .[0].policy_match,
            policy_apply: .[1].policy_apply,
            binding_sync: .[2].binding_sync,
            work_sync: .[3].work_sync,
            cluster_sync: .[4].cluster_sync,
            controller_runtime: .[5].controller_runtime,
            scheduler: .[6].scheduler,
            workqueue: .[7].workqueue,
            rest_client: .[8].rest_client,
            component: .[9].component
        }
    }' > "${OUTPUT_FILE}"
    
    echo "✅ Karmada performance metrics collection is completed"
    echo "output file: ${OUTPUT_FILE}"

    echo "✅ print pod status"
    kubectl --context="${HOST_CLUSTER_NAME}" get pod -n monitor
    kubectl --context="${HOST_CLUSTER_NAME}" get pod -n karmada-system
}

main
