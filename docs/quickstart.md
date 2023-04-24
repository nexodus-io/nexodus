# Quick Start

This guide will walk you through getting your first devices connected via Nexodus.

## Install and Start the Nexodus Agent

### Fedora

```sh
# Enable the COPR repository and install the nexodus package
sudo dnf copr enable russellb/nexodus
sudo dnf install nexodus

# Start the nexodus service and set it to automatically start on boot
sudo systemctl start nexodus
sudo systemctl enable nexodus
```

Edit `/etc/sysconfig/nexodus` if you plan to use a Nexodus service other than <https://try.nexodus.io>.

Query the status of `nexd` and follow the instructions to register your device.

```sh
sudo nexctl nexd status
```

### Brew

For Mac, you can install the Nexodus Agent via [Homebrew](https://brew.sh/).

```sh
brew tap nexodus-io/nexodus
brew install nexodus
```

Start `nexd` with `sudo` and follow the instructions to register your device.

```sh
sudo nexd https://try.nexodus.io
```

### Other

Download the latest release package for your OS and architecture. Each release includes a `nexd` binary and a `nexctl` binary.

*Download `nexd`*

- [Linux x86-64](https://nexodus-io.s3.amazonaws.com/linux-amd64/nexd)
- [Linux arm64](https://nexodus-io.s3.amazonaws.com/linux-arm64/nexd)
- [Linux arm](https://nexodus-io.s3.amazonaws.com/linux-arm/nexd)
- [Mac x86-64](https://nexodus-io.s3.amazonaws.com/darwin-amd64/nexd)
- [Mac arm64 (M1, M2)](https://nexodus-io.s3.amazonaws.com/darwin-arm64/nexd)
- [Windows x86-64](https://nexodus-io.s3.amazonaws.com/windows-amd64/nexd.exe)

*Download `nexctl`*

- [Linux x86-64](https://nexodus-io.s3.amazonaws.com/linux-amd64/nexctl)
- [Linux arm64](https://nexodus-io.s3.amazonaws.com/linux-arm64/nexctl)
- [Linux arm](https://nexodus-io.s3.amazonaws.com/linux-arm/nexctl)
- [Mac x86-64](https://nexodus-io.s3.amazonaws.com/darwin-amd64/nexctl)
- [Mac arm64 (M1, M2)](https://nexodus-io.s3.amazonaws.com/darwin-arm64/nexctl)
- [Windows x86-64](https://nexodus-io.s3.amazonaws.com/windows-amd64/nexctl.exe)

Start `nexd` with `sudo` and follow the instructions to register your device.

```sh
sudo nexd https://try.nexodus.io
```

## Test Connectivity

Once you have the agent installed and running, you can test connectivity between your devices. To determine the IP address assigned to each device, you can check the service web interface at <https://try.nexodus.io>, look the `nexd` logs, or get the IP using `nexctl`.

```sh
sudo nexctl nexd get tunnelip
sudo nexctl nexd get tunnelip --ipv6
```

Try `ping` or whatever other connectivity test you prefer.

```sh
ping 100.100.0.1
```
