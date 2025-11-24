#!/bin/bash

# IDAES Performance Metrics Monitor
# This script provides real-time monitoring of IDAES application metrics

METRICS_URL="http://localhost:8082"
PPROF_URL="http://localhost:6060"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}     IDAES Performance Metrics Monitor        ${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
}

print_section() {
    echo -e "${YELLOW}--- $1 ---${NC}"
}

get_metric_value() {
    local metric_name="$1"
    curl -s "$METRICS_URL/debug/vars" | grep -o "\"$metric_name\":[0-9.]*" | cut -d':' -f2 | tr -d '"'
}

get_json_value() {
    local key="$1"
    local json="$2"
    echo "$json" | grep -o "\"$key\":[0-9.]*" | cut -d':' -f2 | tr -d '"'
}

format_bytes() {
    local bytes=$1
    if [ -z "$bytes" ] || [ "$bytes" = "0" ]; then
        echo "0 B"
        return
    fi
    
    if [ "$bytes" -lt 1024 ]; then
        echo "${bytes} B"
    elif [ "$bytes" -lt 1048576 ]; then
        echo "$(($bytes / 1024)) KB"
    elif [ "$bytes" -lt 1073741824 ]; then
        echo "$(($bytes / 1048576)) MB"
    else
        echo "$(($bytes / 1073741824)) GB"
    fi
}

format_duration() {
    local seconds=$1
    if [ -z "$seconds" ] || [ "$seconds" = "0" ]; then
        echo "0s"
        return
    fi
    
    local days=$((seconds / 86400))
    local hours=$(((seconds % 86400) / 3600))
    local mins=$(((seconds % 3600) / 60))
    local secs=$((seconds % 60))
    
    if [ "$days" -gt 0 ]; then
        echo "${days}d ${hours}h ${mins}m ${secs}s"
    elif [ "$hours" -gt 0 ]; then
        echo "${hours}h ${mins}m ${secs}s"
    elif [ "$mins" -gt 0 ]; then
        echo "${mins}m ${secs}s"
    else
        echo "${secs}s"
    fi
}

check_service_health() {
    local service_name="$1"
    local url="$2"
    
    if curl -s -f "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $service_name"
    else
        echo -e "${RED}✗${NC} $service_name"
    fi
}

show_realtime_metrics() {
    while true; do
        clear
        print_header
        
        # Service Health Check
        print_section "Service Health"
        check_service_health "Main Application" "http://localhost:8081/api/v1/health"
        check_service_health "Metrics Server" "$METRICS_URL/health"
        check_service_health "pprof Server" "$PPROF_URL/debug/pprof/"
        echo ""
        
        # Get all metrics in one call
        metrics_json=$(curl -s "$METRICS_URL/debug/vars" 2>/dev/null)
        
        if [ -z "$metrics_json" ]; then
            echo -e "${RED}Unable to fetch metrics from $METRICS_URL${NC}"
            echo "Make sure the IDAES application is running with -enable_pprof=true"
            exit 1
        fi
        
        # Application Metrics
        print_section "Application Metrics"
        uptime=$(get_json_value "app.uptime_seconds" "$metrics_json")
        docs_uploaded=$(get_json_value "app.documents_uploaded_total" "$metrics_json")
        docs_processed=$(get_json_value "app.documents_processed_total" "$metrics_json")
        analysis_total=$(get_json_value "app.analysis_requests_total" "$metrics_json")
        analysis_active=$(get_json_value "app.analysis_requests_active" "$metrics_json")
        errors=$(get_json_value "app.errors_total" "$metrics_json")
        avg_processing=$(get_json_value "app.avg_processing_time_seconds" "$metrics_json")
        
        echo "Uptime: $(format_duration $uptime)"
        echo "Documents Uploaded: ${docs_uploaded:-0}"
        echo "Documents Processed: ${docs_processed:-0}"
        echo "Analysis Requests Total: ${analysis_total:-0}"
        echo "Analysis Requests Active: ${analysis_active:-0}"
        echo "Total Errors: ${errors:-0}"
        echo "Avg Processing Time: ${avg_processing:-0}s"
        echo ""
        
        # Runtime Metrics
        print_section "Runtime Metrics"
        goroutines=$(get_json_value "runtime.goroutines" "$metrics_json")
        heap_bytes=$(get_json_value "runtime.heap_bytes" "$metrics_json")
        
        echo "Active Goroutines: ${goroutines:-0}"
        echo "Heap Memory: $(format_bytes $heap_bytes)"
        echo ""
        
        # Go Runtime Stats
        print_section "Go Runtime (from cmdline)"
        gc_runs=$(get_json_value "memstats.NumGC" "$metrics_json")
        alloc=$(get_json_value "memstats.Alloc" "$metrics_json")
        sys=$(get_json_value "memstats.Sys" "$metrics_json")
        
        echo "GC Runs: ${gc_runs:-0}"
        echo "Current Alloc: $(format_bytes $alloc)"
        echo "System Memory: $(format_bytes $sys)"
        echo ""
        
        # Quick Performance Tips
        print_section "Performance Status"
        if [ ! -z "$goroutines" ] && [ "$goroutines" -gt 100 ]; then
            echo -e "${YELLOW}⚠ High goroutine count ($goroutines) - consider investigating${NC}"
        fi
        
        if [ ! -z "$heap_bytes" ] && [ "$heap_bytes" -gt 104857600 ]; then  # 100MB
            echo -e "${YELLOW}⚠ High heap usage ($(format_bytes $heap_bytes)) - monitor for memory leaks${NC}"
        fi
        
        if [ ! -z "$errors" ] && [ "$errors" -gt 0 ]; then
            echo -e "${RED}⚠ Application errors detected ($errors)${NC}"
        fi
        
        if [ ! -z "$analysis_active" ] && [ "$analysis_active" -gt 10 ]; then
            echo -e "${YELLOW}⚠ High active analysis requests ($analysis_active)${NC}"
        fi
        
        echo ""
        echo -e "${BLUE}Metrics URLs:${NC}"
        echo "  All metrics: $METRICS_URL/debug/vars"
        echo "  Health check: $METRICS_URL/health"
        echo "  pprof: $PPROF_URL/debug/pprof/"
        echo ""
        echo "Press Ctrl+C to exit, refreshing every 5 seconds..."
        
        sleep 5
    done
}

# Command-line options
case "${1:-realtime}" in
    "realtime"|"live"|"monitor")
        show_realtime_metrics
        ;;
    "once"|"snapshot")
        print_header
        show_realtime_metrics | head -50
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [realtime|once|help]"
        echo ""
        echo "Commands:"
        echo "  realtime  - Show live updating metrics (default)"
        echo "  once      - Show single snapshot of metrics"
        echo "  help      - Show this help message"
        echo ""
        echo "URLs:"
        echo "  Metrics: http://localhost:8082/debug/vars"
        echo "  Health:  http://localhost:8082/health"
        echo "  pprof:   http://localhost:6060/debug/pprof/"
        ;;
    *)
        echo "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac