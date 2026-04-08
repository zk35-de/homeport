# Docker Discovery: Socket Proxy Setup & Debugging

homeport discovers Docker containers via HTTP (`GET /containers/json`). It does **not** mount the Docker socket directly — instead you point it at an HTTP endpoint.

The recommended approach is a **socket proxy** that exposes only the containers endpoint (read-only, no daemon access).

---

## Setup: Socket Proxy

Use [`tecnativa/docker-socket-proxy`](https://github.com/Tecnativa/docker-socket-proxy) or the equivalent `lscr.io/linuxserver/socket-proxy`.

### docker-compose.yml (same host as homeport)

```yaml
services:
  socket-proxy:
    image: tecnativa/docker-socket-proxy:latest
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      CONTAINERS: 1   # allow GET /containers/json
      # everything else defaults to 0 (denied)
    ports:
      - "127.0.0.1:2375:2375"   # only localhost — never bind to 0.0.0.0

  homeport:
    image: ghcr.io/zk35-de/homeport:latest
    ports:
      - "8855:8855"
    volumes:
      - homeport-data:/app/data
    environment:
      - HOMEPORT_PORT=8855
    restart: unless-stopped

volumes:
  homeport-data:
```

> Bind to `127.0.0.1:2375`, not `0.0.0.0:2375`. The proxy endpoint must not be reachable from the network — only homeport needs it.

### homeport configuration

In the homeport manage UI → **Discovery** → **Add Source**:

| Field | Value |
|-------|-------|
| Type  | Docker |
| URL   | `http://127.0.0.1:2375` |

If homeport runs in a separate container (not host network), use the service name:

| Field | Value |
|-------|-------|
| URL   | `http://socket-proxy:2375` |

### Remote host (Docker on another machine)

If you want to discover containers on a **different host**, run the socket proxy there and expose it on a private/VLAN interface:

```yaml
# on the remote host
services:
  socket-proxy:
    image: tecnativa/docker-socket-proxy:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      CONTAINERS: 1
    ports:
      - "192.168.10.5:2375:2375"   # VLAN IP, not 0.0.0.0
```

homeport source URL: `http://192.168.10.5:2375`

---

## Debugging: Discovery Not Working

Work through these steps in order.

### 1. Test the proxy endpoint directly

```bash
curl -s http://127.0.0.1:2375/containers/json | jq '.[].Names'
```

Expected: JSON array with container names.

- **Connection refused** → socket proxy not running or wrong port
- **Empty array `[]`** → no running containers, or Docker socket path wrong
- **403 / `{"message":"Forbidden"}` on /containers/json** → `CONTAINERS: 1` not set in proxy env

### 2. Verify the socket proxy can see the Docker socket

```bash
docker compose exec socket-proxy ls -la /var/run/docker.sock
```

Expected: `srw-rw---- ... /var/run/docker.sock`

If missing: the volume mount is wrong. On some systems the socket is at `/run/docker.sock` — adjust the `volumes` entry.

### 3. Check what homeport sends

Check the homeport logs for discovery errors:

```bash
# Docker Compose
docker compose logs homeport | grep -i discover

# Podman / systemd
journalctl --user -u homeport -n 50 | grep -i discover
```

Common log messages and their meaning:

| Message | Cause |
|---------|-------|
| `docker fetch: dial tcp ... connection refused` | Proxy not reachable at configured URL |
| `docker containers/json: status 403` | `CONTAINERS` env not set on proxy |
| `docker decode: ...` | Proxy returned unexpected response (check with curl) |

### 4. Containers show up but are not imported

homeport skips containers with no reachable URL. A container is included when it has either:
- a `homeport.url` label, **or**
- at least one mapped TCP port (`PublicPort > 0`)

Check your containers:
```bash
docker ps --format '{{.Names}}\t{{.Ports}}'
```

Containers with no port mapping (e.g., internal services) need a label:
```yaml
labels:
  homeport.url: "http://192.168.1.10:8080"
  homeport.name: "My Service"
```

### 5. Socket proxy reports 0.0.0.0 as IP → wrong URL built

When a container maps a port to `0.0.0.0`, homeport uses the **hostname from the proxy URL** as the service host. If that is `127.0.0.1` but the service is only reachable on a different IP, override with the label:

```yaml
labels:
  homeport.url: "http://192.168.1.10:8080"
```

### 6. Podman: socket path differs

Podman uses a user socket, not `/var/run/docker.sock`:

```bash
ls $XDG_RUNTIME_DIR/podman/podman.sock
# or
ls /run/user/$(id -u)/podman/podman.sock
```

Mount that path in the socket proxy:
```yaml
volumes:
  - /run/user/1000/podman/podman.sock:/var/run/docker.sock:ro
```

Or use Podman's Docker-compatible socket (`podman system service --time=0`).

---

## Limitations

### macvlan networks

Containers that run in a **macvlan network** (own IP, no port mappings) are **not auto-discovered**. Docker reports `Ports: []` for these containers — homeport has no way to derive the service URL automatically.

Workaround: set `homeport.url` explicitly on each macvlan container:

```yaml
labels:
  homeport.url: "http://10.35.135.130:80"
  homeport.name: "Vaultwarden"
```

If all your macvlan services sit behind a reverse proxy (NPM, Traefik), use the corresponding **NPM or Traefik discovery source** instead — those know the public URLs.

---

## Container Labels Reference

| Label | Description |
|-------|-------------|
| `homeport.name` | Display name (default: container name) |
| `homeport.url` | Service URL (default: derived from first mapped TCP port) |
| `homeport.description` | Short description shown in discovery inbox |
| `homeport.icon` | Icon name (prism-ui icon set) |

Example:
```yaml
services:
  myapp:
    image: myapp:latest
    ports:
      - "8080:8080"
    labels:
      homeport.name: "My App"
      homeport.url: "http://192.168.1.10:8080"
      homeport.description: "Internal dashboard"
```
