# heCsTackForse
Some multytype platform on go for ctf competition/cources with auto deployment in pve, kubernetes and etc.

## Docker deploy

1. Copy `.env.example` to `.env` and replace `APP_SECRET` and `POSTGRES_PASSWORD`.
2. Start the stack:

```powershell
docker compose --env-file .env up -d --build
```

3. Open `http://localhost:8000/setup` on the first run. The setup page creates the first administrator and basic CTF settings.

The application uses PostgreSQL in Compose. Challenge containers can be built and started through the Docker backend because the app container includes `docker-cli` and mounts the host Docker socket.
