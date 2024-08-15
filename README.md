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

<h2 align="center">mono-go</h2>
<b>mono-go</b> is a collection of tools that were previously written in Bash in the <a href="https://github.com/monobilisim/mono.sh">mono.sh</a> repository. These tools are written in Go and provide a more efficient and faster alternative to the Bash scripts. The tools are cross-platform.
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

- osHealth
  - Checks OS health, including Disk, CPU and Memory usage.
  - Sends alarm notifications to a Slack webhook.
  - Opens issue in Redmine if disks are above the threshold.
  - Config: `/etc/mono.sh/os.yaml`

---

## Usage

1. Configure by editing the config files in `/etc/mono.sh/`. You can default values in the `config` folder. Please keep in mind that to use any of the tools, you need to also configure `/etc/mono.sh/global.yaml` file.

2. Run the desired tool using the following command as root:

```
mono-go NAME
```

Replace NAME with the name of the tool you want to run (e.g. `osHealth`).

A log file will be put on `/var/log/mono-go.log` if you want to check the errors. They will also be printed to stdout.

---


## Building

To build mono-go:

```
./build.sh
```

The resulting binaries will be in the `bin` folder.

---

## License

mono-go is licensed under GPL-3.0-only. See [LICENSE](LICENSE) file for details.

[contributors-shield]: https://img.shields.io/github/contributors/monobilisim/mono-go.svg?style=for-the-badge
[contributors-url]: https://github.com/monobilisim/mono-go/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/monobilisim/mono-go.svg?style=for-the-badge
[forks-url]: https://github.com/monobilisim/mono-go/network/members
[stars-shield]: https://img.shields.io/github/stars/monobilisim/mono-go.svg?style=for-the-badge
[stars-url]: https://github.com/monobilisim/mono-go/stargazers
[issues-shield]: https://img.shields.io/github/issues/monobilisim/mono-go.svg?style=for-the-badge
[issues-url]: https://github.com/monobilisim/mono-go/issues
[license-shield]: https://img.shields.io/github/license/monobilisim/mono-go.svg?style=for-the-badge
[license-url]: https://github.com/monobilisim/mono-go/blob/master/LICENSE