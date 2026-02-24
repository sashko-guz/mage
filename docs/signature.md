# Signature Generation

When `SIGNATURE_SECRET` is configured, signed URLs are required.

Signature behavior is configurable via env:

- `SIGNATURE_ALGO`: `sha256` or `sha512` (default: `sha256`)
- `SIGNATURE_EXTRACT_START`: digest extraction start offset (default: `0`)
- `SIGNATURE_LENGTH`: extracted signature length (default: `16`)

## Payload Format

The signature is generated from the canonical payload path:

```text
/{size}/[filters:{filters}/]{path}[/as/{alias.ext}]
```

Examples of payloads:

```text
/200x350/path/to/image.jpg
/200x350/filters:format(avif);quality(90)/path/to/image.jpg
/200x350/path/to/image.jpg/as/card.avif
```

Signature algorithm:

- HMAC over payload string using configured algorithm (`sha256` or `sha512`)
- Base64 RawURL-encode digest
- Extract signature using configured range:
    - start = `SIGNATURE_EXTRACT_START`
    - length = `SIGNATURE_LENGTH`

Default extraction uses first 16 base64 characters (start `0`, length `16`).

## Node.js Example

```js
const crypto = require('crypto');

function signPayload(payload, secret) {
    const normalizedPayload = payload.startsWith('/') 
        ? payload 
        : `/${payload}`;

    const algo = process.env.SIGNATURE_ALGO || 'sha256';
    const start = Number(process.env.SIGNATURE_EXTRACT_START || '0');
    const length = Number(process.env.SIGNATURE_LENGTH || '16');

    const hash = crypto
        .createHmac(algo, secret)
        .update(normalizedPayload)
        .digest();

    const base64Signature = Buffer.from(hash)
        .toString('base64url');

    return base64Signature.slice(
        start,
        start + length,
    );
}

const secret = process.env.SIGNATURE_SECRET;
const payload = '/200x350/filters:quality(90)/path/to/image.jpg/as/card.avif';
const signature = signPayload(payload, secret);

const url = `/thumbs/${signature}${payload}`;

console.info(url);
```

## PHP Example

```php
<?php

function signPayload(string $payload, string $secret): string
{
    $normalizedPayload = str_starts_with($payload, '/') 
        ? $payload 
        : "/{$payload}";

    $algo = getenv('SIGNATURE_ALGO') ?: 'sha256';
    $start = (int)(getenv('SIGNATURE_EXTRACT_START') ?: 0);
    $length = (int)(getenv('SIGNATURE_LENGTH') ?: 16);

    $hash = hash_hmac(
        $algo,
        $normalizedPayload,
        $secret,
        true
    );

    $encodedHash = base64_encode($hash);

    $replacedHash = strtr(
        $encodedHash,
        '+/',
        '-_',
    );

    $base64Signature = rtrim($replacedHash, '=');

    return substr(
        $base64Signature,
        $start,
        $length
    );
}

$secret = getenv('SIGNATURE_SECRET');
$payload = '/200x350/filters:quality(90)/path/to/image.jpg/as/card.avif';
$signature = signPayload($payload, $secret);

$url = "/thumbs/{$signature}{$payload}";

echo $url . PHP_EOL;
```

## Notes

- The payload must match URL path exactly (including filter order and alias segment if present).
- Any payload change requires recalculating signature.
- Unsigned URLs are allowed only when `SIGNATURE_SECRET` is empty on server.
