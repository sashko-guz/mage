# URL API

## URL Format

```text
/thumbs/[{signature}/]{width}x{height}/[filters:{filters}/]{path}[/as/{alias.ext}]
/thumbs/[{signature}/]{width}x{height}/[f:{filters}/]{path}[/as/{alias.ext}]
```

Both `filters:` and the short alias `f:` are accepted.

### Components

| Segment | Required | Description |
|---|---|---|
| `{signature}` | no | HMAC signature for request validation |
| `{width}x{height}` | yes | Output dimensions — either can be omitted to scale proportionally |
| `filters:` / `f:` | no | Filter segment prefix |
| `{filters}` | no | Semicolon-separated list of operations |
| `{path}` | yes | Source image path in storage |
| `/as/{alias.ext}` | no | Output filename hint — also sets the default format |

See [Operations](operations.md) for the full list of available filters, aliases, defaults, and validation rules.

## Example URLs

No filters — defaults applied (cover fit, jpeg, quality 75):

```text
/thumbs/400x300/photos/cat.jpg
```

With filters (full names):

```text
/thumbs/400x300/filters:format(webp);quality(90)/photos/cat.jpg
```

With filters (short aliases):

```text
/thumbs/400x300/f:fmt(webp);q(90)/photos/cat.jpg
```

With signature:

```text
/thumbs/a1b2c3d4e5f6g7h8/400x300/f:fmt(webp);q(90)/photos/cat.jpg
```

With alias (sets output format automatically):

```text
/thumbs/400x300/photos/cat.jpg/as/card.avif
```

With crop, fit, format, and quality:

```text
/thumbs/400x300/filters:crop(50,30,350,230);fit(fill,black);format(webp);quality(85)/photos/cat.jpg
/thumbs/400x300/f:c(50,30,350,230);fit(fill,black);fmt(webp);q(85)/photos/cat.jpg
```

With signature and alias:

```text
/thumbs/a1b2c3d4e5f6g7h8/400x300/f:fmt(avif);q(90)/photos/cat.jpg/as/card.avif
```

## Signature Generation

See [Signature Generation](signature.md) for payload rules and implementation examples.
