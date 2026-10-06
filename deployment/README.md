# Deployment

`local/compose.yaml` starts PostgreSQL for application development. `kubernetes/` runs the production image and PostgreSQL together for a production-style local test.

## Local Kubernetes

Requirements: Docker, k3d, kubectl, and the existing `k3s-default` cluster.

```sh
k3d cluster start k3s-default
docker build -t raidy:local .
k3d image import raidy:local -c k3s-default

kubectl create namespace raidy --dry-run=client -o yaml | kubectl apply -f -
cp deployment/kubernetes/secret.env.example deployment/kubernetes/secret.env
```

Fill in `deployment/kubernetes/secret.env`. Use a URL-safe PostgreSQL password because Kubernetes inserts it into `DATABASE_URL`. Then apply the complete deployment:

```sh
kubectl create secret generic raidy-secrets \
  --namespace raidy \
  --from-env-file=deployment/kubernetes/secret.env \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k deployment/kubernetes/overlays/local
kubectl rollout status statefulset/postgres -n raidy --timeout=120s
kubectl rollout status deployment/raidy -n raidy --timeout=120s
kubectl port-forward -n raidy service/raidy 8080:8080
```

Register `http://localhost:8080/api/auth/callback` as a Discord OAuth2 redirect. In another terminal, verify the service:

```sh
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:8080/readyz
```

The `raidy` namespace and PostgreSQL persistent volume remain after stopping the k3d cluster. The application stays single-replica because it registers commands and runs its scheduler in-process.

Tagged releases publish `linux/amd64` and `linux/arm64` images to `ghcr.io/joshchen-dev/raidy`. The package must be made public once in GitHub before an unauthenticated homelab can pull it.

Argo CD, ingress, Cloudflare Tunnel, monitoring, backups, and AWS deployment remain deferred until the RHEL homelab milestone.
