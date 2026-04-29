# Deploy RFC

## Goal

All deployable challenge types run from Docker images.

Two delivery modes are supported:

1. External image:
   - Admin sets `image` to a registry image such as `ghcr.io/org/challenge:latest`.
   - `deploy_config.build_context` is omitted.

2. Local build:
   - Admin keeps `image` empty or sets a local tag.
   - `deploy_config.build_context` points to a subdirectory under `./challenges`.
   - The Docker deployer runs `docker build` locally before `docker run`.

Kubernetes deploy targets expect `image` to be already available to the cluster.

## Scheduler

Deploy targets are configured on `/admin/deploy`.

`deploy_backend` on a challenge can be:

- `auto`: let the scheduler pick any target
- `docker`: pick any Docker target
- `kubernetes`: pick any Kubernetes target
- `pve`: pick any PVE target
- `<target-id>`: pin to a specific target configured on `/admin/deploy`

Scheduler modes:

- `ordered`: fill targets by ascending `order`
- `balanced`: rotate over all eligible targets with remaining capacity

Capacity is enforced by target-level limits:

- `max_instances`
- `max_cpu`
- `max_memory_mb`

Requested resources are read from `deploy_config`:

```json
{
  "cpus": "1.0",
  "memory": "512m"
}
```

Supported memory forms: `512`, `512m`, `512mb`, `1g`, `1gb`, `1gi`.

## Deploy Config

Common `deploy_config` fields:

```json
{
  "env": {
    "APP_ENV": "prod"
  },
  "ports": {
    "8080": "18080"
  },
  "command": ["./service"],
  "workdir": "/app",
  "cpus": "1.0",
  "memory": "512m"
}
```

Docker-specific fields:

```json
{
  "build_context": "web/task-01",
  "dockerfile": "Dockerfile",
  "build_args": {
    "MODE": "prod"
  },
  "build_no_cache": false,
  "volumes": [
    "C:/data:/data"
  ],
  "restart": "unless-stopped"
}
```

Kubernetes-specific fields:

```json
{
  "service_type": "ClusterIP",
  "image_pull_secret": "registry-credentials"
}
```

If `ports` are present, the Kubernetes deployer creates both a Pod and a Service.
For `NodePort`, the host-side value from `ports` is used as `nodePort` when valid.
`image_pull_secret` is optional and references an existing Kubernetes image pull secret in the target namespace. Multiple secrets can be supplied with `image_pull_secrets`.

## Runtime Env Contract

Every deployable challenge instance receives:

- `HECSTACK_CHALLENGE_ID`
- `HECSTACK_CHALLENGE_TYPE`
- `HECSTACK_DEPLOY_TYPE`
- `HECSTACK_DEPLOY_SELECTOR`
- `HECSTACK_CONNECTION_INFO`
- `HECSTACK_USER_ID` when owner-scoped
- `HECSTACK_TEAM_ID` when team-scoped

### `dynamic_deploy`

The platform injects:

- `FLAG`
- `DYNAMIC_FLAG`

This is the only supported source of truth for the instance flag. The player submit path validates the saved per-instance flag from the database.

### `pentest`

Recommended shape:

- expose the service over VPN
- keep manual submissions in `checker_config`

When VPN is enabled, the deploy also receives:

- `HECSTACK_VPN_ENDPOINT`
- `HECSTACK_VPN_SERVER_PUBLIC_KEY`
- `HECSTACK_VPN_SERVER_IP`
- `HECSTACK_VPN_GAME_NET_CIDR`
- `HECSTACK_VPN_DNS`

`checker_config` example:

```json
{
  "flags": [
    { "key": "user", "label": "User", "value": "FLAG{user}", "type": "exact", "points": 100 },
    { "key": "root", "label": "Root", "value": "FLAG{root}", "type": "exact", "points": 250 }
  ]
}
```

### `attack_defence_attack`

This type is scored by the AD engine, not by `/submit`.

Use it for services that other teams attack or for exploit-upload challenges. The runtime receives:

- `HECSTACK_ATTACK_MODE=true`

Weighted vulnerability buckets live in `checker_config`:

```json
{
  "buckets": [
    { "key": "rce", "label": "RCE", "points": 200 },
    { "key": "sqli", "label": "SQLi", "points": 100 }
  ]
}
```

Uploaded sploits should target one bucket key.

### `attack_defence_defense`

This type is scored by checker rounds and exploit resistance. The runtime receives:

- `HECSTACK_DEFENSE_MODE=true`
- VPN variables when VPN is enabled

Recommended `checker_config`:

```json
{
  "defense_checks": [
    { "key": "http", "script": "./check-http.sh", "points": 50 },
    { "key": "auth", "script": "./check-auth.sh", "points": 50 }
  ]
}
```

The AD engine records:

- `up` when all weighted checks pass
- `corrupt` when only part of the weighted score survives
- `down` when no weighted score survives

## Connection Info

`connection_info` remains an operator hint. Generated runtime connection details are written to instance `connection_info`.

Recommended admin-facing content:

- Dynamic deploy: short usage note such as `open the spawned instance and extract the flag`
- Pentest / defense: VPN note, jump host, exposed ports
- AD attack: exploit-runner note or service discovery note

## Challenges Directory

`./challenges` is the local build root for Docker challenge sources.

Example:

```text
challenges/
  web/task-01/
    Dockerfile
    app/
  pwn/echo/
    Dockerfile
    src/
```

`deploy_config.build_context` is resolved relative to `deployer.backends.docker.challenges_dir` and is rejected if it escapes that directory.
