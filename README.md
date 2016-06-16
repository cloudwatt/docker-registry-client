# docker-registry-client

`docker-registry-client` is a command-line client for query your private registry.


## Install

Get the binary directly from GitHub releases or download the code and compile it with make. It requires Go 1.5 or later.


## Usage

```shell
$ export REGISTRY=https://url.to.docker.registry
$ export REGISTRY_USERNAME=username   # optional
$ export REGISTRY_PASSWORD=password   # optional
$ ./registry help
usage: docker-registry-client --registry=REGISTRY [<flags>] <command> [<args> ...]

A command-line docker registry client.

Flags:
      --help               Show context-sensitive help (also try --help-long and --help-man).
      --debug              print http headers
      --curl               print http request with curl command
  -r, --registry=REGISTRY  Registry base URL (eg. https://index.docker.io)
  -u, --username=USERNAME  Username
      --password=PASSWORD  Password
      --version            Show application version.

Commands:
  help [<command>...]
    Show help.

  delete <repository> <reference>
    Delete an image

  tags <repository>
    List tags
```
