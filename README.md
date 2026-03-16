# MU-Software-Development-Project2
Project2-Implementation

A Dockerized environment for MU CMP_SC-4320 'Project 2'. This repository provides a quick way to run a PostgreSQL database, a Go web application, and a pgAdmin GUI for database management. It also contains all logic required in the rubric.

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
- Access the webapp at `http://localhost:<webapp-port>` (set in your `.env` file).
- Access pgAdmin at `http://localhost:8080` (default).
- Database data and Go module cache are persisted between runs for faster startup and reliability.

## Troubleshooting
- If you encounter issues, try rebuilding containers:
  ```sh
  docker compose down -v
  docker compose up --build -d
  ```
- Ensure your `.env` file is correctly configured.

## License
MIT
