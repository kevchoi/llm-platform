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


on the TGI server:
Testing TGI
```
curl http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-3.2-1B",
    "prompt": "Hello, my name is",
    "max_tokens": 20
  }'
```
Test DCG
```
curl http://localhost:9400/metrics
```
Test Node exporter
```
curl http://localhost:9100/metrics
```

opening metrics:
first, make sure to expose the enpdoints on AWS. then,
on the monitoring server, grafana is exposed through 3000
prometheus is exposed through 9090

also, make sure to add inbound rules to your GPU instance

use the private IPv4  address

ISSUE: 
had some issues, but restarting the instance seemed to help...