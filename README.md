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

## Logging

Each non-`/healthz` request writes an `access` log with the request URL, cache status, response status, bytes, and duration. It also logs `client_referer`/`client_user_agent` as received by the service and `origin_referer`/`origin_user_agent`/`origin_status` as sent to or returned by the origin when an origin request is made.

## CORS

CORS is enabled by default so browser clients can use the proxy with `fetch()` and read cache diagnostics:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`
- `Access-Control-Expose-Headers: X-Piccache-Status, Warning`

Set `cors.enabled: false` or restrict `cors.allowed_origins` in `config.yaml` if the authenticated `POST /` warm-cache endpoint should only be callable from selected browser origins.

## Run

```sh
go run ./cmd/freshrss-image-cache-service
```

## Docker

Images are built and published by GitHub Actions on every push to `main`:

- `ghcr.io/stek29/freshrss-image-cache-service:latest`
- `ghcr.io/stek29/freshrss-image-cache-service:sha-<shortsha>`

```sh
docker run --rm -p 3000:3000 \
  -e IMAGE_CACHE_ACCESS_TOKEN=change-me \
  -v "$PWD/images:/data/images" \
  ghcr.io/stek29/freshrss-image-cache-service:latest
```

### Docker Compose

See [docker-compose.yaml](docker-compose.yaml). Create a local config file and cache directory before starting it:

```sh
cp config.example.yaml config.yaml
mkdir -p ./data/images
sudo chown 10001:10001 ./data/images
docker compose up -d
```

For local development builds:

```sh
docker build -t freshrss-image-cache-service .
```

## Storage

Cached files are stored under `data_dir`, sharded by the first two hex characters of the SHA-256 hash of the original URL. Metadata is stored as adjacent JSON.

## Related Projects

- [Victrid/freshrss-image-cache-plugin](https://github.com/Victrid/freshrss-image-cache-plugin): FreshRSS plugin that rewrites feed image URLs to use an external cache service.
- [Victrid/image-cache-worker](https://github.com/Victrid/image-cache-worker): Cloudflare Worker implementation of the external image cache service API.
- [s373r/freshrss-image-cache-service-rs](https://github.com/s373r/freshrss-image-cache-service-rs): Rust implementation of an image cache service compatible with the FreshRSS plugin API.

For `referer` support, use this plugin fork: [stek29/freshrss-image-cache-plugin@6ef70b0](https://github.com/stek29/freshrss-image-cache-plugin/commit/6ef70b09fc50c5d79c892846c239645a444c87c1).

## License

This project is licensed under the GNU General Public License v3.0. See [LICENSE](LICENSE).
