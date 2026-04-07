# MU-Software-Development-Project2
Project2-Implementation

## Services

- **PostgreSQL**: Persistent database storage using Docker volumes.
- **Go Webapp**: Connects to PostgreSQL and demonstrates database-backed web development. Go module cache is persisted for faster builds. Air is included for hot-reloading.
- **pgAdmin**: Web-based GUI for managing and viewing PostgreSQL data.

## Getting Started

1. Clone this repository:
   ```sh
   git clone https://github.com/notSam25/MU-Software-Development-Project2
   cd [Repo Dir]
   ```
2. Modify `.env` for required environment variables.
3. Build and start all services:
   ```sh
   docker compose up --build -d
   docker compose logs -f
   ```

## Usage
- Access the webapp at `http://localhost:8080`.
- Access pgAdmin at `http://localhost:8083`.
- Database data and Go module cache are persisted between runs for faster startup and reliability.

## Makefile Shortcuts

If you have `make` installed, you can use:
- `make launch` to build and start all services in detached mode.
- `make logs` to follow logs for all services.
- `make webapp-logs` to follow webapp logs only.
- `make ps` to view container status.
- `make down` to stop/remove containers.
- `make reset` to rebuild from scratch (`down -v` then `up --build -d`).

## Troubleshooting
- If you encounter issues, try rebuilding containers:
  ```sh
  docker compose down -v
  docker compose up --build -d
  ```
- Ensure your `.env` file is correctly configured.