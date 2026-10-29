# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal ./internal

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/webhook .

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/contentways/cert-manager-webhook-poweradmin" \
      org.opencontainers.image.description="cert-manager DNS01 webhook for Poweradmin" \
      org.opencontainers.image.licenses="MIT"

COPY --from=build /out/webhook /webhook
USER 65532:65532
ENTRYPOINT ["/webhook"]
