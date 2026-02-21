# Signature Generation

When `SIGNATURE_SECRET` is configured, signed URLs are required.

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

- HMAC-SHA256 over payload string
- Hex-encode digest
- Use first 16 hex chars

## Node.js Example

```js
const crypto = require('crypto');

function signPayload(payload, secret) {
    const normalizedPayload = payload.startsWith('/') 
        ? payload 
        : `/${payload}`;

    return crypto
        .createHmac('sha256', secret)
        .update(normalizedPayload)
        .digest('hex')
        .slice(0, 16);
}

const secret = process.env.SIGNATURE_SECRET;
const payload = '/200x350/filters:quality(90)/path/to/image.jpg/as/card.avif';
const signature = signPayload(payload, secret);

const url = `/thumbs/${signature}${payload}`;
console.log(url);
```

## PHP Example

```php
<?php

function signPayload(string $payload, string $secret): string
{
    $normalizedPayload = str_starts_with($payload, '/') 
        ? $payload 
        : "/{$payload}";

    $hash = hash_hmac(
        'sha256', 
        $normalizedPayload, 
        $secret
    );

    return substr($hash, 0, 16);
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
