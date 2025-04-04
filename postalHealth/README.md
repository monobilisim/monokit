Got it. Here's the `README.md` in English, straight to the point:

---

```markdown
# postalHealth

`postalHealth` is a Go-based health check tool for monitoring the Postal mail server infrastructure. It verifies the status of Docker containers, MySQL databases, service endpoints, and message queues. Useful for alerting, logging, and operational visibility.

## Features

- Checks if `postal.service` is active via systemd
- Connects to Docker and lists Postal-related containers
- Verifies the status of:
  - Web: `http://localhost:5000/login`
  - Worker: `http://localhost:9090/health`
  - SMTP: `http://localhost:9091/health`
- Connects to `main_db` and `message_db` using config values in `/opt/postal/config/postal.yml`
- Checks:
  - MySQL connection health
  - Queued message count vs threshold
  - Held messages per server vs threshold
- Sends alerts using the Monobilisim `common` and `issue` packages

## Requirements

- Go 1.18+
- Docker
- Running Postal service
- MySQL
- Config file at `/opt/postal/config/postal.yml`
- Monobilisim `common`, `api`, `mail`, and `redmine/issues` packages
