# MU-Software-Development-Project2
Project2-Implementation

## What the project includes

This project is built around Docker Compose and is split into a few services:

- **nginx** serves the static frontend from `nginx/static`.
- **webapp** is the Go backend in `webapp/` and is built from `webapp.dockerfile`.
- **postgres** stores application data in a persistent Docker volume.
- **pgAdmin** provides a browser-based PostgreSQL admin interface.

The webapp container mounts the source tree into the container so code changes are picked up quickly during development. It also uses a Go module cache volume so repeated builds are faster.

## Build and run

1. Clone the repository.
2. Review `.env` and set the values for your local setup.
3. Build and start everything with Docker Compose:
   ```sh
   docker compose up --build -d
   ```
4. View logs if needed:
   ```sh
   docker compose logs -f
   ```

The build process works like this:

- Docker Compose reads the variables in `.env`.
- `webapp` is built from `webapp.dockerfile` using the configured Go version.
- `postgres` starts with the database name, user, and password from `.env`.
- `nginx` serves the frontend files from the `nginx/static` directory.
- Named volumes keep PostgreSQL data and the pgAdmin state between runs.

## Access the app

- Web app: `http://localhost:8080`
- pgAdmin: `http://localhost:8083`

## Makefile shortcuts

If you have `make` installed, the Makefile provides a few common shortcuts:

- `make launch` builds and starts all services in detached mode.
- `make build` builds the service images without starting containers.
- `make logs` follows logs for all services.
- `make webapp-logs` follows only the webapp logs.
- `make nginx-logs`, `make postgres-logs`, and `make pgadmin-logs` follow a single service.
- `make ps` shows the current container status.
- `make down` stops and removes the containers.
- `make reset` removes volumes and recreates the stack from scratch.

## Troubleshooting

- If you encounter issues, rebuild the stack:
  ```sh
  docker compose down -v
  docker compose up --build -d
  ```
- If SMTP mail is failing, double-check the `SMTP_*` values in `.env`.
- Make sure Docker is running before starting the project.