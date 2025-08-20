# MetaDock - AWS Instance Metadata Service Emulator

> [!NOTE]
> Do **NOT** use `MetaDock` in **production**. It is for **local development** environments **only**.

## Overview

`MetaDock` provides a lightweight Go implementation of a subset of the [AWS Instance Metadata Service (IMDSv2)][imds] 
for the purpose of providing session credentials only.

It is designed to run inside a Docker container and can be included in a `docker-compose` setup to
provide AWS credentials to other services, effectively emulating an EC2 environment locally.

Instead of retrieving credentials from AWS, the emulator loads them from the host, via the mounted
`${HOME}/.aws` directory, and then exposes them through the same API paths that would normally be 
available inside an EC2 instance:

* `/latest/api/token`
* `/latest/meta-data/iam/security-credentials`
* `/latest/meta-data/iam/security-credentials/{role-name}`

`MetaDock` responds with the same metadata format as a real EC2 instance, enabling AWS SDKs and CLI
commands inside containers to authenticate transparently.

It relies on the developer obtaining AWS credentials on the host machine *before* running the `metadock` 
service. If long-lived (static) credentials are found, the service will generate session credentials, 
with a default expiry of 12 hours.

## Usage

### Prerequisites

* AWS CLI v2 installed and configured (`aws configure`, `aws sso login`, or equivalent)
* AWS configuration profile with valid credentials.
* Docker and `docker-compose`
* [Task][task] for running development tasks

### Quickstart

#### Ensure you are logged into AWS and have valid credentials

```bash
aws configure
```

Or

```bash
aws sso login [--profile profile-name]
````

#### Include the `metadock` service in your Docker Compose configuration

The `MetaDock` service can be used in one of the following ways. 

1. Using the provided [`compose.metadock.yml`](compose.metadock.yml) file.

   - Use an [`include` directive][include] to include the [`compose.metadock.yml`](compose.metadock.yml) file.
   - Add the `metadock` network to the services which need IMDS.

    See [`compose.example.yml`](compose.example.yml) for an example configuration.

2. Adding the `metadock` service and configure the `AWS_EC2_METADATA_SERVICE_ENDPOINT` environment variable. 

    Edit your compose file, add the `metadock` service and configure services which need IMDS.    

    ```yaml
    services:
   
      metadock:
        image:  "ghcr.io/virtualstaticvoid/metadock:latest"
        command: "${AWS_PROFLE:-default}"
      volumes:
        - "${HOME}/.aws:/root/.aws:ro"

      your_service:
        image: "..."
        env:
          AWS_EC2_METADATA_SERVICE_ENDPOINT: http://metadock/

    ```

#### Your services can now query the emulator

Optionally, from within the respective services (using the `docker exec` command).

* If `curl` is installed in the container, check if the `MetaDock` service is issuing credentials.

    ```bash
    TOKEN=$(curl -X PUT "http://metadock/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
    curl -H "X-aws-ec2-metadata-token: $TOKEN" http://metadock/latest/meta-data/iam/security-credentials/metadock
    # => {"AccessKeyId":"...", ...}  
    ```
  
    See [Use the Instance Metadata Service to access instance metadata][using-imds] documentation for details.

* If the [`aws` CLI tool][aws-cli] is installed in the container, check by running the `sts get-caller-identity` CLI command.

    ```bash
    aws sts get-caller-identity --no-cli-pager
    # => {"UserId": "...", ...}
    ```

## Building and Testing

This project uses [Task][task] to manage common development workflows.

### Build the service

```bash
task build
```

#### Run tests

```bash
task test
```

#### Cleanup artifacts

```bash
task clean
```

Alternatively, you can run it directly on the host.

```bash
go build -o metadock .
PORT=8080 ./metadock <profile-name>
```

And connect, via `localhost` with the configured `PORT`.

```bash
TOKEN=$(curl -X PUT "http://localhost:8080/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
curl -H "X-aws-ec2-metadata-token: $TOKEN" http://localhost:8080/latest/meta-data/iam/security-credentials/metadock
# => {"AccessKeyId":"...", ...}  
```

## License

MIT License. Copyright (c) 2025 Chris Stefano. See [LICENSE](LICENSE) for details.

<!-- links -->

[aws-cli]: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
[imds]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html
[include]: https://docs.docker.com/reference/compose-file/include/
[task]: https://taskfile.dev/
[using-imds]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html
