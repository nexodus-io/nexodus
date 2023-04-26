# Nexodus API Access with `nexctl`

## Installation

You can install `nexctl` using the following two ways

### Install pre-built binary

You can directly fetch the binary from the Nexodus AWS S3 bucket.

```sh
sudo curl -fsSL https://nexodus-io.s3.amazonaws.com/nexctl-linux-amd64 --output /usr/local/sbin/nexctl
sudo chmod a+x /usr/local/sbin/nexctl
```

### Build from the source code

You can clone the Nexodus repo and build the binary using

```sh
make dist/nexctl
```

## Using the Nexctl Utility

`nexctl` is a CLI utility that is used to interact with the Nexodus Service. It provides command line options to get the existing configuration of the resources like Organization, Peer, User and Devices from the Nexodus Service. It also allows limited options to configure certain aspects of these resources. Please use `nexctl -h` to learn more about the available options.
