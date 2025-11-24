#!/bin/bash

# Performance test script for IDAES
# This script uploads documents and performs analysis to generate profiling data

BASE_URL="http://localhost:8081"
API_BASE="$BASE_URL/api/v1"

echo "Starting IDAES performance testing..."

# Check if service is healthy
echo "Checking service health..."
curl -f "$API_BASE/health" || {
    echo "Service not healthy! Exiting."
    exit 1
}

# Function to upload and analyze a document
test_document_analysis() {
    local doc_name="$1"
    local content="$2"
    
    echo "Testing with document: $doc_name"
    
    # Create a temporary file
    local temp_file=$(mktemp)
    echo "$content" > "$temp_file"
    
    # Upload document
    echo "Uploading document..."
    upload_response=$(curl -s -X POST \
        -F "file=@$temp_file" \
        -F "filename=$doc_name" \
        "$API_BASE/documents/upload")
    
    echo "Upload response: $upload_response"
    
    # Extract document ID (assuming JSON response)
    doc_id=$(echo "$upload_response" | grep -o '"document_id":"[^"]*"' | cut -d'"' -f4)
    
    if [ ! -z "$doc_id" ]; then
        echo "Document uploaded with ID: $doc_id"
        
        # Trigger analysis
        echo "Triggering analysis..."
        analysis_response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "{\"document_id\":\"$doc_id\"}" \
            "$API_BASE/documents/analyze")
        
        echo "Analysis response: $analysis_response"
    else
        echo "Failed to get document ID"
    fi
    
    # Cleanup
    rm -f "$temp_file"
}

# Test documents with varying complexity
echo "=== Test 1: Simple Document ==="
test_document_analysis "simple.txt" "This is a simple test document. It contains basic text for testing the system performance. The quick brown fox jumps over the lazy dog."

echo "=== Test 2: Scientific Document ==="
test_document_analysis "science.txt" "Quantum computing represents a paradigm shift in computational science. Unlike classical computers that use bits, quantum computers use quantum bits or qubits. These qubits can exist in superposition states, allowing quantum computers to process multiple possibilities simultaneously. Research by IBM, Google, and other organizations has demonstrated quantum supremacy in specific computational tasks. The implications for cryptography, optimization, and scientific simulation are profound."

echo "=== Test 3: Technical Document ==="
test_document_analysis "technical.txt" "Machine learning algorithms have revolutionized data processing and analysis. Neural networks, particularly deep learning architectures, have shown remarkable performance in tasks such as image recognition, natural language processing, and predictive analytics. Convolutional Neural Networks (CNNs) excel at image processing, while Recurrent Neural Networks (RNNs) and Transformers have transformed natural language understanding. The implementation of these algorithms requires careful consideration of hyperparameters, training data quality, and computational resources."

echo "=== Test 4: Complex Document ==="
test_document_analysis "complex.txt" "The integration of artificial intelligence in healthcare systems presents both opportunities and challenges. Electronic Health Records (EHRs) contain vast amounts of patient data that can be analyzed using machine learning algorithms to identify patterns, predict outcomes, and support clinical decision-making. However, privacy concerns, data standardization issues, and regulatory requirements must be addressed. The Health Insurance Portability and Accountability Act (HIPAA) in the United States and the General Data Protection Regulation (GDPR) in Europe establish strict guidelines for handling sensitive health information. Successful implementation requires collaboration between healthcare providers, technology companies, and regulatory bodies."

# Run multiple concurrent requests to test load handling
echo "=== Load Testing: Multiple Concurrent Requests ==="
for i in {1..5}; do
    {
        test_document_analysis "load_test_$i.txt" "Load testing document $i. This document is part of a concurrent testing scenario to evaluate system performance under load. Document number: $i. Random data: $(date +%s)"
    } &
done

# Wait for all background processes to complete
wait

echo "=== Performance Testing Complete ==="
echo "You can now analyze the performance data using:"
echo "1. CPU Profile: curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof"
echo "2. Memory Profile: curl http://localhost:6060/debug/pprof/heap > heap.prof"
echo "3. Goroutine Profile: curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof"
echo "4. Open web interface: http://localhost:6060/debug/pprof/"