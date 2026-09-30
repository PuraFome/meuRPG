# MeuRPG image: the Go API server plus the Angular web app it serves on the
# same origin (see docs/arquitetura.md#frontend-web). Build context: the
# repo root, because this needs both backend/ and web/. The Dockerfile
# itself stays under backend/ (that is still the Go module it builds), so
# every build passes -f explicitly:
#
#   docker build -f backend/Dockerfile \
#     --build-arg VERSION=v0.1.0 --build-arg COMMIT=$(git rev-parse HEAD) \
#     -t meurpg-api .
#
# All three base images are pinned by digest (the multi-platform index, so
# the same line works on amd64 and arm64). A tag can be moved to a different
# image; a digest cannot. Refresh them with:
#   docker buildx imagetools inspect <image:tag> --format '{{.Manifest.Digest}}'

# ---- build stage: compile the Angular app -----------------------------------
FROM node:22-trixie-slim@sha256:b26b04c123d9ff8ab646ceb18b9d75a1173acf64b9a401094b906d27b29338d4 AS web-build

WORKDIR /src

# package.json/package-lock.json first, in their own layer: it is only
# rebuilt when a dependency changes, not on every source change.
COPY web/package.json web/package-lock.json ./
# Never run install scripts from a third-party package (see CONTRIBUTING.md).
RUN --mount=type=cache,target=/root/.npm \
    npm ci --ignore-scripts

COPY web/ .
RUN npm run build

# ---- build stage: compile static binaries -----------------------------------
FROM golang:1.27.1-trixie@sha256:433790e515d27dc6003e847e644cc0af956985cf315c1c58a3b73ee2dd305183 AS build

WORKDIR /src

# Download modules first, in their own layer: it is only rebuilt when
# go.mod/go.sum change, not on every code change.
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend/ .

ARG VERSION=dev
ARG COMMIT=unknown

# CGO_ENABLED=0: pure Go, statically linked, so it runs on distroless/static
#   (no libc there).
# -trimpath: no local file paths inside the binary (reproducible builds).
# -s -w: strip debug symbols to make the binary smaller.
# -X: inject the build information into cmd/api's version/commit variables.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
      -o /out/migrate ./cmd/migrate

# ---- runtime stage: just the binaries and the web build ---------------------
# distroless/static has CA certificates and timezone data but no shell or
# package manager, which leaves very little to attack. The :nonroot variant
# runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/api /out/migrate /app/
# Matches config.DefaultWebDir; override with $WEB_DIR if it ever moves.
COPY --from=web-build /src/dist/web/browser /app/web

USER nonroot:nonroot
EXPOSE 8080

# The API is the default. Run migrations with the same image:
#   docker run --rm -e DATABASE_URL=... --entrypoint /app/migrate <image> up
ENTRYPOINT ["/app/api"]
