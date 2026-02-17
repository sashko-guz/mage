# Mage

> [!WARNING]
> This project was developed mostly with AI agents.
> It is for learning and experimentation, and is not recommended for production use.

Go image processing service powered by libvips.

Mage generates thumbnails on-the-fly, supports local/S3 storage backends, and includes multi-layer caching for performance.

## Highlights

- Thumbnail generation via URL-based API
- Local filesystem and S3/S3-compatible storage support
- Optional HMAC request signature validation
- Memory + disk caching with async disk writes
- Docker and docker-compose support

## Quick Start

```bash
git clone https://github.com/sashko-guz/mage.git
cd mage
cp .env.example .env
cp storage.local.example.json storage.json
go run ./cmd/server
```

You can configure runtime settings from `.env` (for example `LOG_LEVEL=error`).

## Documentation

- [Getting Started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [URL API and Filters](docs/api.md)
- [Caching](docs/caching.md)
- [S3 HTTP Client Optimization](docs/s3-http-client.md)
- [Project Structure](docs/project-structure.md)

## Inspiration & Thanks

- [cshum/vipsgen](https://github.com/cshum/vipsgen)
- [cshum/imagor](https://github.com/cshum/imagor)

## License

Project is created for learning purposes and is `NOT RECOMMENDED` to be used `IN PRODUCTION`
