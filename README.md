# MetaDock - AWS Instance Metadata Service Emulator

> [!NOTE]
> Do **NOT** use `MetaDock` in **production**. It is for **local development** environments **only**.

## Overview

`MetaDock` provides a lightweight Go implementation of the [AWS Instance Metadata Service (IMDSv2)][imds].

It is designed to run inside a Docker container and can be included in a `docker-compose` setup to
provide AWS credentials to other services, effectively emulating an EC2 environment locally.

Instead of retrieving credentials from AWS, the emulator loads them from the host, via the mounted
`${HOME}/.aws` directory, using the `aws configure export-credentials` command. It then exposing
them through the same API paths that would normally be available inside an EC2 instance:

* `/latest/api/token`
* `/latest/meta-data/iam/security-credentials`
* `/latest/meta-data/iam/security-credentials/{role-name}`

`MetaDock` responds with the same metadata format as a real EC2 instance, enabling AWS SDKs and CLI
commands inside containers to authenticate transparently.

It relies on the developer obtaining AWS credentials on the host machine *before* running the
`metadock` service, and attaching the services which require the service to the emulators docker
network.

## Usage

### Prerequisites

* Host machine with:

  - Linux
  - [Task][task]
  - Docker and `docker-compose`
  - AWS CLI v2 installed and configured (`aws configure`, `aws sso login`, or equivalent)

* An existing AWS profile with valid credentials.

### Quickstart

1. Ensure you are logged into AWS:

    ```bash
    aws configure
    ```

    Or

    ```bash
    aws sso login [--profile profile-name]
    ````

2. Include `compose.metadock.yml` in your `docker-compose.yml`.

    - Use `include` directive to include the supplied [`compose.metadock.yml`](compose.metadock.yml) file.
    - Add the `metadock` network to the services which need AWS credentials.

    See [`compose.example.yml`](compose.example.yml) for an example configuration.

3. Your services can now query the emulator.

    Optionally, from within the service (using `docker exec ...`):

    * Use `curl` to check if the `MetaDock` service is accessible

      ```bash
      curl http://metadock/

      # => MetaDock
      ```

    * Or, check by running `aws` CLI commands

      ```bash
      aws s3 ls
      ```

## Building and Testing

This project uses [Task][task] to manage common development workflows.

* Build the service

  ```bash
  task build
  ```

* Run tests

  ```bash
  task test
  ```

* Cleanup artifacts

  ```bash
  task clean
  ```

Alternatively, you can build manually:

```bash
go build -o metadock .
```

## License

MIT License. Copyright (c) 2025 Chris Stefano. See [LICENSE](LICENSE) for details.

<!-- links -->

[imds]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html
[task]: https://taskfile.dev/
