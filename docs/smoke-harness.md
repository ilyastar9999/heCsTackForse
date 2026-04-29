# Smoke Harness

## Purpose

`smoke-deploy` is a minimal runtime check for the deploy pipeline.

It does not validate challenge semantics. It only validates that:

1. API login works
2. instance start works
3. instance fetch works
4. instance stop works

This is the fastest way to catch broken Docker/Kubernetes wiring after config changes.

## Command

Build or run directly:

```powershell
go run .\cmd\smoke-deploy `
  -base-url http://127.0.0.1:8080 `
  -username smoke `
  -password smoke-pass `
  -challenge-id 42 `
  -register
```

Arguments:

- `-base-url` server base URL
- `-username` login username
- `-password` login password
- `-challenge-id` deployable challenge ID
- `-register` create the user before login
- `-email` optional email for registration
- `-timeout` HTTP timeout, default `45s`

## Expected Output

The command prints one JSON object with:

- `start`: response from `POST /api/challenges/:id/instance`
- `fetch`: response from `GET /api/challenges/:id/instance`
- `stop_response`: response from `DELETE /api/challenges/:id/instance`

Any failed step exits with code `1` and prints the API error body.

## Notes

- Use a challenge configured with `deploy_type != no_deploy`.
- For Docker targets, the daemon must be reachable by the server.
- For Kubernetes targets, `kubectl` and cluster access must be configured on the server host.
- This tool is intentionally small. It is a liveness check, not a full integration test suite.
