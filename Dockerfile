FROM --platform=$BUILDPLATFORM golang:1.26.2 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/fvf-web ./cmd/fvf-web

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /out/fvf-web /usr/local/bin/fvf-web
ENV PORT=8080
EXPOSE 8080
CMD ["fvf-web"]
