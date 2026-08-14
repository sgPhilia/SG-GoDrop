# SG‑GoDrop

**Fast, simple LAN file transfer – no cloud, no account, just drop.**

SG‑GoDrop is a lightweight local network file transfer system written in Go. It lets you share files between any device on the same network using only a web browser.

- **No cloud** – files never leave your network.
- **No account** – start the server and go.
- **No installation** – clients only need a browser.
- **Temporary** – files are deleted when the server stops.
- **Cross‑platform** – runs on Linux, Windows, macOS.

---

## Features

- **Web‑based UI** with drag‑and‑drop uploads
- **Concurrent transfers** – multiple clients at once
- **Streaming I/O** – handles large files without extra memory
- **Instant connection** – QR code + copyable LAN URL
- **Progress tracking** – see upload/download status in real‑time
- **Simple API** – for automation or integration
- **Zero configuration** – run `godrop` and you're ready

---

## Installation

### From source (requires Go 1.23+)

```bash
git clone https://github.com/yourusername/SG-GoDrop.git
cd SG-GoDrop
go mod download
go build -o godrop ./cmd/godrop
```
### Pre‑built binaries
Download the latest release for your platform from the Releases page.

Usage
Start the server:

```bash
./godrop
```
You'll see:

```text
GoDrop is running!
Local:   http://localhost:8080
LAN:     http://192.168.1.109:8080
Waiting for connections...
```
Open the displayed LAN URL from any other device on the same network.

### Options
| Flag   | Default     | Description                 |
|--------|-------------|-----------------------------|
| --host | 0.0.0.0     | IP to bind to               |
| --port | 8080        | Port to listen on           |
| --dir  | OS temp dir | Temporary storage directory |

Example:

```bash
godrop --port 9000 --dir /mnt/ramdisk
```
Web Interface
- Connection info – the LAN URL is shown with a Copy Link button.
- QR code – scan with your phone camera to instantly open the page.
- Upload – drag & drop files or click the choose button.
- Progress – each transfer shows a progress bar.
- Download – click the download button for completed transfers.
- Delete – remove transfers manually (or they are cleaned up on shutdown).

### API Reference
All endpoints are under /api/:

- GET /api/info – server metadata (name, URL, host, port)
- Example: {"name":"GoDrop","url":"http://192.168.1.109:8080","host":"0.0.0.0","port":8080}
- GET /api/qr – returns a PNG QR code encoding the LAN URL.
- POST /api/upload – upload a file (multipart form, field file).
- Response: {"id":"...","name":"...","size":...}
- GET /api/transfers – list all transfers.
-Response: {"transfers":[{"ID":"...","Name":"...","Size":...,"Status":"completed",...}]}
- GET /api/transfers/:id – get details of a specific transfer.
- GET /api/transfers/:id/download – download a completed transfer.
- DELETE /api/transfers/:id – delete a transfer.

### Development
#### Project Structure
```text
SG-GoDrop/
├── cmd/
│   └── godrop/
│       └── main.go               # entry point
├── internal/
│   ├── config/                   # configuration
│   ├── logger/                   # structured logging
│   ├── server/                   # HTTP server, routes, handlers
│   │   └── web/                  # embedded HTML/CSS/JS
│   └── transfer/                 # transfer manager (core logic)
├── tests/
│   └── integration_test.go       # end‑to‑end tests
├── .gitignore
├── go.mod
├── go.sum
└── godrop                        # built binary (optional)
```

### Testing
Run all tests:

```bash
go test ./...
```

With race detection:
```bash
go test -race ./...
```

Build
```bash
go build ./cmd/godrop
```
Cross‑compile
Example for Linux ARM64:

```bash
GOOS=linux GOARCH=arm64 go build -o godrop-linux-arm64 ./cmd/godrop
```