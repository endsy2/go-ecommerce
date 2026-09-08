# syntax=docker/dockerfile:1

# --- build ------------------------------------------------------------------
# Multi-stage. The Go toolchain is ~350MB and is needed only to produce the
# binary, so it stays in this stage; the final image below copies out the binary
# alone. Beyond size, the runtime image ends up with no compiler and no source,
# so there is nothing in it to build or modify.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Set once rather than repeated on each go build below, so the two binaries
# cannot drift onto different flags.
#
# CGO_ENABLED=0 produces a statically linked binary with no libc dependency.
# That is what lets a binary built here run on plain alpine — the default cgo
# build would link against glibc and die with "not found" on musl.
ENV CGO_ENABLED=0 GOOS=linux

# The manifests are copied and resolved BEFORE the source, in their own layer.
# Docker invalidates a layer only when the files that produced it change, so
# editing a .go file reuses this layer; only a go.mod/go.sum edit redoes it.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# The cache mounts are what make "rebuild the image on every code change" a
# usable dev loop. Layer caching alone does not help: `COPY . .` above
# invalidates this layer on any source edit, and the re-run starts from a
# filesystem holding no compiled objects — so all ~85 modules (gin, gorm, pgx,
# sonic, otel, the testcontainers tree) recompile from scratch every time. A
# cache mount is not part of any layer and survives that invalidation, turning a
# one-file edit from tens of seconds into a second or two.
#
# -trimpath removes local filesystem paths from the binary, and -s -w drop the
# symbol table and DWARF debug info: together roughly a third of the size, and
# none of it is useful in a container you debug through logs.
#
# Both binaries build in one RUN so the second reuses every package the first
# compiled; cmd/migrate then costs little more than its own main package.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime ----------------------------------------------------------------
FROM alpine:3.21

# Root certificates for outbound TLS. Nothing needs them yet — Postgres and
# Redis are plaintext on the compose network — but the failure when something
# eventually does (a payment API, object storage) is an opaque x509 error, so
# the ~1MB is worth not debugging that later.
#
# alpine rather than distroless, despite the static binary: the health check
# below needs a shell and busybox wget, which distroless/static does not ship.
RUN apk add --no-cache ca-certificates

# An unprivileged user. Root inside a container is still root against a bind
# mount or a kernel escape, and an HTTP server that only listens on a port needs
# no privilege whatsoever.
RUN addgroup -S app && adduser -S -G app app
USER app

WORKDIR /app

# Both commands ship. cmd/api serves; cmd/migrate applies the schema, so the
# image can run migrations as a deploy step without a second build:
#   docker compose run --rm backend migrate status
# Shipping them together also guarantees they are always the same commit, so
# migrate cannot apply a schema this api does not expect.
COPY --from=build /out/api /out/migrate /usr/local/bin/

# Metadata for `docker inspect` and `docker run -P`. It publishes nothing on its
# own; compose maps the port.
EXPOSE 8080

# Declared here rather than in docker-compose.yml so it travels with the image:
# a bare `docker run`, an ECS task, or a `docker inspect` during an incident all
# get the same signal instead of each having to rediscover which path means
# healthy. Compose inherits this automatically.
#
# /health/ready, not /health. The latter proves only that the process is alive.
# Ready pings Postgres, and deliberately still returns 200 ("degraded") when
# only Redis is down, so a cache blip does not take the instance out of
# rotation. Aiming a liveness probe at Ready instead would restart every
# instance on a brief database blip.
#
# wget, not curl: alpine ships busybox wget and no curl. The 8080 here must move
# with EXPOSE above and HTTP_PORT if the port is ever changed.
#
# start-period is generous because a first start also applies migrations when
# POSTGRES_AUTO_MIGRATE is on.
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=5 \
    CMD wget -q -O /dev/null http://localhost:8080/health/ready || exit 1

# No shell form, so the binary is PID 1 and receives SIGTERM directly. cmd/api
# traps it and drains in-flight requests; wrapped in `sh -c` the signal would
# reach the shell instead and the container would be killed after the timeout.
CMD ["api"]
