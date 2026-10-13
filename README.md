# relay

A lightweight, TLS-encrypted HTTP CONNECT tunnel and TCP relay with SQLite API key authentication. Designed to forward ports to IPv6-only servers (databases, SSH, etc.) from IPv4 networks.

---

## Server

### 1. Generate Certificates
Run once inside the `server/` directory:
```bash
cd server
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

### 2. Run the Server
```bash
go run ./server
```
*Listens on port `28473` by default. Automatically generates an initial `admin` API key in `relay.db` on first run.*

### 3. Key Management
```bash
# Add a key for a user
./relay-server -add-key "alice"

# List all keys and statuses
./relay-server -list-keys
```

---

## Client

### 1. Install CLI Tool
```bash
go build -o ~/.local/bin/relay-tunnel ./client
```

### 2. Run a Tunnel
```bash
relay-tunnel \
  -relay "<SERVER_IP>:28473" \
  -target "[<IPV6_TARGET>]:<TARGET_PORT>" \
  -local <LOCAL_PORT> \
  -key "<API_KEY>"
```

---

## Examples

### PostgreSQL (e.g., Supabase)
```bash
# 1. Start tunnel
relay-tunnel -target "db.your-id.supabase.co:5432" -local 5432 -key "rk_..."

# 2. Connect
psql "postgresql://postgres:password@127.0.0.1:5432/postgres?sslmode=require"
```

### SSH
```bash
# 1. Start tunnel
relay-tunnel -target "[2406:da1a:...]:22" -local 2222 -key "rk_..."

# 2. Connect
ssh -i ~/.ssh/key user@127.0.0.1 -p 2222
```