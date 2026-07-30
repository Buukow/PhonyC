# syntax=docker/dockerfile:1

FROM node:20-bookworm AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY internal/webembed /src/internal/webembed
RUN npx vite build

FROM golang:1.26-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/webembed/dist ./internal/webembed/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/phonyg ./cmd/phonyg

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --create-home --uid 10001 --shell /usr/sbin/nologin phonyg \
    && mkdir -p /data \
    && chown phonyg:phonyg /data
COPY --from=backend /out/phonyg /usr/local/bin/phonyg
USER phonyg
ENV PHONYG_ADDR=0.0.0.0:8080 \
    PHONYG_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/phonyg"]
