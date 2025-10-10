#!/bin/bash

# HTTP Benchmarking Script for Octo Framework using Apache Bench (ab)
# This script provides comprehensive performance metrics

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Default values
HOST="localhost"
PORT="8056"
TOTAL_REQUESTS=10000
CONCURRENCY=100
WARMUP_REQUESTS=1000
WARMUP_CONCURRENCY=10
AUTO_START_SERVER=true
SERVER_PID=""

# Results directory
RESULTS_DIR="bench_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
SESSION_DIR="$RESULTS_DIR/$TIMESTAMP"

# Function to print colored output
print_header() {
    echo -e "\n${BLUE}${BOLD}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}${BOLD}$1${NC}"
    echo -e "${BLUE}${BOLD}═══════════════════════════════════════════════════════════════${NC}\n"
}

print_subheader() {
    echo -e "\n${YELLOW}${BOLD}▶ $1${NC}"
    echo -e "${YELLOW}───────────────────────────────────────────${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ $1${NC}"
}

# Function to check if ab is installed
check_dependencies() {
    if ! command -v ab &> /dev/null; then
        print_error "Apache Bench (ab) is not installed!"
        echo "Install it with:"
        echo "  Ubuntu/Debian: sudo apt-get install apache2-utils"
        echo "  MacOS: ab comes pre-installed"
        echo "  RHEL/CentOS: sudo yum install httpd-tools"
        exit 1
    fi
    print_success "Apache Bench (ab) found"
}

# Function to start test server
start_test_server() {
    if [ "$AUTO_START_SERVER" = true ]; then
        print_info "Starting test server..."
        
        # Check if testserver exists
        if [ ! -f "cmd/testserver/main.go" ]; then
            print_error "Test server not found at cmd/testserver/main.go"
            exit 1
        fi
        
        # Start server in background with PORT env var
        cd cmd/testserver && PORT=$PORT go run main.go &
        SERVER_PID=$!
        cd ../..
        
        # Wait for server to start
        local max_attempts=30
        local attempt=0
        while [ $attempt -lt $max_attempts ]; do
            if curl -s -o /dev/null "http://$HOST:$PORT/" 2>/dev/null; then
                print_success "Test server started (PID: $SERVER_PID)"
                
                # Verify server is returning JSON
                print_info "Verifying server responses..."
                local test_response=$(curl -s "http://$HOST:$PORT/" 2>/dev/null)
                if echo "$test_response" | grep -q '"success"'; then
                    print_success "Server is returning valid JSON responses"
                    
                    # Show sample response
                    echo "Sample response from /:"
                    echo "$test_response" | jq . 2>/dev/null || echo "$test_response"
                    echo ""
                    return 0
                else
                    print_error "Server is not returning expected JSON format"
                    echo "Got: $test_response"
                    kill $SERVER_PID 2>/dev/null || true
                    exit 1
                fi
            fi
            attempt=$((attempt + 1))
            sleep 0.5
        done
        
        print_error "Failed to start test server"
        exit 1
    fi
}

# Function to check if server is running
check_server() {
    if ! curl -s -o /dev/null "http://$HOST:$PORT/"; then
        if [ "$AUTO_START_SERVER" = true ]; then
            start_test_server
        else
            print_error "Server is not running at http://$HOST:$PORT/"
            echo "Please start your Octo server first or use --auto-start"
            exit 1
        fi
    else
        print_success "Server is responding at http://$HOST:$PORT/"
    fi
}

# Cleanup function
cleanup() {
    if [ ! -z "$SERVER_PID" ] && [ "$AUTO_START_SERVER" = true ]; then
        print_info "Stopping test server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        print_success "Test server stopped"
    fi
}

# Set up trap to ensure cleanup on exit
trap cleanup EXIT INT TERM

# Function to create results directory
setup_results_dir() {
    mkdir -p "$SESSION_DIR"
    print_success "Results directory created: $SESSION_DIR"
}

# Function to run warmup
run_warmup() {
    print_info "Running warmup ($WARMUP_REQUESTS requests, $WARMUP_CONCURRENCY concurrent)..."
    ab -n $WARMUP_REQUESTS -c $WARMUP_CONCURRENCY "http://$HOST:$PORT/" > /dev/null 2>&1
    print_success "Warmup completed"
    sleep 2
}

# Function to run benchmark and parse results
run_benchmark() {
    local path=$1
    local name=$2
    local desc=$3
    local output_file="$SESSION_DIR/${name}.txt"
    
    print_subheader "$desc"
    echo "Path: $path"
    echo "Requests: $TOTAL_REQUESTS, Concurrency: $CONCURRENCY"
    echo
    
    # Run ab and save output
    ab -n $TOTAL_REQUESTS -c $CONCURRENCY -g "$SESSION_DIR/${name}_gnuplot.tsv" "http://$HOST:$PORT$path" > "$output_file" 2>&1
    
    # Parse and display key metrics
    if [ -f "$output_file" ]; then
        # Extract key metrics
        local requests_per_sec=$(grep "Requests per second" "$output_file" | awk '{print $4}')
        local time_per_request=$(grep "Time per request.*mean" "$output_file" | head -1 | awk '{print $4}')
        local time_per_request_all=$(grep "Time per request.*across" "$output_file" | awk '{print $4}')
        local transfer_rate=$(grep "Transfer rate" "$output_file" | awk '{print $3}')
        local min_time=$(grep "Total:" "$output_file" | awk '{print $2}')
        local mean_time=$(grep "Total:" "$output_file" | awk '{print $3}')
        local median_time=$(grep "Total:" "$output_file" | awk '{print $4}')
        local max_time=$(grep "Total:" "$output_file" | awk '{print $5}')
        
        # Connection times
        local connect_mean=$(grep "Connect:" "$output_file" | awk '{print $3}')
        local processing_mean=$(grep "Processing:" "$output_file" | awk '{print $3}')
        local waiting_mean=$(grep "Waiting:" "$output_file" | awk '{print $3}')
        
        # Display results with proper color formatting
        echo -e "${GREEN}Performance Metrics:${NC}"
        echo -e "├─ Requests/sec:     ${BOLD}$requests_per_sec${NC} req/s"
        echo -e "├─ Time/request:     ${BOLD}$time_per_request${NC} ms (mean)"
        echo -e "├─ Time/request:     ${BOLD}$time_per_request_all${NC} ms (mean, across all concurrent requests)"
        echo -e "├─ Transfer rate:    ${BOLD}$transfer_rate${NC} KB/s"
        echo ""
        echo -e "${GREEN}Response Times (ms):${NC}"
        echo -e "├─ Min:              ${BOLD}$min_time${NC}"
        echo -e "├─ Mean:             ${BOLD}$mean_time${NC}"
        echo -e "├─ Median:           ${BOLD}$median_time${NC}"
        echo -e "└─ Max:              ${BOLD}$max_time${NC}"
        echo ""
        echo -e "${GREEN}Connection Times (ms):${NC}"
        echo -e "├─ Connect:          ${BOLD}$connect_mean${NC}"
        echo -e "├─ Processing:       ${BOLD}$processing_mean${NC}"
        echo -e "└─ Waiting:          ${BOLD}$waiting_mean${NC}"
        
        # Extract percentiles
        echo ""
        echo -e "${GREEN}Percentiles (ms):${NC}"
        grep "%" "$output_file" | head -5 | while read line; do
            echo "├─ $line"
        done
        
        # Check for actual errors (not content length variations)
        local non_2xx=$(grep "Non-2xx responses" "$output_file" | awk '{print $3}')
        
        if [ ! -z "$non_2xx" ] && [ "$non_2xx" != "0" ]; then
            echo ""
            print_error "Non-2xx responses: $non_2xx"
        fi
        
        # Note: We ignore "Failed requests" because ab reports content length 
        # variations as failures, which is normal for dynamic content with UUIDs
        
        # Save summary (without failed count since it's meaningless)
        echo "$name|$path|$requests_per_sec|$time_per_request|$transfer_rate" >> "$SESSION_DIR/summary.csv"
    else
        print_error "Benchmark failed for $name"
    fi
    
    echo ""
    sleep 1
}

# Function to generate summary report
generate_summary() {
    print_header "BENCHMARK SUMMARY"
    
    echo -e "${CYAN}Test Configuration:${NC}"
    echo "├─ Host:             $HOST:$PORT"
    echo "├─ Total Requests:   $TOTAL_REQUESTS"
    echo "├─ Concurrency:      $CONCURRENCY"
    echo "└─ Timestamp:        $TIMESTAMP"
    echo ""
    
    if [ -f "$SESSION_DIR/summary.csv" ]; then
        echo -e "${CYAN}Results Summary:${NC}"
        echo ""
        printf "%-25s %-30s %-12s %-12s %-12s\n" "Test Name" "Path" "Req/s" "Time/req(ms)" "Transfer(KB/s)"
        echo "──────────────────────────────────────────────────────────────────────────────────────────────"
        
        while IFS='|' read -r name path rps tpr tr; do
            if [ ! -z "$rps" ]; then
                printf "%-25s %-30s ${BOLD}%-12s${NC} %-12s %-12s\n" "$name" "$path" "$rps" "$tpr" "$tr"
            fi
        done < "$SESSION_DIR/summary.csv"
        
        echo ""
        print_success "Full results saved in: $SESSION_DIR"
        echo ""
        echo "View detailed results:"
        echo "  cat $SESSION_DIR/<test_name>.txt"
        echo ""
        echo "Gnuplot data available in:"
        echo "  $SESSION_DIR/*_gnuplot.tsv"
    fi
}

# Function to run all benchmarks
run_all_benchmarks() {
    print_header "OCTO FRAMEWORK HTTP BENCHMARK"
    
    print_subheader "Testing Static Routes"
    run_benchmark "/" "home_json" "Home (JSON response)"
    run_benchmark "/health" "health_check" "Health Check Endpoint"
    
    print_subheader "Testing List Endpoints"
    run_benchmark "/api/v1/users" "users_list" "Users List (JSON array)"
    run_benchmark "/api/v1/posts/1/comments" "comments_list" "Comments List (JSON array)"
    
    print_subheader "Testing Single Parameter Routes"
    run_benchmark "/api/v1/users/123" "user_single" "Single User (JSON object)"
    run_benchmark "/api/v1/posts/456" "post_single" "Single Post (JSON object)"
    
    print_subheader "Testing Multiple Parameter Routes"
    run_benchmark "/api/v1/users/123/posts" "user_posts" "User Posts (nested route)"
    run_benchmark "/api/v1/organizations/10/projects/20/tasks/30" "complex_nested" "Complex Nested Route (3 params)"
    
    print_subheader "Testing Query Parameters"
    run_benchmark "/api/v1/search?q=test&limit=20&offset=0" "search_query" "Search with Query Params"
    
    print_subheader "Testing Wildcard Routes"
    run_benchmark "/static/images/logo.png" "static_file" "Static File Route"
    run_benchmark "/static/css/styles/main.css" "static_nested" "Nested Static Route"
    
    print_subheader "Testing Error Handling"
    run_benchmark "/api/v1/notfound" "not_found" "404 Not Found (JSON error)"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        -n|--requests)
            TOTAL_REQUESTS="$2"
            shift 2
            ;;
        -c|--concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        --no-auto-start)
            AUTO_START_SERVER=false
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -h, --host HOST          Server host (default: localhost)"
            echo "  -p, --port PORT          Server port (default: 8080)"
            echo "  -n, --requests NUM       Total number of requests (default: 10000)"
            echo "  -c, --concurrency NUM    Number of concurrent requests (default: 100)"
            echo "  --no-auto-start          Don't auto-start the test server"
            echo "  --help                   Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                       # Auto-start server and run with defaults"
            echo "  $0 -n 50000 -c 200       # Auto-start with custom settings"
            echo "  $0 --no-auto-start       # Use existing server"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Main execution
main() {
    check_dependencies
    check_server
    setup_results_dir
    run_warmup
    run_all_benchmarks
    generate_summary
}

# Run main function
main