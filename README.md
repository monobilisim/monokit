# [![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![GPL License][license-shield]][license-url]

[![Readme in English](https://img.shields.io/badge/Readme-English-blue)](README.md)

<div align="center"> 
<a href="https://mono.net.tr/">
  <img src="https://monobilisim.com.tr/images/mono-bilisim.svg" width="340"/>
</a>

<h2 align="center">monokit</h2>
<b>monokit</b> is a collection of tools that were previously written in Bash in the <a href="https://github.com/monobilisim/mono.sh">mono.sh</a> repository. These tools are written in Go and provide a more efficient and faster alternative to the Bash scripts. The tools are cross-platform.
</div>

---

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Tools](#tools)
- [Usage](#usage)
- [Building](#building)
- [License](#license)

---

## Tools

- traefikHealth
    - Check Traefik health, including service status and logs.
    - Sends alarm notifications to a Slack webhook.

- sshNotifier
    - Notifies of a successful SSH login/logout through the Slack webhook.
    - Config: `/etc/mono/ssh-notifier.yaml`

- pritunlHealth
    - Check Pritunl server health.
    - Sends alarm notifications to a Slack webhook.
    - Config: `/etc/mono/pritunl.yaml` (optional)

- postalHealth
    - Check Postal health, including service status, containers, and queued messages.
    - Sends alarm notifications to a Slack webhook.
    - Opens an issue in Redmine if the database is down.
    - Config: `/etc/mono/mail.yaml`

- pmgHealth
    - Checks PMG health, including service status, queued messages and PostgreSQL status.
    - Sends alarm notifications to a Slack webhook.
    - Config: `/etc/mono/mail.yaml`

- k8sHealth
    - Checks Kubernetes health, including node status and certificate expiration.
    - Sends alarm notifications to a Slack webhook.
    - Config: `/etc/mono/k8s.yaml`

- shutdownNotifier
  - Notifies of a poweroff/poweron through the Slack webhook.

- mysqlHealth
  - Checks MySQL health, including read and write operations.
  - Sends alarm notifications to a Slack webhook.
  - Config: `/etc/mono/db.yaml`

- redisHealth
  - Checks Redis health, including read and write operations.
  - Sends alarm notifications to a Slack webhook.
  - Config: `/etc/mono/redis.yml` (optional)

- rmqHealth
  - Checks RabbitMQ health, including Management API and node status.
  - Sends alarm notifications to a Slack webhook.
  - Config: `/etc/mono/rabbitmq.yaml` (optional)

- osHealth
  - Checks OS health, including Disk, CPU and Memory usage.
  - Sends alarm notifications to a Slack webhook.
  - Opens issue in Redmine if disks are above the threshold.
  - Config: `/etc/mono/os.yaml`

- redmine
  - Allows you to create, update and close issues in Redmine.
  - Has a service system that keeps track of the issue ID.
  - Config: `/etc/mono/global.yaml`

- alarm
  - Sends alarm notifications to a Slack webhook.
  - Config: `/etc/mono/global.yaml`

- update
  - Updates the monokit binary to the latest version.
  - Allows you to choose the version to update with the `--version` flag.

- migrate
    - Migrates from a old version of Monokit to a new one in case of a breaking change.

- lbPolicy
    - Allows you to switch between the load balancing policies on Caddy, and list current policies.
    - Config: `/etc/mono/glb-*.yaml`

- wppconnectHealth
    - Checks WPPConnect health, phone status.
    - Sends alarm notifications to a Slack webhook.
    - Config: `/etc/mono/wppconnect.yaml`

---

## Usage

1. Configure by editing the config files in `/etc/mono/`. You can default values in the `config` folder. Please keep in mind that to use any of the tools, you need to also configure `/etc/mono/global.yaml` file.

2. Run the desired tool using the following command as root:

```
monokit NAME
```

Replace NAME with the name of the tool you want to run (e.g. `osHealth`).

A log file will be put on `/var/log/monokit.log` if you want to check the errors. They will also be printed to stdout.

---


## Building

To build monokit:

```
./build.sh
```

The resulting binaries will be in the `bin` folder.

---

## License

monokit is licensed under GPL-3.0-only. See [LICENSE](LICENSE) file for details.

[contributors-shield]: https://img.shields.io/github/contributors/monobilisim/monokit.svg?style=for-the-badge
[contributors-url]: https://github.com/monobilisim/monokit/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/monobilisim/monokit.svg?style=for-the-badge
[forks-url]: https://github.com/monobilisim/monokit/network/members
[stars-shield]: https://img.shields.io/github/stars/monobilisim/monokit.svg?style=for-the-badge
[stars-url]: https://github.com/monobilisim/monokit/stargazers
[issues-shield]: https://img.shields.io/github/issues/monobilisim/monokit.svg?style=for-the-badge
[issues-url]: https://github.com/monobilisim/monokit/issues
[license-shield]: https://img.shields.io/github/license/monobilisim/monokit.svg?style=for-the-badge
[license-url]: https://github.com/monobilisim/monokit/blob/master/LICENSE
