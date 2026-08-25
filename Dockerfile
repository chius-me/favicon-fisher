FROM --platform=$BUILDPLATFORM golang:1.26.6 AS builder
ARG TARGETOS
ARG TARGETARCH
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/fvf-web ./cmd/fvf-web

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 nonroot \
    && useradd --uid 65532 --gid 65532 --home-dir /app --shell /usr/sbin/nologin nonroot
WORKDIR /app
COPY --from=builder /out/fvf-web /usr/local/bin/fvf-web
ENV PORT=8080
EXPOSE 8080
USER 65532:65532
CMD ["fvf-web"]
