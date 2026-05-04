# List available recipes
default:
    @just --list

set dotenv-load := true
set dotenv-filename := ".envrc"

# LIMELIGHT_DB_DSN := env_var('LIMELIGHT_DB_DSN')

# Tidy module dependencies
tidy:
    @echo '> Tidying module dependencies...'
    go mod tidy
    @echo '> Verifying and vendoring module dependencies...'
    go mod verify
    go mod vendor
    @echo '> Formatting .go files...'
    go fmt ./...

# run quality control checks
audit:
    @echo '> Checking module dependencies...'
    go mod tidy -diff
    go mod verify
    @echo '> Vetting code...'
    go vet ./...
    go tool staticcheck ./...
    @echo '> Running tests...'
    go test -race -vet=off ./...

# Comprehensive golangci-lint
ci-lint:
    golangci-lint run ./...

# Automatic refactors and formatting
ci-fix:
    golangci-lint run --fix ./...

# Run the app with wgo
run:
    wgo run ./cmd/api -db-dsn=${LIMELIGHT_DB_DSN}

## Database migration recipes

# New Database migration
[group('db-migrations')]
db-migrate-new name:
    @echo 'Creating migration files for {{name}}...'
    migrate create -seq -ext=.sql -dir=./migrations {{name}}

# Up Database migrations
[group('db-migrations')]
[confirm("Are you sure you want to run migrations? [y/N]")]
db-migrate-up:
    @echo 'Running up migrations...'
    migrate -path ./migrations -database ${LIMELIGHT_DB_DSN} up
