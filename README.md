# dokadoka-bc
## What's this?
Doka doka builder container (DDBC, ドカドカ構築器!) is a container-based Golang project-building tool that primarily supports WSL without the proprietary lock-in.

## What it does

- Clones the repository to the local working directory
- Handles the container to build the copied repository
- Uploads the artifact to S3-compatible object store if you wish

## Environment Variables and Corresponding Options

At the first, `.env` file is also supported.

| Variable name | Throughout Option | Summary | Default value |
| --- | --- | --- | --- |
| `DDBC_DOCKER_HOST` | --docker-host | DDBC handles Docker according to the variable. If the host is Linux or Mac, you shouldn't pay it much attention, but for Windows, there's a section later for you. | (depends on the host architecture) |
| `DDBC_LOCAL_DEST_DIR` | --local-dest-dir | Artifacts will be created in the location specified by the variable. | ("." (the current directory) if store specifications are missing or "" (empty)) |
| `DDBC_STORE_ENDPOINT` | --store-endpoint | Artifacts will be uploaded onto the location specified by the variable. | "" (empty) |
| `DDBC_STORE_ACCESS_KEY` | --store-access-key | Artifacts will be uploaded onto the location specified by the variable. | "" (empty) |
| `DDBC_STORE_SECRET_KEY` | --store-secret-key | Artifacts will be uploaded onto the location specified by the variable. | "" (empty) |
| `DDBC_STORE_BUCKET_NAME` | --store-bucket-name | Artifacts will be uploaded onto the location specified by the variable. | "" (empty) |
| `DDBC_STORE_FOLDERS` | --store-folders | Artifacts will be uploaded onto the location specified by the variable. | "" (empty) |
| `DDBC_STORE_REGION_NAME` | --store-region-name | Artifacts will be uploaded onto the location specified by the variable. | (empty) |
| `DDBC_LOCAL_REPO_ROOT` | --local-repo-root | Working directories for local repository will be created in the location specified by the variable. | "" (blank to use the temporary directory, otherwise, by specifying it, reuses for efficiency) |
| `DDBC_DOCKER_SSH_PRIVATE_KEY_FILENAME` | --docker-ssh-private-key-filename | DDBC uses the private key for Docker SSH connection | "" (empty) |
| `DDBC_SOURCE_REPO_URL` | --source-repo-url | DDBC will clone the repository specified by the variable | (should be specified) |
| `DDBC_SOURCE_TAG` | --source-tag | DDBC will checkout the commit specified by the variable | "" (empty) |
| `DDBC_GIT_SSH_PRIVATE_KEY_FILENAME` | --git-ssh-private-key-filename | DDBC will use the private key to clone the repository | "" (empty) |
| `DDBC_TOOLCHAIN_AND_VER` | --toolchain-and-ver | DDBC will select the image to use and the command to build by the variable | "golang:1.25.5" |
| `DDBC_GOMODCACHE` | --gomodcache | BBDC will specify the Go module cache path as the GOMODCACHE; it will be used instead of $GOPATH/pkg/mod" | "" (empty) |

## DDBC_DOCKER_HOST for Windows
You can specify "npipe:////./pipe/docker_engine" to DDBC_DOCKER_HOST if you use Docker Desktop or a compatible (e.g., Rancher Desktop).
However, DDBC supports an alternative to a pure WSL2 Docker environment.
The alternative way uses ssh access.

### Monika's scenario
This section describes a scenario for adding a WSL Distribution-side account for SSH Docker access.
After following the scenario, you can specify "localhost:2222" to DDBC_DOCKER_SSH_ADDRESS and "unix:///var/run/docker.sock" to DDBC_DOCKER_HOST.

#### SSH key generation on cmd.exe
```
DOS> mkdir "%HOMEDRIVE%%HOMEPATH%\.ssh"
DOS> ssh-keygen -t rsa -b 4096 -f "%HOMEDRIVE%%HOMEPATH%\.ssh\id_monika" -N ""
```

#### Installing OpenSSH server on the WSL Ubuntu
```
WSL$ sudo apt install openssh-server
WSL$ sudo cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak
WSL$ sudo sed -i 's/^#*Port 22/Port 2222/' /etc/ssh/sshd_config
WSL$ sudo systemctl daemon-reload
WSL$ sudo systemctl restart ssh
```

#### Adding the user `monika` to sign into the WSL Ubuntu via SSH

Note: I'll learn more about how to restrict Monika's behavior later. nologin?

```
WSL$ sudo useradd -m -s /bin/bash monika
WSL$ sudo usermod -aG docker monika
WSL$ sudo mkdir -p /home/monika/.ssh/
WSL$ sudo chown monika:monika /home/monika/.ssh
WSL$ sudo chmod 700 /home/monika/.ssh
WSL$ sudo touch /home/monika/.ssh/authorized_keys
WSL$ sudo chown monika:monika /home/monika/.ssh/authorized_keys
WSL$ sudo chmod 600 /home/monika/.ssh/authorized_keys
WSL$ cat /mnt/c/Users/YOUR_NAME/.ssh/id_monika.pub | sudo tee -a /home/monika/.ssh/authorized_keys
```

To check the account and credentials, try to log in with the identifier.
```
DOS> ssh monika@localhost -p 2222 -i "%HOMEDRIVE%%HOMEPATH%\.ssh\id_monika"
```
You can `exit` if it succeeds.

#### Advice
To configure SSH port forwarding for this SSH port, you should understand the risks, which are not light-weight. DDBC doesn't require exposing the port outside of your localhost (not recommended)

## Usage

```
Usage: ddbc [THRUOUT_OPTS] [(-t | --target) filename [TGT_OPTS]...]...

TGT_OPTS:
  -s <os>, --os <os>                target os
  -a <arch>, --arch <arch>          target architecture
  -p <param>..., --param <param>... additional parameter to pass to the build command (repeatable, example `-p "-race" -p "-ldflags=-s" -p "-ldflags=-w"`)
```

## License

Copyright (c) 2026-present Aki. Kuramoto. Doka doka builder container is free and open-source software licensed under the MIT License.

