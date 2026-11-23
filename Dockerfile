# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.1

# ---- build ----
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# Download modules first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/issue-tracker ./cmd/issue-tracker

# ---- runtime ----
# distroless/static ships CA certificates (needed for Google OAuth HTTPS calls)
# and runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/issue-tracker /app/issue-tracker

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/issue-tracker"]
