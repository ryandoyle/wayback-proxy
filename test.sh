#!/bin/bash

# Test script for Wayback Machine Proxy
# This script tests the proxy by making requests through different ports

echo "Testing Wayback Machine Proxy..."
echo "================================"
echo ""

# Test a few different years
for year in 1995 2000 2005; do
    echo "Testing port $year (year $year)..."
    echo "Fetching http://example.com through proxy on port $year"
    
    # Make a request through the proxy
    # Using --max-time to avoid hanging if proxy isn't running
    curl --proxy http://localhost:$year \
         --max-time 10 \
         --silent \
         --show-error \
         --write-out "\nHTTP Status: %{http_code}\n" \
         --output /dev/null \
         http://example.com
    
    echo "---"
    echo ""
done

echo "Test complete!"
echo ""
echo "To see actual content, try:"
echo "  curl --proxy http://localhost:2000 http://google.com | head -50"
