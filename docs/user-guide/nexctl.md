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

<!--  everything after this comment is generated with: ./hack/nexctl-docs.sh -->
### Usage

```text
NAME:
   nexctl - controls the Nexodus control and data planes

USAGE:
   nexctl [global options] command [command options] [arguments...]

COMMANDS:
   device          Commands relating to devices
   invitation      commands relating to invitations
   nexd            Commands for interacting with the local instance of nexd
   organization    Commands relating to organizations
   reg-key         Commands relating to registration keys
   security-group  commands relating to security groups
   user            Commands relating to users
   version         Get the version of nexctl
   vpc             Commands relating to vpcs
   help, h         Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                     Enable debug logging (default: false) [$NEXCTL_DEBUG]
   --service-url value         Api server URL (default: "https://try.nexodus.127.0.0.1.nip.io")
   --username value            Username
   --password value            Password
   --output value              Output format: json, json-raw, yaml, no-header, column (default columns) (default: "column")
   --insecure-skip-tls-verify  If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure (default: false)
   --help, -h                  Show help
```

#### nexctl device

```text
NAME:
   nexctl device - Commands relating to devices

USAGE:
   nexctl device command [command options] [arguments...]

COMMANDS:
   list      List all devices
   delete    Delete a device
   update    Update a device
   metadata  Commands relating to device metadata
   help, h   Shows a list of commands or help for one command

OPTIONS:
   --help, -h  Show help
```

#### nexctl invitation

```text
NAME:
   nexctl invitation - commands relating to invitations

USAGE:
   nexctl invitation command [command options] [arguments...]

COMMANDS:
   list     List invitations
   create   create an invitation
   delete   delete an invitation
   accept   accept an invitation
   help, h  Shows a list of commands or help for one command

OPTIONS:
   --help, -h  Show help
```

#### nexctl nexd

```text
NAME:
   nexctl nexd - Commands for interacting with the local instance of nexd

USAGE:
   nexctl nexd command [command options] [arguments...]

COMMANDS:
   version    Display the nexd version
   status     Display the nexd status
   get        Get a value from the local nexd instance
   set        Set a value on the local nexd instance
   proxy      Commands for interacting nexd's proxy configuration
   peers      Commands for interacting with nexd peer connectivity
   exit-node  Commands for interacting nexd exit node configuration
   help, h    Shows a list of commands or help for one command

OPTIONS:
   --unix-socket value  Path to the unix socket nexd is listening against (default: /var/run/nexd.sock)
   --help, -h           Show help
```

#### nexctl organization

```text
NAME:
   nexctl organization - Commands relating to organizations

USAGE:
   nexctl organization command [command options] [arguments...]

COMMANDS:
   list     List organizations
   create   Create a organizations
   delete   Delete a organization
   help, h  Shows a list of commands or help for one command

OPTIONS:
   --help, -h  Show help
```

#### nexctl user

```text
NAME:
   nexctl user - Commands relating to users

USAGE:
   nexctl user command [command options] [arguments...]

COMMANDS:
   list         List all users
   get-current  Get current user
   delete       Delete a user
   remove-user  Remove a user from an organization
   help, h      Shows a list of commands or help for one command

OPTIONS:
   --help, -h  Show help
```

#### nexctl security-group

```text
NAME:
   nexctl security-group - commands relating to security groups

USAGE:
   nexctl security-group command [command options] [arguments...]

COMMANDS:
   list     List all security groups
   delete   Delete a security group
   create   create a security group
   update   update a security group
   help, h  Shows a list of commands or help for one command

OPTIONS:
   --help, -h  Show help
```
