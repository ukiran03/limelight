# Run all checks
check: vet lint

# Fast correctness check using standard Go tools
vet:
    go vet ./...

# Comprehensive linting suite
lint:
    golangci-lint run ./...

# Automatically fix what can be fixed (formatting, simple refactors)
fix:
    golangci-lint run --fix ./...
