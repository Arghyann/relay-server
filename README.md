# relay-tunnel

A secure, TLS-encrypted tunnel client to connect to remote IPv6 and private services (PostgreSQL, Redis, SSH, etc.) from any local machine.

---

## Installation

### Option 1: Using Go Install
```bash
go install github.com/Arghyann/relay-server.git/client@latest
```
*(Installs directly to `~/go/bin/client`)*

### Option 2: From Source
Clone the repository and build:
```bash
git clone https://github.com/Arghyann/relay-server.git
cd relay-server
go build -o ~/.local/bin/relay-tunnel ./client
```

---

## Quick Start

To open a tunnel, you need:
1. The **target address** you want to reach (e.g. `db.example.supabase.co:5432` or `[2001:...]:22`)
2. Your assigned **API Key** (`rk_...`)

```bash
relay-tunnel -target "<TARGET_ADDRESS>:<PORT>" -key "<YOUR_API_KEY>"
```

By default, this binds locally to port `5432`.

---

## CLI Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| **`-target`** *(required)* | Remote target address and port (e.g. `[2001:...]:22`) | `""` |
| **`-key`** *(required)* | Your personal API key | `""` |
| **`-local`** | Local port to listen on your machine | `5432` |
| **`-relay`** | Relay server endpoint | `13.207.62.103:28473` |

---

## Examples

### 1. PostgreSQL / Supabase
Start the tunnel on your machine:
```bash
relay-tunnel -target "db.your-project.supabase.co:5432" -local 5432 -key "rk_..."
```

Connect your application or database GUI to `localhost`:
```bash
psql "postgresql://postgres:password@127.0.0.1:5432/postgres?sslmode=require"
```

---

### 2. SSH into an IPv6 Machine
Start the tunnel on port `2222`:
```bash
relay-tunnel -target "[2406:da1a:...]:22" -local 2222 -key "rk_..."
```

SSH directly into `localhost`:
```bash
ssh -i ~/.ssh/id_ed25519 user@127.0.0.1 -p 2222
```

---

### 3. Redis / Cache
Start the tunnel on port `6379`:
```bash
relay-tunnel -target "[2001:db8:...]:6379" -local 6379 -key "rk_..."
```

Connect using `redis-cli`:
```bash
redis-cli -p 6379
```