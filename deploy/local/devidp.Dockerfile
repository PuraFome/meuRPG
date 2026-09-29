# devidp: the development-only OpenID Connect provider (backend/cmd/devidp)
# behind the local stack (compose.yaml) and the Playwright tests in CI.
#
# It signs anyone in without a password, so it lives in its own image, used
# only by compose.yaml. The production image, backend/Dockerfile, builds only
# cmd/api and cmd/migrate, and devidp itself refuses to start unless its
# issuer is on a loopback host (see the package comment of cmd/devidp).
#
# Build context: the repo root, like backend/Dockerfile, so both share the
# root .dockerignore (no test files, no node_modules). The base images are
# the same ones, pinned by the same digests; refresh them together.

# ---- build stage -----------------------------------------------------------
FROM golang:1.27.1-trixie@sha256:433790e515d27dc6003e847e644cc0af956985cf315c1c58a3b73ee2dd305183 AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/devidp ./cmd/devidp

# ---- runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/devidp /app/devidp

USER nonroot:nonroot
EXPOSE 9090

ENTRYPOINT ["/app/devidp"]
