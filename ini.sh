# Create standard Go project directories
mkdir -p api/v1          # For API contracts/protos
mkdir -p configs         # For configuration files
mkdir -p scripts         # For deployment/build scripts
mkdir -p test/e2e        # For different test types
mkdir -p third_party     # For third party dependencies

# Reorganize existing directories
mv pkg/config internal/          # Move config to internal implementation
mv pkg/log internal/logger       # Move logging to internal
mv pkg/utils internal/           # Move utilities to internal

# Create new service layer
mkdir -p internal/service        # For business logic services
mkdir -p internal/repository     # For data access layer
mkdir -p internal/transport      # For HTTP/gRPC handlers

# Cleanup empty directories
rmdir pkg 2>/dev/null || true

# Update build script
echo '#!/bin/sh
go build -ldflags "-s -w" -trimpath -o ./bin/core ./cmd/core' > build.sh
