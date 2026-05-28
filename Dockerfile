FROM golang:1.26-alpine AS builder

WORKDIR /usr/src/app

# Copy the whole project
COPY . .

# Build using the vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build \
    -mod=vendor -ldflags="-s -w" \
    -o=./limelight-api ./cmd/api

# --- Final Run Stage ---
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /

# Copy the compiled binary from the builder stage
COPY --from=builder /usr/src/app/limelight-api /limelight-api

# Copy migrations folder, keeping them handy
COPY --from=builder /usr/src/app/migrations /migrations

# Expose the application port
EXPOSE 4000

CMD ["/limelight-api"]
