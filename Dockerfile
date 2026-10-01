# syntax=docker/dockerfile:1
#
# Production image for the Sluss router (cmd/router).
#
# The router needs no database to run — it uses in-memory trackers and reaches
# the configured provider over HTTPS — so this single container is a complete
# deployment. Configure it entirely via environment variables (see .env.example);
# at minimum set LOCAL_API_KEY (a strong secret, not the dev default) and, for a
# real provider, OPENROUTER_API_KEY.
#
# Build:  docker build -t sluss-router .
# Run:    docker run -p 8080:8080 -e LOCAL_API_KEY=... -e OPENROUTER_API_KEY=... sluss-router

# --- build stage ---
# Must satisfy the go directive in go.mod (bumped to 1.25 by modernc.org/sqlite,
# the pure-Go SQLite driver behind durable history — ISSUE-070).
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads separately from source for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# The commit and build time are stamped into the binary so a running instance
# can say exactly which build it is (/metrics sluss_build_info, console footer,
# MCP server_info). The image workflow passes them; a local build says "dev".
ARG GIT_SHA=dev
ARG BUILD_TIME=
# Static, stripped binary so the runtime image can stay minimal.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/magnusfroste/sluss/internal/buildinfo.Commit=${GIT_SHA} -X github.com/magnusfroste/sluss/internal/buildinfo.Built=${BUILD_TIME}" \
    -o /out/router ./cmd/router

# --- runtime stage ---
FROM alpine:3.20

# ca-certificates so the router can call providers over HTTPS (e.g. OpenRouter);
# wget (busybox) backs the healthcheck. Run as a non-root user.
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 app \
 && mkdir -p /data && chown app:app /data

COPY --from=build /out/router /usr/local/bin/router

USER app
ENV ROUTER_ADDR=:8080
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1

ENTRYPOINT ["router"]
