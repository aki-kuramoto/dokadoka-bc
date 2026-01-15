# dokadoka-bc
## What's this?
Doka doka builder container (DDBC) is a container-based Golang project-building tool that primarily supports WSL without the proprietary lock-in.

## What it does

- Clones the repository to the local working directory
- Handles the container to build the copied repository
- Uploads the artifact to S3-compatible object store if you wish

## Environment Variables

At the first, `.env` file is also supported.

| Variable name | Summary | Default value |
| --- | --- | --- |
| `DDBC_DOCKER_HOST` | DDBC handles Docker according to the variable. If the host is Linux or Mac, you shouldn't pay attention to it so much, but for Windows, there's a section later for you. | depends on the host architecture |

## DDBC_DOCKER_HOST for Windows
You can specify "npipe:////./pipe/docker_engine" to DDBC_DOCKER_HOST if you use Docker Desktop or a compatible (e.g., Rancher Desktop).
However, DDBC supports an alternative to a pure WSL2 Docker environment.
The alternative way uses ssh access.

### Monika's scenario
This section describes a scenario for adding a WSL Distribution-side account for SSH Docker access.
After following the scenario, you can specify "ssh://localhost:2222" to DDBC_DOCKER_HOST.

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


## License

Copyright (c) 2026-present Aki. Kuramoto. Doka doka builder container is free and open-source software licensed under the MIT License.

