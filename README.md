# url-resolver

Resolves an url using a routing rules in `routing.json`. Routing complex load-balancing rules or routing an endpoint to both `http` and `https` endpoints is a non-trivial task using something like `nginx`. Sometimes you just want to resolve the actual connection url based on simple weighted load-balancing rules without generating ssl certificates if the upstream has both `http` and `https` endpoint.

This is a simple golang api that resolves the url based on routing rules in a [json config rule file](./routing.json). 

## Development

```sh
go install github.com/cosmtrek/air@latest
air
```

## Implementation

### Using environment variables

```sh
CONFIG_FPATH=./routing.json go run main.go
```

### Using command line arguments

```sh
go run main.go --fpath ./routing.json
```

## Deployment

### Docker

```sh
docker-compose up
```

### Kubernetes

```sh
kubectl apply -f k8s.yaml
```
