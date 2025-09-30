To run, install Docker on EC2 / Amazon Linux 2023:

```
sudo dnf update -y
sudo dnf install docker -y


# From https://docs.docker.com/compose/install/linux/#install-the-plugin-manually
DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
mkdir -p $DOCKER_CONFIG/cli-plugins
curl -SL https://github.com/docker/compose/releases/download/v2.39.4/docker-compose-linux-x86_64 -o $DOCKER_CONFIG/cli-plugins/docker-compose

chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose
docker compose version
```

# Start the Docker service
sudo systemctl start docker
# Enable Docker to start automatically on boot
sudo systemctl enable docker
# or ...
sudo systemctl enable --now docker?


# Add the ec2-user to the 'docker' group to run commands without 'sudo'
sudo usermod -a -G docker ec2-user

# ==> IMPORTANT: Log out and log back in to apply the group changes <==
exit
```

Then, run Docker Compose as a plugin:
```
docker compose version
```

Copy over any relevant files (docker-compose.yml, prometheus.yml) and run

```
docker compose up -d
docker compose ps
```