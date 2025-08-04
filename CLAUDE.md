# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based utility called `bootstrap` that downloads files from AWS S3 using EC2 instance profiles and optionally executes them. It's designed for cloud environments where instances need to fetch and run scripts or executables from S3.

## Build and Development Commands

```bash
# Build the binary
make build                      # Creates dist/bootstrap binary

# Run tests
go test -v ./...               # Run all tests with verbose output
go test ./internal/s3client    # Run tests for specific package

# Release workflow
make bump                      # Update version references in README
make tag                       # Create and push git tag
make release                   # Create GitHub release
```

## Architecture

The codebase follows a simple structure:

- **main.go**: Core application logic including:
  - S3 URL parsing
  - File download and temporary storage
  - Cross-platform execution logic (Windows/Unix)
  - Optional post-execution actions (shutdown)
  
- **internal/s3client/**: AWS S3 client abstraction
  - Provides testable interface for S3 operations
  - Handles S3 object downloads with context support

Key architectural decisions:
- Uses AWS SDK v2 for modern Go AWS integration
- Relies on EC2 instance profiles for authentication (no hardcoded credentials)
- Cross-platform support with OS-specific execution logic
- Temporary file handling with proper cleanup on signals
- Context-based cancellation for graceful shutdown

## Testing Approach

The project uses Go's standard testing package. The S3 client is designed with dependency injection to allow mocking in tests (see the `GetObjectAPI` interface in internal/s3client/s3.go:12).