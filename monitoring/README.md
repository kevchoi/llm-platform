To run, install Docker on EC2 / Amazon Linux 2023:

```
sudo dnf update -y
sudo dnf install docker -y
````

Install Docker compose...
```
# From https://docs.docker.com/compose/install/linux/#install-the-plugin-manually
DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
mkdir -p $DOCKER_CONFIG/cli-plugins
curl -SL https://github.com/docker/compose/releases/download/v2.39.4/docker-compose-linux-x86_64 -o $DOCKER_CONFIG/cli-plugins/docker-compose

chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose
docker compose version
```

```
docker compose up -d
docker compose ps
```