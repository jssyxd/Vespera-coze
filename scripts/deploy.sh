#!/bin/bash

# Vespera Deployment Script
set -e

echo "🚀 Vespera Deployment Script"
echo "=============================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check dependencies
check_dependency() {
    if ! command -v "$1" &> /dev/null; then
        echo -e "${RED}❌ $1 is not installed${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ $1 found${NC}"
    return 0
}

echo ""
echo "📋 Checking dependencies..."
check_dependency git
check_dependency go
check_dependency docker || true  # Optional

# Load environment variables
if [ -f .env ]; then
    echo -e "${GREEN}✅ Loading environment from .env${NC}"
    export $(cat .env | grep -v '#' | xargs)
else
    echo -e "${YELLOW}⚠️  .env file not found, using .env.example${NC}"
    if [ -f .env.example ]; then
        cp .env.example .env
        echo -e "${YELLOW}📝 Please edit .env with your actual values${NC}"
    fi
fi

# Verify required env vars
echo ""
echo "🔍 Verifying configuration..."

required_vars=("SUPABASE_HOST" "SUPABASE_PASSWORD" "AI_API_KEY")
missing_vars=()

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        missing_vars+=("$var")
    fi
done

if [ ${#missing_vars[@]} -ne 0 ]; then
    echo -e "${RED}❌ Missing required environment variables:${NC}"
    printf '  - %s\n' "${missing_vars[@]}"
    exit 1
fi

echo -e "${GREEN}✅ Configuration verified${NC}"

# Build application
echo ""
echo "🔨 Building Vespera..."
cd src

go mod tidy
go build -o ../vespera ./cmd/vespera

cd ..
echo -e "${GREEN}✅ Build successful${NC}"

# Test database connection
echo ""
echo "🗄️  Testing database connection..."
if ./vespera -test-db; then
    echo -e "${GREEN}✅ Database connection successful${NC}"
else
    echo -e "${RED}❌ Database connection failed${NC}"
    exit 1
fi

# Run initial scan test (optional)
echo ""
read -p "🧪 Run a test scan? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Running test scan..."
    ./vespera -m mode2 -c eth -range 20000000-20000100
fi

echo ""
echo -e "${GREEN}🎉 Deployment completed successfully!${NC}"
echo ""
echo "Next steps:"
echo "  1. Push to GitHub: git push origin main"
echo "  2. Configure GitHub Secrets in repository settings"
echo "  3. Trigger first scan from GitHub Actions"
echo ""
