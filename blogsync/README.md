# Dropbox Sync

A Rust application that synchronizes files from a Dropbox app folder to local disk, responds to webhook notifications for real-time updates, and automatically builds static sites using Zola or Hugo.

## Features

- **OAuth Authentication**: Secure authentication with Dropbox using OAuth 2.0
- **Webhook Support**: Real-time file synchronization via Dropbox webhooks
- **Encrypted Token Storage**: Securely stores refresh tokens on disk with AES-256 encryption
- **Automatic Building**: Runs build commands (Zola/Hugo) after file synchronization
- **Flexible Copy Rules**: Configurable post-build file copying with glob patterns
- **CLI Interface**: Easy-to-use command-line interface

## Prerequisites

1. **Dropbox App**: Create a Dropbox app at https://www.dropbox.com/developers/apps
   - Choose "Scoped access"
   - Choose "App folder" access type
   - Note down your App key and App secret

2. **Rust**: Install Rust from https://rustup.rs/

## Installation

```bash
# Clone and build
git clone <repository>
cd dropbox-sync
cargo build --release
```

## Configuration

1. Generate a default configuration file:
```bash
./target/release/dropbox-sync init-config
```

2. Edit `config.toml` with your Dropbox app credentials:
```toml
[dropbox]
app_key = "your_actual_app_key"
app_secret = "your_actual_app_secret"
redirect_uri = "http://localhost:3000/auth/callback"

[server]
host = "0.0.0.0"
port = 3000
webhook_path = "/webhook"

[sync]
local_base_path = "./sync"
dropbox_folder = "/"

[build]
command = "zola build"
working_directory = "./sync"

[[copy_rules]]
source_pattern = "./sync/public/**/*"
destination = "./output"
recursive = true
```

## Usage

### 1. Authentication

First, authenticate with Dropbox:
```bash
./target/release/dropbox-sync auth
```

This will:
- Open a browser to Dropbox OAuth page
- Prompt you to paste the authorization code
- Securely store the refresh token

### 2. One-time Sync

Perform a manual synchronization:
```bash
./target/release/dropbox-sync sync
```

### 3. Start Webhook Server

Start the continuous sync service:
```bash
./target/release/dropbox-sync start
```

This will:
- Start a webhook server on the configured port
- Listen for Dropbox file change notifications
- Automatically sync files and run build commands

### 4. Configure Dropbox Webhook

In your Dropbox app settings, set the webhook URL to:
```
https://your-domain.com/webhook
```

For local testing, you can use tools like ngrok:
```bash
ngrok http 3000
# Use the generated URL + /webhook
```

## Environment Variables

- `DROPBOX_SYNC_PASSWORD`: Password for token encryption (alternative to CLI prompt)

## CLI Options

```bash
# Use custom config file
./target/release/dropbox-sync -c custom-config.toml start

# Provide password via CLI
./target/release/dropbox-sync -p mypassword auth
```

## Configuration Options

### Dropbox Section
- `app_key`: Your Dropbox app key
- `app_secret`: Your Dropbox app secret
- `redirect_uri`: OAuth redirect URI (must match Dropbox app settings)

### Server Section
- `host`: Server bind address
- `port`: Server port
- `webhook_path`: Webhook endpoint path

### Sync Section
- `local_base_path`: Local directory to sync files to
- `dropbox_folder`: Dropbox folder to sync from ("/" for app root)

### Build Section
- `command`: Build command to run after sync (e.g., "zola build", "hugo")
- `working_directory`: Directory to run build command in

### Copy Rules
Array of rules for copying files after build:
- `source_pattern`: Glob pattern for source files
- `destination`: Destination directory
- `recursive`: Whether to copy directories recursively

## Security

- Tokens are encrypted using AES-256-GCM with a password-derived key
- Tokens are stored in `~/.dropbox_sync/tokens.enc`
- Password can be provided via CLI, environment variable, or interactive prompt

## Docker Support

Create a `Dockerfile`:
```dockerfile
FROM rust:1.70 as builder
WORKDIR /app
COPY . .
RUN cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/dropbox-sync /usr/local/bin/
ENTRYPOINT ["dropbox-sync"]
```

Build and run:
```bash
docker build -t dropbox-sync .
docker run -d -p 3000:3000 -v ./config.toml:/config.toml -v ./sync:/sync dropbox-sync start
```

## Troubleshooting

### Authentication Issues
- Ensure redirect URI matches exactly between config and Dropbox app settings
- Check that app key and secret are correct
- Verify internet connectivity

### Webhook Issues
- Ensure webhook URL is publicly accessible
- Check Dropbox app webhook settings
- Verify webhook path matches configuration
- Check server logs for error messages

### Build Issues
- Ensure build command is correct and executable
- Check working directory exists and contains necessary files
- Verify build dependencies are installed

## License

[Your License Here]