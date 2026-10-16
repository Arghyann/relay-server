# relay-tunnel

A fast, lightweight, dual-purpose networking tool and VPN written in Go. Connect to remote IPv6 and private services (PostgreSQL, Redis, SSH) via non-root port forwarding, or turn your machine into a full-tunnel, dual-stack Layer-3 VPN.

---

## Features

* **Two Modes of Operation**:
  * **Port Forwarding Mode**: Non-root TLS HTTP CONNECT tunnel for forwarding local TCP ports to remote IPv4 or IPv6 endpoints.
  * **VPN Mode (`-vpn`)**: Full-system Layer-3 ephemeral TUN device (`relay0`) with dual-stack IPv4 and IPv6 traffic redirection.
* **Smart Routing (/1 Trick)**:
  * Redirects internet traffic through `0.0.0.0/1` & `128.0.0.0/1` (IPv4) and `::/1` & `8000::/1` (IPv6).
  * Never touches or deletes your physical default gateway. Stopping the tunnel instantly restores native networking.
  * Automatic host bypass routing prevents TLS connection loops.
* **Concurrent Multi-Client Mesh**:
  * Dynamic IP allocation (`198.18.0.0/24` and `fd00:198:18::/64`) negotiated over HTTP headers.
  * Central kernel TUN packet dispatcher routes packets to the correct client based on destination IP.
  * Connected clients can communicate directly with each other over the private subnet.
* **Machine-Bound Encrypted Key Storage**:
  * Automatically saves your API key on first use encrypted with **AES-256-GCM** derived from `/etc/machine-id`.
  * Permissions locked to `0600` (user-only).
  * Respects `$SUDO_USER` so `sudo relay-tunnel -vpn` seamlessly shares the key with your non-root user.
  * Run once with `-key <KEY>` and never have to paste your key again.

---

## Installation

### Option 1: Using Go Install
```bash
go install github.com/Arghyann/relay-server/client@latest
sudo cp ~/go/bin/client /usr/local/bin/relay-tunnel
```

### Option 2: From Source
```bash
git clone https://github.com/Arghyann/relay-server.git
cd relay-server
go build -o ~/.local/bin/relay-tunnel ./client
```

---

## Quick Start

### 1. Save Your API Key (First Time Setup)
Pass your API key once with `-key`:
```bash
relay-tunnel -target "[2001:db8::1]:5432" -key "rk_367d344dc4f43454..."
```
This automatically encrypts and stores the key in `~/.config/relay/key.enc`. 
From now on, you can omit the `-key` flag completely!

---

## Mode 1: System-Level VPN (`-vpn`)

Creates a virtual network interface, claims a private dual-stack IP from the server, and routes all your internet traffic through the encrypted TLS tunnel.

```bash
sudo relay-tunnel -vpn
```

### What happens under the hood:
1. Performs TLS handshake and authenticates with your saved key.
2. Receives dynamically assigned IPv4 and IPv6 addresses (e.g. `198.18.0.2` & `fd00:198:18::2`).
3. Creates an ephemeral `relay0` TUN device.
4. Adds the server bypass route on your physical interface.
5. Installs the dual-stack `/1` routes redirecting all traffic through `relay0`.
6. Intercepts `Ctrl+C` (`SIGINT`) to cleanly flush routes and restore your native connection.

#### Verify VPN connection:
```bash
# Check IPv4 egress (matches VPS IPv4)
curl -4 https://icanhazip.com

# Check IPv6 egress (matches VPS IPv6)
curl -6 https://icanhazip.com

# Direct peer ping to another connected VPN client
ping 198.18.0.3
ping6 fd00:198:18::3
```

---

## Mode 2: Point-to-Point Port Forwarding (Non-Root)

Binds to a local port and forwards all connections across the relay to a remote target. Does **not** require root/sudo.

```bash
relay-tunnel -target "<TARGET_ADDRESS>:<PORT>" [-local <PORT>]
```

### Examples:

#### 1. PostgreSQL / Supabase
Forward port 5432:
```bash
relay-tunnel -target "db.your-project.supabase.co:5432" -local 5432
```
Connect your application or database GUI to `localhost`:
```bash
psql "postgresql://postgres:password@127.0.0.1:5432/postgres?sslmode=require"
```

#### 2. SSH into an IPv6-Only Machine
Forward port 2222:
```bash
relay-tunnel -target "[2406:da1a:5d5:...]:22" -local 2222
```
SSH directly into `localhost`:
```bash
ssh -i ~/.ssh/id_ed25519 user@127.0.0.1 -p 2222
```

#### 3. Redis / Cache
Forward port 6379:
```bash
relay-tunnel -target "[2001:db8:...]:6379" -local 6379
```
Connect using `redis-cli`:
```bash
redis-cli -p 6379
```

---

## Self-Hosting Guide (`relay-server`)

You can easily host your own `relay-server` on any Linux VPS (AWS, DigitalOcean, Hetzner, Linode, etc.).

### 1. Prerequisites
* **Linux server** with `root` privileges (required to create TUN devices and configure iptables forwarding).
* **Network tools**: Ensure `iptables`, `ip6tables`, and `iproute2` (`ip`) are installed:
  ```bash
  # Debian / Ubuntu
  sudo apt install -y iptables iproute2
  # Arch Linux
  sudo pacman -S iptables iproute2
  ```
* **Firewall**: Open port `28473/tcp` (or your chosen port):
  ```bash
  sudo ufw allow 28473/tcp
  ```

---

### 2. Build and Install

```bash
# Clone the repository
git clone https://github.com/Arghyann/relay-server.git
cd relay-server

# Build the server binary
go build -o relay-server ./server

# Install to /usr/local/bin
sudo install -m 755 relay-server /usr/local/bin/relay-server
```

---

### 3. Generate TLS Certificates

`relay-server` communicates over TLS and requires `cert.pem` and `key.pem` in its working directory (or where it is run):

```bash
mkdir -p /etc/relay-server
cd /etc/relay-server

# Generate self-signed TLS certificates (valid for 1 year)
sudo openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=relay-server"
sudo chmod 600 key.pem
```

---

### 4. Manage API Keys

API keys are stored in an embedded SQLite database (`relay.db`).

* **First Run**: When started for the first time, `relay-server` automatically creates `relay.db` and generates an initial `admin` API key printed to standard output.
* **Generate a new key for a client**:
  ```bash
  cd /etc/relay-server
  sudo relay-server -add-key "alice"
  # Output: Created API Key for [alice]: rk_367d344dc4f43454...
  ```
* **List all active keys**:
  ```bash
  cd /etc/relay-server
  sudo relay-server -list-keys
  ```

---

### 5. Run as a Systemd Service (Recommended)

Create a systemd unit file at `/etc/systemd/system/relay-server.service`:

```ini
[Unit]
Description=Relay Server Daemon
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/relay-server
ExecStart=/usr/local/bin/relay-server -port 28473
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now relay-server
sudo systemctl status relay-server
```

---

### 6. Connect Clients to Your Server

Pass your server's public IP/domain using the `-relay` flag:

#### Point-to-Point Forwarding:
```bash
relay-tunnel -relay "your-server-ip:28473" -target "db.internal:5432" -key "rk_..."
```

#### Dual-Stack VPN Mode:
```bash
sudo relay-tunnel -relay "your-server-ip:28473" -key "rk_..." -vpn
```

---

## CLI Reference

| Flag | Description | Default |
| :--- | :--- | :--- |
| **`-vpn`** | Run in Layer-3 VPN mode (requires `sudo`) | `false` |
| **`-key`** | API key (saved encrypted to disk on first run; optional thereafter) | `""` |
| **`-target`** | Target address and port for port-forwarding mode (e.g. `[2001:...]:5432`) | `""` |
| **`-local`** | Local port to listen on in port-forwarding mode | `5432` |
| **`-relay`** | Relay server endpoint (IPv4 or `[IPv6]:port`) | `13.207.62.103:28473` |

---

## Environment Variables

* **`RELAY_KEY`**: Provide the API key via environment variable instead of flags or disk storage (e.g. `export RELAY_KEY="rk_..."`).