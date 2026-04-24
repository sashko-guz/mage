# Systemd Service Examples

## Setup

1. Create user and directories:

```bash
sudo useradd -r -s /bin/false mage
sudo mkdir -p /opt/mage /var/cache/mage/sources /var/cache/mage/thumbs
sudo chown -R mage:mage /var/cache/mage
```

2. Build and install binary:

```bash
go build -o mage ./cmd/mage
sudo cp mage /usr/local/bin/
sudo chmod +x /usr/local/bin/mage
```

3. Create config:

```bash
sudo cp .env.example /opt/mage/.env
sudo chown mage:mage /opt/mage/.env
sudo chmod 600 /opt/mage/.env
# Edit /opt/mage/.env with your settings
```

4. Install service:

```bash
# For local storage:
sudo cp examples/systemd/mage-local.service.example /etc/systemd/system/mage.service

# Or for S3 storage:
sudo cp examples/systemd/mage-s3.service.example /etc/systemd/system/mage.service

# Edit service file as needed
sudo vim /etc/systemd/system/mage.service
```

5. Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable mage
sudo systemctl start mage
```

## Commands

```bash
sudo systemctl status mage    # Check status
sudo systemctl restart mage   # Restart
sudo journalctl -u mage -f    # View logs
```

## Notes

- `EnvironmentFile=-/opt/mage/.env` - the `-` prefix means ignore if file doesn't exist
- Secrets (S3 keys, signature secret) should go in `.env` file, not the service file
- Adjust `MemoryMax` based on your cache settings and expected load
- `ReadWritePaths` must include cache directories
