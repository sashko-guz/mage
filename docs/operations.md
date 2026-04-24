# Operations

Operations are applied to images during thumbnail generation. They are passed as a semicolon-separated filter list in the URL.

```text
/thumbs/{size}/f:{op1};{op2};.../path/to/image.jpg
/t/{size}/f:{op1};{op2};.../path/to/image.jpg
```

Both `/thumbs/` and `/t/` are accepted. Filter prefix `filters:` and `f:` are both accepted.

Rules:

- Only one operation of each type is allowed per request
- Operations are applied in order: crop/pcrop → fit → resize → format/quality (export)
- `crop` and `pcrop` cannot be used together

---

## format

Sets the output image format.

**Alias:** `fmt`

**Syntax:** `format(type)` or `fmt(type)`

**Supported values:** `jpeg`, `jpg`, `png`, `webp`, `avif`

**Default:** alias extension when `/as/{alias.ext}` is present, otherwise source path extension, fallback `jpeg`

Note: if both an alias extension and an explicit `format(...)` are present, they must match.

**Examples:**

```text
# Explicit format
/thumbs/400x300/filters:format(webp)/photos/cat.jpg
/thumbs/400x300/f:fmt(webp)/photos/cat.jpg

# Default — format detected from source extension (jpeg)
/thumbs/400x300/photos/cat.jpg

# Default — format detected from alias extension (avif)
/thumbs/400x300/photos/cat.jpg/as/card.avif
```

---

## quality

Sets the compression quality for the output image.

**Alias:** `q`

**Syntax:** `quality(level)` or `q(level)`

**Range:** `1..100`

**Default:** `75`

**Examples:**

```text
# Explicit quality
/thumbs/400x300/filters:quality(90)/photos/cat.jpg
/thumbs/400x300/f:q(90)/photos/cat.jpg

# Default quality (75) — no quality filter needed
/thumbs/400x300/photos/cat.jpg

# Combined with format
/thumbs/400x300/filters:format(avif);quality(85)/photos/cat.jpg
/thumbs/400x300/f:fmt(avif);q(85)/photos/cat.jpg
```

---

## fit

Controls how the image is fitted into the requested dimensions.

**Alias:** none

**Syntax:** `fit(mode)` or `fit(mode,color)`

**Modes:**

- `cover` — crops to fill the target dimensions (default when no `fit` filter is provided)
- `fill` — resizes to fit within the dimensions and pads the remaining area with a fill color

**Fill colors** (only for `fill` mode): `white` (default), `black`, `transparent`

Note: `transparent` requires `png`, `webp`, or `avif` format.

**Examples:**

```text
# Cover mode (default — no filter needed)
/thumbs/400x300/photos/cat.jpg

# Explicit cover
/thumbs/400x300/filters:fit(cover)/photos/cat.jpg
/thumbs/400x300/f:fit(cover)/photos/cat.jpg

# Fill with default white padding
/thumbs/400x300/filters:fit(fill)/photos/cat.jpg
/thumbs/400x300/f:fit(fill)/photos/cat.jpg

# Fill with black padding
/thumbs/400x300/filters:fit(fill,black)/photos/cat.jpg
/thumbs/400x300/f:fit(fill,black)/photos/cat.jpg

# Fill with transparent padding (requires PNG/WebP/AVIF)
/thumbs/400x300/filters:format(png);fit(fill,transparent)/photos/cat.jpg
/thumbs/400x300/f:fmt(png);fit(fill,transparent)/photos/cat.jpg
```

---

## crop

Pixel-based crop applied before resize.

**Alias:** `c`

**Syntax:** `crop(x1,y1,x2,y2)` or `c(x1,y1,x2,y2)`

Coordinates are absolute pixel values in the source image.

**Validation:**

- all coordinates must be non-negative integers
- `x2 > x1` and `y2 > y1`
- crop bounds must fit within the source image dimensions
- cannot be combined with `pcrop`

**Examples:**

```text
# Crop a 300x200 region starting at (50,30)
/thumbs/200x150/filters:crop(50,30,350,230)/photos/cat.jpg
/thumbs/200x150/f:c(50,30,350,230)/photos/cat.jpg

# Crop then resize to fit
/thumbs/200x150/filters:crop(0,0,800,600);fit(fill,black)/photos/cat.jpg
/thumbs/200x150/f:c(0,0,800,600);fit(fill,black)/photos/cat.jpg

# Crop with format and quality
/thumbs/200x150/filters:crop(50,30,350,230);format(webp);quality(90)/photos/cat.jpg
/thumbs/200x150/f:c(50,30,350,230);fmt(webp);q(90)/photos/cat.jpg
```

---

## pcrop

Percent-based crop applied before resize. Coordinates are percentages (`0..100`) of the source image dimensions.

**Alias:** `pc`

**Syntax:** `pcrop(x1,y1,x2,y2)` or `pc(x1,y1,x2,y2)`

**Validation:**

- all coordinates in `0..100`
- `x2 > x1` and `y2 > y1`
- cannot be combined with `crop`

**Examples:**

```text
# Crop the center 50% of the image
/thumbs/400x300/filters:pcrop(25,25,75,75)/photos/cat.jpg
/thumbs/400x300/f:pc(25,25,75,75)/photos/cat.jpg

# Crop the top half
/thumbs/400x200/filters:pcrop(0,0,100,50)/photos/cat.jpg
/thumbs/400x200/f:pc(0,0,100,50)/photos/cat.jpg

# Percent crop with format and quality
/thumbs/400x300/filters:pcrop(10,10,90,90);format(avif);quality(80)/photos/cat.jpg
/thumbs/400x300/f:pc(10,10,90,90);fmt(avif);q(80)/photos/cat.jpg
```

---

## resize (size segment)

Controls the output dimensions. This is always the `{width}x{height}` segment in the URL — not a filter.

**Syntax:** `{width}x{height}`, `{width}x`, `x{height}`, `x`

Either dimension can be omitted — the missing dimension is calculated to preserve the original aspect ratio. Using `x` alone keeps the original dimensions.

**Validation:**

- provided dimensions must be positive integers
- maximum value for each dimension is configurable via `MAX_RESIZE_WIDTH` / `MAX_RESIZE_HEIGHT` (default: `5120`)
- maximum total resolution is configurable via `MAX_RESIZE_RESOLUTION` (default: `MAX_RESIZE_WIDTH × MAX_RESIZE_HEIGHT`)

**Examples:**

```text
# Fixed width and height
/thumbs/400x300/photos/cat.jpg

# Width only — height scales proportionally
/thumbs/400x/photos/cat.jpg

# Height only — width scales proportionally
/thumbs/x300/photos/cat.jpg

# Original dimensions — no resizing
/thumbs/x/photos/cat.jpg
```

---

## Combining operations

Operations are separated by `;` inside the filter segment.

```text
# Full example with all short aliases
/thumbs/400x300/f:c(50,30,350,230);fit(fill,black);fmt(webp);q(85)/photos/cat.jpg

# Same with full names
/thumbs/400x300/filters:crop(50,30,350,230);fit(fill,black);format(webp);quality(85)/photos/cat.jpg

# With signature and alias
/thumbs/a1b2c3d4e5f6g7h8/400x300/f:fmt(avif);q(90)/photos/cat.jpg/as/card.avif
```
