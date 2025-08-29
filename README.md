# Homelab Docker Setup Project

> **Status — Aug 27 2025 (America/Phoenix)**
> Objective: a secure, stable, production‑like home‑lab on **Proxmox VE** with **no WAN port‑forwards**, private local DNS, central HTTPS, and clear, reproducible configs.

---

## 0) High‑level roadmap

**Today (1–2 hours)**

- Lock down OPNsense access (non‑root admin, TOTP, GUI/SSH rules).
- Keep **Suricata** on **WAN** in **alert‑only** to verify it’s healthy.
- Start swapping Traefik BasicAuth → **SSO (Authentik)**; leave **Cloudflare Access** on public hostnames.
- Turn on Traefik access logs (JSON) and Prometheus metrics.

**This week**

- Add **CrowdSec** + Traefik plugin (virtual patching + reputation).
- Put **Portainer** behind a **Docker Socket Proxy**.
- Enable **OPNsense backups** (Nextcloud/Git) and basic monitoring alerts.

**Soon / when you have hardware**

- Stage **HA** notes (CARP + pfsync) for a future second firewall.
- **Extend Suricata to LAN** for visibility; once tuned, decide whether to run LAN inline as well.

---

## Project Goal & Core Services

### Hosts/VMs

- **OPNsense** firewall / router (virtualized on Proxmox)
- **Ubuntu “docker‑host”** VM (application services via Docker Compose)

### Key services

| Category      | Service                      | Purpose                                                               |
| ------------- | ---------------------------- | --------------------------------------------------------------------- |
| Network core  | **Pi‑hole**                  | DHCPv4, stateful DHCPv6, DNS filtering                                |
| Ingress       | **Traefik (v3)**             | Reverse‑proxy, ACME (DNS‑01), TLS, middleware chains, TCP passthrough |
| Mgmt/UI       | **Portainer**                | Docker stack management                                               |
| Secure access | **Cloudflared Tunnel**       | Edge → tunnel → Traefik (publish without opening WAN ports)           |
| VPN           | **WireGuard**, **Tailscale** | Private access to admin UIs & non‑HTTP protocols                      |
| Monitoring    | Uptime Kuma                  | URL/ICMP checks *(ready to launch)*                                   |
| Metrics       | Prometheus                   | Metrics store *(ready to launch)*                                     |
| Dashboards    | Grafana                      | Visualization *(ready to launch)*                                     |

---

## Hardware & Network

- **Proxmox host**: Intel i5‑9600KF · 32 GB · 240 GB SSD (OS) · HDDs for VMs/ISOs
- **Path**: Cable modem (bridge) → **OPNsense VM** → Netgear AX5400 AP → LAN
- **LAN**: `192.168.1.0/24`, `fd2b:06d4:c6af:cafe::/64`
- **OPNsense** `192.168.1.1` / `fd…::1`

- **Unbound** recursive with DNSSEC; RA **Managed** (M=1, O=1); **DHCP off** (Pi‑hole serves)
- NAT rule **redirects all LAN :53** to Pi‑hole
- **docker‑host** (Ubuntu): static `192.168.1.3` / `fd…::3`, Docker Engine + Compose

---

## Configuration Backups — Current (local) & Target (Git→Gitea, Nextcloud)

- **Current (interim):** After any change, create an **encrypted local backup**:
  *OPNsense → System → Configuration → Backups → Backup* → Area **All** + **Encrypt**, download.
  Keep **3 copies** (e.g., laptop, NAS, USB).
- **Target A — Git history (diffs):** Enable the **Git backup** plugin to a **private GitHub repo** now, then migrate to **Gitea** later (mirror).
  *Note:* Git stores `config.xml` **plaintext** in the repo; keep it private and restrict access.
- **Target B — Encrypted cloud copy:** Add **Nextcloud backup** once Nextcloud is online (use an app password + encryption password).
- **Privilege note:** If using a **non-root** admin, ensure the group has the **Backups** page privileges; otherwise those sections won’t appear.

---

## DNS, VPN, and Access Posture

- **Local split‑horizon**: Pi‑hole **Local DNS Records** map `*.lablabland.com → 192.168.1.3`, keeping on‑LAN traffic local (no tunnel hairpin).
- **WireGuard**: split and full/exit profiles are working; outbound NAT + interface rules present in OPNsense.
- **Tailscale**: host install available for private access to UIs and non‑HTTP services.
- **Public exposure (when needed)**: Cloudflare Tunnel → Traefik. Gate sensitive hostnames with **Cloudflare Access** (SSO/MFA).

---

## Traefik (v3) — Wiring & Policy

**Static (`core/traefik/config/traefik.yaml`)**

- `entryPoints.web` → **redirect** to `websecure`
- `entryPoints.websecure` → TLS via ACME **cloudflare** resolver; TLS options **`default`**
- **Forwarded headers**: `insecure: false`; `trustedIPs` limited to the **cloudflared** container IPs on the `proxy` network (preserves real client IPs via tunnel; direct LAN/WG/TS hits remain correct)
- **Providers**: Docker (`exposedByDefault=false`, `network=proxy`) + File (`/etc/traefik/dynamic_conf`, watch enabled)
- **ACME (DNS‑01)**: Cloudflare provider with public resolvers; certs in `/letsencrypt/acme.json`

**Dynamic (`core/traefik/config/dynamic_conf/config.yaml`)**

- **Middlewares**
  - `secHeaders` (HSTS, nosniff, frame‑deny, referrer/permissions policy, etc.)
  - `basic-auth` (temporary; swap to **Authentik** soon)
  - *(planned)* `authentik-forwardauth` (ForwardAuth outpost)
  - *(optional)* `small-ratelimit` (attach to login/admin UIs)
  - Allow‑lists: `allow-lan`, `allow-vpn`, and combined **`allow-private`** (LAN + WireGuard + Tailscale)
- **Chains** *(pick one per admin UI)*
  - `default-public` → headers only
  - `default-public-auth` → headers + **SSO** (Authentik) or BasicAuth (temporary)
  - `default-private` → headers + allow‑private
  - **`default-private-auth`** → headers + allow‑private + **SSO** *(target)*
- **Routers & services**
  - **Pi‑hole UI** `dns01` → `Host("dns01.lablabland.com")` on `websecure`, chain **default-private-auth**, service **pihole** → `http://192.168.1.3:8080`
    *(Pi‑hole uses `network_mode: host`; the file‑provider service handles this.)*
  - **Proxmox TCP passthrough (443)** → `tcp.routers.proxmox` on **`websecure`** with `HostSNI("proxmox.lablabland.com" || "pve.lablabland.com")`, `tls.passthrough=true`, service → `192.168.1.2:8006`

**IP strategy**

- **Default:** no `ipStrategy` so the socket’s remote IP is authoritative (ideal for LAN/WG/TS).
- **If later gating Cloudflared client IPs:** add to the specific allow‑list middleware:

```yaml
ipStrategy:
  depth: 1
  excludedIPs: [127.0.0.1/32, ::1/128, 172.20.0.0/16, fd00:d0cc:0:1::/64]
```

### Dashboard

- Traefik’s dashboard router: `traefik.lablabland.com` on `websecure`, chain **default-private-auth**, service `api@internal`.

### Observability (Traefik)

- Enable **access logs (JSON)** with rotation for auditability.
- Expose **Prometheus metrics** and import a Traefik dashboard in Grafana.

### Time zone

- Container `TZ=${TZ}` and `/etc/localtime` bind‑mounted → logs in local time (`‑07:00`).

---

## Cloudflared — Locally Managed

- Config: `core/cloudflared/config/config.yaml` + credentials JSON
  **Tunnel ID**: `fb9833df-062c-4055-97e1-d828eec27cc0`
- `originRequest.noTLSVerify: true` (Traefik cert SNI is set to your public hostname)
  `originRequest.originServerName: traefik.lablabland.com`
- Typical ingress:

```yaml
- hostname: pterodactyl.lablabland.com
  service: https://traefik:443
  originRequest:
    httpHostHeader: pterodactyl.lablabland.com
- service: http_status:404   # last rule
```

- DNS: create **proxied** CNAMEs pointing to `<UUID>.cfargotunnel.com`.
- Note: `secrets/tunnel_token.secret` remains only for legacy methods; remove when convenient.

---

## Portainer — Current Use & Future Direction

- Routed via Traefik (`portainer.lablabland.com`) with **private allow‑list** + **BasicAuth** chain.
- **Preferred backend**: **HTTP :9000** behind Traefik TLS termination to avoid self‑signed verification issues.
  *(If using :9443, provision a valid internal certificate; avoid skip‑verify transports.)*
- **Planned**: adopt **`tecnativa/docker-socket-proxy`** to minimize Docker API exposure. Portainer itself remains part of the stack; the goal is to retire any “insecure transport” or "skip-verify" patterns as we move to the proxy.

---

## Pi‑hole — DHCP/DNS + Reverse‑Proxied UI

- Image `jacklul/pihole:latest`, **`network_mode: host`** for DHCP/DNS.
- Upstream DNS → **OPNsense Unbound**; DHCPv4 and stateful DHCPv6 enabled.
- UI is published via Traefik **file‑provider** service to `http://192.168.1.3:8080`.

---

## Domains & Access Patterns

- **Local**: `*.lablabland.com → 192.168.1.3` via Pi‑hole Local DNS Records (no tunnel hairpin).

- **Remote**: Cloudflare Tunnel → Traefik; add **Cloudflare Access** (Zero Trust) if the UI is sensitive.

- **Proxmox**:

- Through Traefik: `https://proxmox.lablabland.com` (**no port needed**, SNI passthrough on 443)

- Direct: `https://<proxmox-ip>:8006`

---

## **IDS/IPS architecture (decided)** — Suricata on **WAN + LAN**

> Suricata runs as a single engine in OPNsense. **IPS mode is global**: if IPS is enabled, all selected interfaces run inline.
> Recommended approach:
>
> - **Phase 1 (baseline):** select **WAN** only, run **IPS inline**; settle policies.
> - **Phase 2 (tuning):** temporarily disable IPS, add **LAN** to gather alerts; disable noisy SIDs or add passes.
> - **Phase 3 (optional):** re‑enable IPS with **LAN** still selected if you want inline on LAN; otherwise unselect LAN and keep only WAN inline.

### Suricata — operating pattern

- **Settings → Interfaces:**

- Phase 1: **WAN** selected

- Phase 2/3: **WAN + LAN** selected (per the approach above)

- **IPS mode:** on for enforcement phases, off during LAN‑tuning phases

- **Pattern matcher:** Hyperscan

- **EVE/syslog:** enabled

**Feeds to keep enabled** (then **Download & Update rules**):

- Enabled sets (current): `abuse.ch.sslblacklist.rules`, `abuse.ch.threatfox.rules`, `abuse.ch.urlhaus.rules`, `emerging-dns.rules`, `emerging-dos.rules`, `emerging-exploit.rules`, `emerging-info.rules`, `emerging-ja3.rules`, `emerging-malware.rules`, `emerging-phishing.rules`, `emerging-policy.rules`, `emerging-scan.rules`, `emerging-user_agents.rules`, `emerging-web_client.rules`, `tor.rules`

**Policies** (examples; adjust as you tune):

- `P20_ALERT_DETECTION` → **Alert**
  `abuse.ch.sslblacklist.rules, abuse.ch.threatfox.rules, abuse.ch.urlhaus.rules, emerging-dns.rules, emerging-dos.rules, emerging-exploit.rules, emerging-info.rules, emerging-ja3.rules, emerging-malware.rules, emerging-phishing.rules, emerging-policy.rules, emerging-scan.rules, emerging-user_agents.rules, emerging-web_client.rules, tor.rules`
- `P05_NOISY_DISABLE` → **Disable/Drop** known‑noisy SIDs you identify
  `emerging-ja3.rules`
- `P01_APP_DETECT_TEST` → **Drop** (temporary plumbing test)
  `opnsense.test.rules`

**Verification notes**

- HTTP EICAR blocks by content signature. Over **HTTPS**, payload is encrypted so content rules won’t match. Use DNS/category controls, SNI/JA3, and reputation lists for TLS flows.

---

## Compose Layout (current)

```bash
.
├── archived/
│   ├── pihole/
│   └── portainer/
├── core/
│   ├── cloudflared/
│   │   ├── config/{cert.pem,config.yaml,fb9833df-062c-4055-97e1-d828eec27cc0.json}
│   │   ├── docker-compose.yaml
│   │   └── secrets/tunnel_token.secret       # legacy
│   ├── pihole/{docker-compose.yaml,etc-dnsmasq.d/06-dhcpv6.conf,secrets/...}
│   ├── portainer/docker-compose.yaml
│   ├── tailscale/{docker-compose.yaml,data/,secrets/ts_authkey.secret}
│   └── traefik/
│       ├── config/{dynamic_conf/config.yaml,traefik.yaml}
│       ├── docker-compose.yaml
│       └── secrets/cloudflare_api_token.secret
├── data/gitea/docker-compose.yaml
├── docs/gaming/Gaming-Pterodactyl.md
├── gaming/pterodactyl/{panel/...,wings/...}
├── media/jellyfin/docker-compose.yaml
├── monitoring/{grafana,homepage,prometheus,uptime_kuma}/docker-compose.yaml
├── scripts/initialize.sh
└── security/
    └── authentik/
        ├── docker-compose.yaml               # (server, worker; outpost via profile)
        └── secrets/
           ├── postgres_password.secret
           ├── authentik_secret_key.secret
           ├── bootstrap_password.secret
           └── outpost_token.secret           # created after Outpost is created in the UI
```

---

## Add a new HTTP service behind Traefik (reference pattern)

1. Attach the service to the `proxy` network.
2. Use labels similar to:

```yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.myapp.rule=Host(`myapp.${DOMAIN}`)
  - traefik.http.routers.myapp.entrypoints=websecure
  - traefik.http.routers.myapp.middlewares=default-private-auth@file   # or default-public-auth@file
  - traefik.http.routers.myapp.service=myapp
  - traefik.http.services.myapp.loadbalancer.server.port=8080          # container’s HTTP port
  # If your backend truly requires HTTPS and has a valid cert:
  # - traefik.http.services.myapp.loadbalancer.server.scheme=https
  # (Prefer valid certs or plain HTTP behind Traefik; avoid skip-verify transports.)
```

> **TLS** is already applied at `entryPoints.websecure`; no per‑router `tls` labels are required.
> The first successful request to a hostname triggers DNS‑01 issuance.

---

## Publish a hostname publicly (Tunnel)

1. In **Cloudflare DNS**, create a **proxied** CNAME:
   `myapp.lablabland.com → <tunnel-uuid>.cfargotunnel.com`

2. In `core/cloudflared/config/config.yaml`, add:

   ```yaml
   - hostname: myapp.lablabland.com
     service: https://traefik:443
     originRequest:
       httpHostHeader: myapp.lablabland.com
       originServerName: myapp.lablabland.com
   ```

3. Add **Cloudflare Access** (Zero Trust) if the UI is sensitive.

---

## Operations quick‑start

- **Redeploy Traefik only**

```bash
cd core/traefik && docker compose up -d traefik
```

Dynamic config changes in `dynamic_conf/` are auto-reloaded.

- **Create an encrypted local backup (interim)**

```bash
OPNsense → System → Configuration → Backups
Backup: Area=All, check "Encrypt", then Download
```

- **Confirm access policy from LAN/WG/TS**

```bash
curl -kI https://portainer.lablabland.com \
  --resolve portainer.lablabland.com:443:192.168.1.3
# Expect 401 (BasicAuth now; will swap to Authentik SSO). if your IP is in allow-private; 403 means blocked by allow-list.
```

- **Check tunnel health**

```bash
docker exec cloudflared-tunnel cloudflared tunnel info
```

- **Inspect proxy network addressing**

```bash
docker network inspect proxy -f '{{range .Containers}}{{println .Name "→" .IPv4Address .IPv6Address}}{{end}}'
```

- **Suricata rule changes**
  After any policy edit: **Policies → Apply** → **Download → Download & Update rules** → **Restart Suricata**.

### Authentik admin quick commands (first‑run vs ongoing)

```bash
# If you don't know akadmin's password, reset it (no data loss):
docker exec -it authentik-server /lifecycle/ak changepassword akadmin

# Preferred: create a named superuser and use akadmin as break-glass:
docker exec -it authentik-server /lifecycle/ak createsuperuser \
  --username <your-admin> --email admin@lablabland.com

# Outpost: create it in the UI first, then paste its token and start the container
echo '<OUTPOST_TOKEN>' > security/authentik/secrets/outpost_token.secret
docker compose -f security/authentik/docker-compose.yaml --profile outpost up -d outpost
```

> **Bootstrap vs DB:** `AUTHENTIK_BOOTSTRAP_*` are consumed **only on first database initialization**. Once the DB exists, change/reset accounts via `/lifecycle/ak` or the UI; the bootstrap secrets won’t overwrite existing users.

---

## Security posture & plans

- Apply **middleware chains** consistently (use `*-auth` chains for admin UIs).
- Keep **WAN closed**; prefer WireGuard/Tailscale for private access and Cloudflare Tunnel + Access for public access.
- **SSO plan:** adopt **Authentik** (ForwardAuth with a Traefik outpost) for LAN/WG/TS; keep **Cloudflare Access** on public hostnames; retire BasicAuth.
- **IDS/IPS plan:** **Suricata** on **WAN** (IPS inline). Add **LAN** for visibility, tune, then decide whether to keep LAN inline or unbound.
- **Plan**: introduce **Docker Socket Proxy** (and optionally for Traefik) to reduce Docker API exposure.
- **WAF/AppSec (optional)**: add **CrowdSec** with the Traefik plugin for virtual patching and reputation bans.
- **Observability**: Traefik **access logs (JSON)** + **Prometheus metrics**; dashboards in Grafana.
- **OPNsense admin**: named admin with TOTP, GUI bound to **LAN/WG/TS only**, SSH key‑only if enabled.

---

## Troubleshooting history (selected)

- **TLS options mismatch** → resolved by matching `entryPoints.websecure.http.tls.options` (`default`) with `tls.options.default`.

- **403 from LAN** → caused by an `ipStrategy` under `allow-private` when not going through a proxy; removed so socket remote IP is authoritative.

- **Proxmox 404** → fixed by moving TCP passthrough to `entryPoints.websecure` (443) with SNI hostnames.

- **Cloudflared** → switched to locally managed config + credentials JSON; raised OS UDP buffers; QUIC warnings cleared.

- **Pi‑hole** → UI published via file‑provider service to support host‑network mode cleanly.

- **Suricata** → policies settled: `P20_ALERT_DETECTION`, `P05_NOISY_DISABLE`, `P01_APP_DETECT_TEST`; verified with HTTP EICAR (HTTPS intentionally not matched by content signatures).

- **On‑LAN visits to `auth.*` didn’t appear in Traefik access logs** (Aug 27, 2025).
  *Cause:* DNS resolution on‑LAN wasn’t pointing at the local reverse‑proxy path.
  *Fix:* Added Pi‑hole Local DNS record for `*.lablabland.com → 192.168.1.3`; on‑LAN and off‑LAN flows now traverse Traefik and are logged.

- **Authentik login failed with bootstrap password** (Aug 27, 2025).
  *Cause:* DB already contained `akadmin` from a previous first‑boot; bootstrap secrets are only used on initial DB creation.
  *Fix:* Reset `akadmin` via `/lifecycle/ak changepassword akadmin` or create a new named superuser.

---

## Current status & next steps

- ✅ Traefik healthy; ACME via Cloudflare DNS‑01; strict TLS; global HTTP→HTTPS.
- ✅ Admin UIs gated by allow‑private + BasicAuth (SSO swap planned).
- ✅ Proxmox available at `https://proxmox.lablabland.com` (TCP passthrough over 443).
- ✅ Interim **encrypted local backups** in place.
- ✅ Suricata operational on WAN; alerts flowing; policies active.
- ✅ **LAN DNS** via Pi‑hole Local DNS Records ensures on‑LAN requests hit local Traefik (access logs present).
- 🔜 Replace **Authelia → Authentik** (ForwardAuth outpost) and flip admin UIs to `*-auth` chains.
- 🔜 Launch Prometheus/Grafana/Uptime Kuma; wire up exporters.
- 🔜 Introduce Docker Socket Proxy; retire skip‑verify transports.
- 🔜 Enable **Git backup** to private GitHub (migrate to **Gitea** later) and **Nextcloud backup** for encrypted off‑site.
- 🔜 Add **CrowdSec** (Traefik plugin) and a **rate‑limit** middleware on login routes.

---

## Healthy “tunnel + proxy” indicators

- **Zero WAN port‑forwards**; Cloudflare terminates at the edge; tunnel uses mTLS; Traefik handles internal TLS and SNI passthrough for TCP.
- **4× QUIC** with tuned OS buffers; stable throughput.
- **Split‑horizon DNS**: LAN resolves `*.lablabland.com` to `192.168.1.3`; remote users resolve to Cloudflare via tunnel CNAMEs.

---

## Quick “Note for me” (Auth‑SSO rollout & account hygiene)

- **Outpost workflow:** Create the **Outpost in Authentik UI first** (Provider → Application → Outpost), copy token → save to `security/authentik/secrets/outpost_token.secret` → start `authentik-outpost` with the compose profile → flip one Traefik router to `*-auth` (Authentik) and test.
- **Accounts:** Create a **named superuser** for daily admin; enable **TOTP** (and WebAuthn). Convert `akadmin` to **break‑glass** (random long password in vault) or disable `is_active` (document re‑enable steps).
- **Bootstrap vs DB:** `AUTHENTIK_BOOTSTRAP_*` apply **only on first DB init**. For existing installs, use `/lifecycle/ak` to reset passwords or create users; bootstrap won’t overwrite.
        - **LAN DNS sanity:** Keep Pi‑hole Local DNS `*.lablabland.com → 192.168.1.3` so on‑LAN traffic hits Traefik (access logs/metrics consistent).

---

## mcp-launch — TUI Quick Start (Preflight, Launch, Results)

This repository includes a minimal supervisor `mcp-launch` with a Bubble Tea TUI that helps you inspect and launch MCP stacks (via mcpo) and optionally publish them over Cloudflare.

### Build

```bash
go mod tidy && go build -o mcp-launch
```

### Launch (TUI)

```bash
./mcp-launch up --tui [--config path ...] [-v|-vv]
```

- If `--config` is omitted: a Config Collector appears so you can add paths in‑TUI (supports `~` and `$ENV`, Tab for suggestions), set verbosity, tunnel type, and base ports.

### Launch with pre-made configs

```bash
go mod tidy && go build -o mcp-launch && ./mcp-launch up \
  --tui \
  --config "mcp_configs/mcp.chatgpt.spec-workflow.json" \
  --config "mcp_configs/mcp.chatgpt.utils.json" \
  --config "mcp_configs/mcp.serena.json"
```

### Preflight Screens & Keys


- List
  - `↑/↓` select server; `Enter`/`d` details; `c` tunnel picker; `g` toggle Controller (MCPO/RAW); `?` help.
  - Badges: `OK`, `ERR`, `HTTP`, `DISABLED`.

- Server Menu (selectable list)
  - Edit allowed tools, Edit disallowed tools, Edit tool descriptions, Toggle disable.

- Descriptions
  - `e` edit (uses `$VISUAL/$EDITOR` if set; falls back to textarea).
  - `d` diff (unified/side‑by‑side, `u/s`), `w` wrap toggle, `d/Enter/b/Esc` back.
  - `m` multi‑select: `Space` toggle, `t` Trim Selected (word‑boundary ≤300), `r` Truncate Selected (hard cut ≤300), `-` Clear Selected, `b` back.
  - Badges:
    - `RAW n>300` raw too long; can be reduced
    - `OVR TRIM ≤300` override equals a word-boundary trim of raw
    - `OVR TRUNC ≤300` override equals a hard truncate of raw
    - `OVR ≤300` custom short override

- Tunnel Picker (MCPO controller)
  - `Local (no tunnel)` → only 127.0.0.1 URLs
  - `Cloudflare Quick` → trycloudflare.com URL
  - `Cloudflare Named` → uses your named tunnel (no quick tunnel)

- Controller (MCPO vs RAW)
  - Press `g` on the list to toggle. MCPO runs full supervisor + merged OpenAPI; RAW starts MCP servers directly.

### Results Screen

After launch, a dedicated Results TUI shows per-stack summary and a logs panel.

- Summary per stack
  - OpenAPI URL, masked API key, config path
  - MCP server count, Endpoints (warn near/over 30), per-server tool counts and long‑desc flags
  - Copy block includes raw `OPENAPI=…` and `API_KEY=…`

- Logs Panel (bounded)
  - `l` toggle; `j/k` scroll, `Shift+J/K` fast scroll; `w` wrap
  - `/` search; `n/N` next/prev match
  - `S` save to `.mcp-launch/logs/YYYYMMDD_HHMMSS.log`

### Notes

- Preflight inspection uses `tools/list` pagination and retries for strict servers to avoid “Invalid request parameters”.
- Diff unified view uses char‑level highlights; side‑by‑side also highlights at char level in both columns.
- Multi‑select trim/truncate never overwrites fresh manual edits; it operates on the override when present.
