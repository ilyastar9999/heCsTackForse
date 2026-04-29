# VPN Provisioning Hook

The platform can provision per-team WireGuard peers by calling an external command whenever a peer is created or re-synced.

## Config

```yaml
ad:
  vpn:
    enabled: true
    server_public_key: "<wireguard-server-public-key>"
    server_endpoint: "vpn.example.com:51820"
    server_ip: "10.8.0.1"
    team_subnet_base: "10.8."
    game_net_cidr: "10.10.0.0/16"
    dns: "1.1.1.1"
    hook_command: "powershell"
    hook_args:
      - "-File"
      - "./scripts/vpn-hook.ps1"
    hook_timeout: "15s"
```

If `hook_command` is empty, the platform stays in config-only mode: it still generates and serves WireGuard client configs, but it does not push peers to a gateway automatically.

## Hook contract

The command is executed with the action in `HECSTACK_VPN_ACTION`.

Current action values:

- `provision`

Environment variables passed to the hook:

- `HECSTACK_VPN_ACTION`
- `HECSTACK_VPN_TEAM_ID`
- `HECSTACK_VPN_PEER_ID`
- `HECSTACK_VPN_PRIVATE_KEY`
- `HECSTACK_VPN_PUBLIC_KEY`
- `HECSTACK_VPN_ALLOWED_SUBNET`
- `HECSTACK_VPN_CLIENT_ADDRESS`
- `HECSTACK_VPN_SERVER_PUBLIC_KEY`
- `HECSTACK_VPN_SERVER_ENDPOINT`
- `HECSTACK_VPN_SERVER_IP`
- `HECSTACK_VPN_GAME_NET_CIDR`
- `HECSTACK_VPN_DNS`

The hook should:

1. create or update the peer on the WireGuard gateway;
2. ensure the client's routed subnet is allowed;
3. return exit code `0` on success;
4. write a concise error to stdout/stderr on failure.

If the hook fails, the peer is kept in the database with `provisioned = false`, and the error is shown in the challenge modal. The player can retry provisioning via `Sync VPN Access`.
