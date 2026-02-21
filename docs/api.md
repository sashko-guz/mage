# URL API and Filters

## URL Format

```text
/thumbs/[{signature}/]{width}x{height}/[filters:{filters}/]{path}[/as/{alias.ext}]
```

### Components

- `{signature}` - Optional HMAC-SHA256 signature
- `{width}x{height}` - Required size (example: `200x350`)
	- each dimension must be `1..10000`
- `{filters}` - Optional filter list split by `;`
- `{path}` - Source image path in storage
- `{alias.ext}` - Optional output alias suffix (`/as/{alias.ext}`)

## Available Filters

### `format(format)`

- Supported: `jpeg`, `png`, `webp`, `avif`
- Default: alias extension when `/as/{alias.ext}` is present, otherwise source path extension, fallback `jpeg`
- If both alias extension and explicit `format(...)` filter are present, they must match

### `quality(level)`

- Range: `1..100`
- Default: `75`

### `fit(mode[,color])`

- Modes: `cover` (default), `fill`
- Fill colors for `fill`: `black`, `white`, `transparent` (PNG, WebP, and AVIF)

### Resize (`{width}x{height}`)

Validation:

- width and height must be positive integers when provided
- maximum value for width or height is `10000`

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
/thumbs/200x350/filters:format(avif);quality(90);fit(fill,black)/path/to/image.jpg
```

With signature:

```text
/thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(webp);quality(90)/path/to/image.jpg
```

With alias:

```text
/thumbs/200x350/path/to/image.jpg/as/card.avif
```

With signature and alias:

```text
/thumbs/a1b2c3d4e5f6g7h8/200x350/filters:format(avif);quality(90)/path/to/image.jpg/as/card.avif
```

## Signature Generation

See [Signature Generation](signature.md) for payload rules and implementation examples (Node.js and PHP).
