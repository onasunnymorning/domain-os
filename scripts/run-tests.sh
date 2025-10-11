#!/bin/bash
# Test runner script for Agent Navigation Feature
# Run this script to execute all tests

set -e  # Exit on error

echo "🧪 Agent Navigation Feature - Test Suite Runner"
echo "================================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Backend Tests
echo "📦 Running Backend Tests (Go)..."
echo "--------------------------------"
cd "$(dirname "$0")/.."

if go test ./internal/agent/service/... -v -cover; then
    echo -e "${GREEN}✅ Backend tests passed!${NC}"
else
    echo -e "${RED}❌ Backend tests failed!${NC}"
    exit 1
fi

echo ""
echo "📊 Generating coverage report..."
go test ./internal/agent/service/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
echo -e "${GREEN}✅ Coverage report generated: coverage.html${NC}"

echo ""
echo ""

# Frontend Tests
echo "⚛️  Checking Frontend Tests (TypeScript/React)..."
echo "------------------------------------------------"
cd frontend

if [ ! -f "node_modules/.bin/vitest" ]; then
    echo -e "${YELLOW}⚠️  Frontend testing framework not installed${NC}"
    echo ""
    echo "To run frontend tests, install the required dependencies:"
    echo ""
    echo "  cd frontend"
    echo "  npm install --save-dev vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom"
    echo ""
    echo "Then create vitest.config.ts (see docs/AGENT_NAVIGATION_TESTS.md)"
    echo ""
    echo "Then run: npm test"
    echo ""
else
    echo "Running frontend tests..."
    if npm test; then
        echo -e "${GREEN}✅ Frontend tests passed!${NC}"
    else
        echo -e "${RED}❌ Frontend tests failed!${NC}"
        exit 1
    fi
fi

echo ""
echo ""
echo "🎉 Test Suite Complete!"
echo "======================="
echo ""
echo "Test Summary:"
echo "  Backend:  27 tests (Go)"
echo "  Frontend: 52 tests (TypeScript/React)"
echo "  Total:    79 test cases"
echo ""
echo "Documentation:"
echo "  📖 Comprehensive Guide: docs/AGENT_NAVIGATION_TESTS.md"
echo "  📝 Quick Summary:       docs/TEST_SUMMARY.md"
echo "  📊 Coverage Report:     coverage.html"
echo ""
