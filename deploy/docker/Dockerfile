# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
# proxy CA only for mod download, not in img
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then \
      SSL_CERT_FILE=/run/secrets/proxy_ca go mod download; \
    else go mod download; fi
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/ web/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=readonly -trimpath -ldflags='-s -w' -o /sleepy-go ./cmd/sleepy-go \
    && mkdir -p /runtime/data && chown 65532:65532 /runtime/data

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /sleepy-go /sleepy/sleepy-go
COPY --from=build --chown=65532:65532 /runtime/data /sleepy/data
# keep /sleepy/data, old mounts
WORKDIR /sleepy
USER 65532:65532
ENV sleepy_main_host=0.0.0.0
EXPOSE 9010
VOLUME ["/sleepy/data"]
ENTRYPOINT ["/sleepy/sleepy-go"]
