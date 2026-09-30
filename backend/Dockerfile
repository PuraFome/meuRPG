# MeuRPG backend image. Build context: backend/
#
#   docker build -t meurpg-api \
#     --build-arg VERSION=v0.1.0 --build-arg COMMIT=$(git rev-parse HEAD) \
#     backend
#
# Both base images are pinned by digest (the multi-platform index, so the
# same line works on amd64 and arm64). A tag can be moved to a different
# image; a digest cannot. Refresh them with:
#   docker buildx imagetools inspect <image:tag> --format '{{.Manifest.Digest}}'

# ---- build stage: compile static binaries -----------------------------------
FROM golang:1.27.1-trixie@sha256:433790e515d27dc6003e847e644cc0af956985cf315c1c58a3b73ee2dd305183 AS build

WORKDIR /src

# Download modules first, in their own layer: it is only rebuilt when
# go.mod/go.sum change, not on every code change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

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

# ---- runtime stage: just the binaries ---------------------------------------
# distroless/static has CA certificates and timezone data but no shell or
# package manager, which leaves very little to attack. The :nonroot variant
# runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/api /out/migrate /app/

USER nonroot:nonroot
EXPOSE 8080

# The API is the default. Run migrations with the same image:
#   docker run --rm -e DATABASE_URL=... --entrypoint /app/migrate <image> up
ENTRYPOINT ["/app/api"]
