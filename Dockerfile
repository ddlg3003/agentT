# Stage 1: Build frontend (native arch — avoids QEMU crash with esbuild binary)
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

# Stage 2: Build backend (cross-compile Go natively, no emulation needed)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/server ./cmd/server

# Stage 3: Final minimal image (target platform: linux/amd64)
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/server /app/server
COPY --from=frontend /app/dist /app/dist
COPY backend/mock /app/mock
ENV STATIC_DIR=/app/dist
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
