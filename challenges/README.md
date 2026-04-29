# Challenge Images

This directory is the local build root for deployable Docker-based challenges.

Each challenge source lives in its own subdirectory and usually contains a `Dockerfile`.

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

## Admin Usage

For a challenge that should be built locally:

- set `deploy_backend` to `auto`, `docker`, or a specific Docker target id
- leave `image` empty or set a local tag
- set `deploy_config.build_context` to a path under this directory

Example:

```json
{
  "build_context": "web/task-01",
  "cpus": "1.0",
  "memory": "512m",
  "ports": {
    "8080": "18080"
  }
}
```

## Build Script

Use [`scripts/build-challenge-images.ps1`](C:\Users\ilyastarcek\Documents\GitHub\heCsTackForse\scripts\build-challenge-images.ps1) to prebuild local images for all challenge folders that contain a `Dockerfile`.
