FROM golang:1.26-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/freshrss-image-cache-service ./cmd/freshrss-image-cache-service
RUN mkdir -p /out/rootfs/data/images /out/rootfs/etc/freshrss-image-cache-service /out/rootfs/etc/ssl/certs
RUN cp /etc/ssl/certs/ca-certificates.crt /out/rootfs/etc/ssl/certs/ca-certificates.crt
RUN cp config.example.yaml /out/rootfs/etc/freshrss-image-cache-service/config.yaml

FROM scratch

WORKDIR /app
COPY --from=build /out/freshrss-image-cache-service /usr/local/bin/freshrss-image-cache-service
COPY --from=build --chown=65532:65532 /out/rootfs/ /

USER 65532:65532
ENV IMAGE_CACHE_LISTEN=:3000
ENV IMAGE_CACHE_DATA_DIR=/data/images
EXPOSE 3000

ENTRYPOINT ["freshrss-image-cache-service"]
