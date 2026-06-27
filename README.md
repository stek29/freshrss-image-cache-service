# FreshRSS Image Cache Service

Go image cache service compatible with the FreshRSS image cache plugin APIs.

## API

```sh
curl 'http://localhost:3000/?url=https%3A%2F%2Fexample.com%2Fimage.jpg'
```

```sh
curl -X POST http://localhost:3000/ \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/image.jpg","access_token":"change-me"}'
```

Successful POST responses use the compatibility body:

```json
{"status":"OK"}
```

GET responses include `X-Piccache-Status` with `MISS`, `HIT`, `REVALIDATED`, `REFRESHED`, `STALE`, or `BYPASS`.

## Configuration

Config lookup order:

1. `IMAGE_CACHE_CONFIG`
2. `./config.yaml`
3. `/etc/freshrss-image-cache-service/config.yaml`

Environment overrides:

- `IMAGE_CACHE_LISTEN`
- `IMAGE_CACHE_DATA_DIR`
- `IMAGE_CACHE_ACCESS_TOKEN`
- `IMAGE_CACHE_CONFIG`

See [config.example.yaml](config.example.yaml).

## Run

```sh
go run ./cmd/freshrss-image-cache-service
```

## Docker

```sh
docker build -t freshrss-image-cache-service .
docker run --rm -p 3000:3000 \
  -e IMAGE_CACHE_ACCESS_TOKEN=change-me \
  -v "$PWD/images:/data/images" \
  freshrss-image-cache-service
```

## Storage

Cached files are stored under `data_dir`, sharded by the first two hex characters of the SHA-256 hash of the original URL. Metadata is stored as adjacent JSON.
