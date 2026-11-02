[日本語](README.md) | [简体中文](README.zh-CN.md) | **English**

# Sleepy Go

A lightweight personal status page: it shows your current status (online / offline) and the devices and apps you are using right now, reported in real time by small clients or an iOS Shortcut. The server is written in Go and runs as a single binary with a single SQLite file.

## Features

- **Status page**: a quiet, spacious design showing your manual status, the devices, apps, battery and music in use, and the visitor's local time (device order is set in the admin page), with a light / dark toggle.
- **Japanese, Chinese, English**: pick the UI language with `page.language` (default `ja`).
- **Live updates**: Server-Sent Events with automatic reconnect; returning to the tab on a phone refreshes immediately.
- **Admin panel**: switch status, manage devices, privacy mode, and edit the page text by clicking it in a live preview (random lines and variables supported).
- **Single binary**: pages, styles, scripts, icons and time zone data are embedded; no Python, Node or frontend build needed.
- **Security options**: host allowlist, device-only secret, an option to reject secrets in URLs, and optional reverse proxy examples (Caddy / nginx).
- **Client compatibility**: `/api/…` endpoints plus the legacy routes used by original Sleepy clients (such as `/device/set`).

## One-command deploy (Docker)

On a Linux server with Docker (including the Compose plugin) and git:

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | bash
```

The script clones the repository into `~/sleepy-go`, generates a random secret, builds the image and starts it on port `9010`, then prints the URL and the secret.

**Optional: domain and HTTPS.** The script does not install a reverse proxy. If you want your own domain with HTTPS, set up Caddy or nginx yourself using the examples in [`deploy/examples/`](deploy/examples/), and let Sleepy listen on localhost only and accept that domain only:

```sh
curl -fsSL https://raw.githubusercontent.com/kisukri/sleepy-go/main/deploy/install.sh | LISTEN=127.0.0.1 ALLOWED_HOSTS=status.example.com bash
```

On an existing installation run `bash ~/sleepy-go/deploy/install.sh` with the same variables; the secret and settings are kept (`LISTEN=-` goes back to public, `ALLOWED_HOSTS=-` clears the allowlist).

| Variable | Default | Meaning |
| --- | --- | --- |
| `INSTALL_DIR` | `~/sleepy-go` | Install location |
| `PORT` | `9010` | Sleepy's port |
| `LISTEN` | all interfaces | `127.0.0.1` = localhost only (for your own reverse proxy) |
| `ALLOWED_HOSTS` | none | Allowed domains, comma separated |
| `SECRET` | generated | Secret for the admin login and device reports |
| `PAGE_NAME` | `Sleepy` | Name shown on the page |
| `PAGE_LANGUAGE` | `ja` | UI language (`ja` / `zh` / `en`); `zh` when migrating from the original |
| `OLD_DIR` | `~/sleepy` | Location of an original Sleepy install (see "Migrating from the original Sleepy") |
| `REMOVE_OLD` | `1` | Delete the old directory after a successful migration (a backup archive is made first); `0` keeps it |

**Update**: run the same script again (`git pull` → rebuild → restart, a few seconds of downtime):

```sh
bash ~/sleepy-go/deploy/install.sh
```

The secret is stored in `~/sleepy-go/docker-compose.yml` (mode 600, ignored by git); data lives in `~/sleepy-go/data/`.

## Security recommendations

1. Enable HTTPS with your own reverse proxy (examples in `deploy/examples/`). Without HTTPS the secret travels in plain text.
2. Set `main.device_secret` in `data/config.yaml` and give clients this device secret (it can only report devices); keep `main.secret` for the admin panel.
3. Once the Shortcut sends `Authorization: Bearer <device secret>` as a header, set `main.allow_query_secret: false`.

Details: [Configuration · Security](docs/configuration.md#セキュリティ) (Japanese).

## Migrating from the original Sleepy

If the server already runs the original [Sleepy](https://github.com/sleepy-project/sleepy) (Python) with Docker (in `~/sleepy` by default, or set `OLD_DIR=/path`), just run the one-command deploy. The script:

1. clones and builds the new version first while the old one keeps running;
2. carries over the secret, name and other `sleepy_` settings and the port from the old `docker-compose.yml`;
3. stops the old container, archives the whole old directory to `~/sleepy-python-backup-<time>.tgz` and copies `data/` (database, page text, config);
4. starts Sleepy Go and runs a health check: on failure it restores the old version automatically, on success it removes the old directory and image.

Clients need no changes (same secret, device IDs and URLs). Log in to the admin panel once more. For a manual migration see the [migration guide](docs/migration.md) (Japanese).

## Other ways to run

### Docker Compose (manual)

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
mkdir -p data
export SLEEPY_UID="$(id -u)" SLEEPY_GID="$(id -g)"
export SLEEPY_MAIN_SECRET='replace with a long random secret'
export SLEEPY_PAGE_NAME='Your name'
docker compose -f deploy/compose.yaml up --build -d
```

### From source

Requires **Go 1.25+** (`go.mod` pins the go1.27.3 toolchain, which older Go versions download automatically):

```sh
git clone https://github.com/kisukri/sleepy-go.git && cd sleepy-go
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/sleepy-go ./cmd/sleepy-go
export sleepy_main_secret='replace with a long random secret'
export sleepy_page_name='Your name'
./bin/sleepy-go
```

By default it listens on `127.0.0.1:9010` and stores data in `data/` under the current directory. Set `sleepy_main_host=0.0.0.0` to serve other machines, or use `-data-dir /path/to/data`. Reverse proxies and HTTPS: [operations](docs/operations.md) (Japanese).

## Configuration

Simple settings use `sleepy_` environment variables (for example `sleepy_main_secret`, `sleepy_page_name`). For everything else copy the example:

```sh
cp configs/config.example.yaml data/config.yaml
```

Precedence, low to high: defaults → `data/.env` / environment → `data/config.yaml` → `data/config.toml` → `data/config.json`. The secret must not be empty; a short one triggers a warning in the log. All fields: [configuration](docs/configuration.md) (Japanese).

## Clients and API

See [`clients/`](clients/README.md) (Japanese): on Windows the Go client is recommended (one exe, no Python; reports window, idle, battery and sends heartbeats); there are also Python and PowerShell clients and Linux (KDE, Hyprland) scripts; iOS/iPadOS can use a Shortcut. Configure the server URL, secret, device ID and display name to start reporting.

New clients should POST JSON to `/api/device/set` with `Authorization: Bearer <secret>`. See the [API reference](docs/api.md) (Japanese).

For the clients below, "secret" means `main.device_secret` if you set one, otherwise `main.secret` (the one you log in to the admin page with).

### Windows client

1. Open the [Go workflow under Actions](https://github.com/kisukri/sleepy-go/actions/workflows/go.yml), pick the latest successful run and download `sleepy-client-windows` (you need to be signed in to GitHub). Unzip it and use `sleepy-client.exe`, or `sleepy-client-arm64.exe` on ARM Windows.
2. Put the exe in a folder where it will stay and double-click it once. It writes `sleepy-client.toml` next to itself and exits.
3. Open `sleepy-client.toml` in Notepad and fill in:

   ```toml
   server = "https://status.example.com"
   secret = "your secret"
   device_id = "my-pc"
   device_name = "My PC"
   ```

4. Double-click it again to start reporting. It has no window; check `sleepy-client.log` in the same folder to see what it is doing.
5. Start with Windows: press `Win + R`, open `shell:startup` and put a shortcut to the exe there.

The exe is unsigned, so SmartScreen may stop it the first time; choose "More info" → "Run anyway". Other options are in [clients/](clients/README.md#sleepy-clientgo-版) (Japanese).

### iOS Shortcut

Needs iOS 18 or later. Create a new shortcut in the Shortcuts app and add these actions in order:

1. **Get Current App**
2. **Get Battery Level** (skip it if you don't want the battery shown)
3. **Text**: `[`, the battery level, `%]` and the app's Name, e.g. `[87%]Safari`. The page shows the percentage in brackets as the battery level; `[🔋87%]` works too, and `[87%⚡]` marks it as charging.
4. **Get Contents of URL**:
   - URL: `https://status.example.com/api/device/set`
   - Method: `POST`
   - Headers: `Authorization` with the value `Bearer your-secret`
   - Request Body: JSON

   | Key | Type | Value |
   | --- | --- | --- |
   | `id` | Text | `iphone` (a unique ID for the device) |
   | `show_name` | Text | `iPhone` (the name shown on the page) |
   | `using` | Boolean | on |
   | `status` | Text | the Text from step 3 |

Run it once by hand; if the device shows up on the page, it works. Then go to Automation → App, select all apps, choose "Is Opened" and "Run Immediately", and pick this shortcut so it reports every time you open an app.

Use `https://`. Over `http://` the secret is sent in plain text, and when a reverse proxy redirects to HTTPS the POST can turn into a GET or lose its headers.

## Layout

```text
.
├── cmd/sleepy-go/          # server entry point
├── cmd/sleepy-client/      # Go client for Windows
├── internal/sleepy/        # config, HTTP API, auth, SQLite, SSE, page text, tests
├── web/                    # embedded pages, styles, scripts and the text editor
├── clients/                # other clients (Python, PowerShell, shell)
├── configs/                # example configuration
├── deploy/                 # install script, Dockerfile, Compose and reverse proxy examples
└── docs/                   # configuration, API, operations, migration (Japanese)
```

## Development

```sh
go test -race ./...
go vet ./...
```

## License

[MIT License](LICENSE) © Kisukri. The client scripts in `clients/` come from [sleepy-project/sleepy](https://github.com/sleepy-project/sleepy) and keep their original MIT license ([clients/LICENSE](clients/LICENSE)). Thanks to the original authors and client contributors. The bundled Newsreader font is under the SIL Open Font License 1.1 ([web/static/fonts/OFL.txt](web/static/fonts/OFL.txt)).
