# Project Rules

## Secrets & Environment Variables

- This project uses **Doppler** for secrets management. Never hardcode secrets or read `.env` files directly.
- When running Go binaries locally, prefix commands with `doppler run --` to inject environment variables (e.g. `doppler run -- go run ./cmd/askg "..."`).
- When adding a new secret (e.g. an API key), instruct the user to add it via `doppler secrets set KEY=value` — do not create `.env` files or suggest `export` commands.
- Docker Compose services receive secrets via `${VAR}` interpolation from Doppler-managed `.env` files. Do not inline secret values in `docker-compose.yml`.
