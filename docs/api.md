# URL API and Filters

## URL Format

```text
/thumbs/[{signature}/]{width}x{height}/[filters:{filters}/]{path}
```

### Components

- `{signature}` - Optional HMAC-SHA256 signature
- `{width}x{height}` - Required size (example: `200x350`)
- `{filters}` - Optional filter list split by `;`
- `{path}` - Source image path in storage

## Available Filters

### `format(format)`

- Supported: `jpeg`, `png`, `webp`
- Default: from extension, fallback `jpeg`

### `quality(level)`

- Range: `1..100`
- Default: `75`

### `fit(mode[,color])`

- Modes: `cover` (default), `fill`
- Fill colors for `fill`: `black`, `white`, `transparent` (PNG only)

### `crop(x1,y1,x2,y2)`

Pixel-based crop before resize/fit.

Validation:

- all coordinates non-negative
- `x2 > x1`
- `y2 > y1`
- crop bounds must be valid for source image

### `pcrop(x1,y1,x2,y2)`

Percent-based crop (`0..100`) before resize/fit.

Validation:

- all coordinates in `0..100`
- `x2 > x1`
- `y2 > y1`
- cannot be combined with `crop`

## Example URLs

Without filters:

```text
/thumbs/200x350/path/to/image.jpg
```

With filters:

```text
/thumbs/200x350/filters:format(webp);quality(90);fit(fill,black)/path/to/image.jpg
```

With signature:

```text
/thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(webp);quality(88)/path/to/image.jpg
```

## Signature Generation

When `signature_secret` is configured, signature is required.

Payload format:

```text
/{size}/[filters:{filters}/]{path}
```

The signature is first 16 hex chars of HMAC-SHA256 over payload.
