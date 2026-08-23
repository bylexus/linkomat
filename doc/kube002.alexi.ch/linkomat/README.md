# linkomat on kube002.alexi.ch

Manifests for deploying linkomat on the existing k3s cluster at
`kube002.alexi.ch`. Mirrors the listomat deployment shape: shared Envoy
Gateway (`kube002-gateway`) in `envoy-gateway-system`, cert-manager with
the `lets-encrypt-alexi-ch` ClusterIssuer, local-path static PVs,
private registry `registry.alexi.ch` with `registry-alexich-cred` pull
secret.

Hostname: **linkomat.alexi.ch**

## Files (apply in this order)

| #   | File                                       | Scope                       | Notes                                              |
| --- | ------------------------------------------ | --------------------------- | -------------------------------------------------- |
| 00  | `00-namespace.yaml`                        | `linkomat`                  | empty namespace                                    |
| 10  | `10-pv.yaml`                               | cluster                     | static PV on node `kube002`, path `/data/k3s-storage/linkomat-data` |
| 20  | `20-pvc.yaml`                              | `linkomat`                  | binds to PV via `volumeName:`                      |
| 30  | `30-secret-env.yaml`                       | `linkomat`                  | env-only, no sensitive values                      |
| 40  | `40-deployment.yaml`                       | `linkomat`                  | port 8080, `/healthz` probes, RWO + Recreate       |
| 50  | `50-service.yaml`                          | `linkomat`                  | ClusterIP, port 80 → targetPort `http` (8080)      |
| 60  | `60-httproute-redirect.yaml`               | `linkomat`                  | 80 → 443, attaches to Gateway listener `http`      |
| 70  | `70-httproute-https.yaml`                  | `linkomat`                  | terminates TLS at Gateway, attaches to `linkomat-https` listener |
| 80  | `80-certificate.yaml`                      | `envoy-gateway-system`      | DNS-01 for `linkomat.alexi.ch`, secret `linkomat-tls` |
| 90  | `90-gateway-linkomat-listener.yaml`        | `envoy-gateway-system`      | strategic-merge patch on shared `kube002-gateway` adding the `linkomat-https` listener |

## Prerequisites (one-time, host)

1. The PV path `/data/k3s-storage/linkomat-data` must exist on node
   `kube002` and be writable by the container's UID (`65532`,
   `nonroot`). On the node:

   ```sh
   ssh kube002.alexi.ch
   sudo install -d -o 65532 -g 65532 -m 0750 /data/k3s-storage/linkomat-data
   ```

   (Listomat presumably used the same UID; reuse the same approach if it
   already exists.)

2. The image must be present at `registry.alexi.ch/linkomat:latest`.
   Build and push from the linkomat repo root:

   ```sh
   docker build -t registry.alexi.ch/linkomat:latest .
   docker push registry.alexi.ch/linkomat:latest
   ```

   (`imagePullPolicy: Always` will refetch the tag on every pod start.)

3. The `registry-alexich-cred` imagePullSecret must already exist in
   the `linkomat` namespace. The listomat namespace already has it; for
   `linkomat` either copy it or recreate it:

   ```sh
   kubectl get secret registry-alexich-cred -n listomat -o yaml \
     | sed 's/namespace: listomat/namespace: linkomat/' \
     | kubectl apply -f -
   ```

## Apply

```sh
# From the linkomat repo root.
cd doc/kube002.alexi.ch/linkomat

kubectl apply -f 00-namespace.yaml
kubectl apply -f 10-pv.yaml
kubectl apply -f 20-pvc.yaml
kubectl apply -f 30-secret-env.yaml
kubectl apply -f 40-deployment.yaml
kubectl apply -f 50-service.yaml

# TLS side first: cert must be Ready before the Gateway listener
# references the secret, else Envoy Gateway rejects the listener.
kubectl apply -f 80-certificate.yaml
kubectl wait --for=condition=Ready -n envoy-gateway-system certificate/linkomat-tls --timeout=120s

# Then add the listener to the shared Gateway via Server-Side Apply.
# (Gateway API does not support strategic-merge-patch; SSA merges by
# field ownership and is idempotent.)
kubectl apply --server-side --field-manager=linkomat-add \
  -f 90-gateway-linkomat-listener.yaml

# Now the routes will resolve.
kubectl apply -f 60-httproute-redirect.yaml
kubectl apply -f 70-httproute-https.yaml

# Sanity:
kubectl -n linkomat rollout status deploy/linkomat --timeout=120s
kubectl -n envoy-gateway-system get gateway kube002-gateway -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}'
kubectl get httproute -A --selector=app=linkomat
```

## First user

The image is distroless (`gcr.io/distroless/base-debian12:nonroot`), so
no shell — but `kubectl exec` passes argv straight to the entrypoint,
which is what we want:

```sh
# Wait for the pod to be Ready first.
kubectl -n linkomat wait --for=condition=Ready pod -l app=linkomat --timeout=120s

# Create the first user. The binary will prompt on stdin (TTY) or read
# "<pw>\n<pw>\n" from a pipe. Note the `-i`: kubectl exec defaults to
# --stdin=false, so without it the pipe never reaches the container
# and promptPassword() gets EOF -> "password must not be empty".
printf 'testpw\ntestpw\n' | kubectl -n linkomat exec -i deploy/linkomat -- /linkomat-server user create alex
```

Or interactively (password is not echoed):
```sh
kubectl -n linkomat exec -it deploy/linkomat -- /linkomat-server user create alex
```

(`exec` targets the Deployment; it picks the running pod. The arg vector
becomes `[/linkomat-server, user, create, alex]` because the container
ENTRYPOINT is `[/linkomat-server, serve]` and kubectl exec replaces the
command line — see `kubectl exec --help`.)

## Smoke checks

```sh
# In-cluster reachability. The image is distroless (no wget/curl/sh),
# so exec-based probes don't work; trust the kubelet readiness probe
# (the pod is `Ready 1/1` only when /healthz returns 200). To verify
# the listener wiring, hit the service from a debug pod:
kubectl -n linkomat run -it --rm --restart=Never --image=curlimages/curl debug \
  -- curl -fsS http://linkomat-service/healthz

# Public HTTPS reachability (should be 200).
curl -fsS https://linkomat.alexi.ch/healthz

# Login over HTTPS (should return access + refresh Set-Cookie).
curl -fsS -i -c /tmp/cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username":"alex","password":"testpw"}' \
  https://linkomat.alexi.ch/auth/login
```

## Notes / traps

- **Single replica + `Recreate`.** The PV is RWO and the SQLite file
  cannot tolerate concurrent writers across pods. Do not bump
  `replicas` until the schema moves off per-user SQLite files
  (deferred, see `doc/PLANNING.md` §11).

- **`imagePullPolicy: Always` + `:latest` tag.** Matches listomat. A
  bad push goes live on the next pod restart. For a pinned release, set
  a tag (`v0.1.0`) and switch policy to `IfNotPresent`.

- **`LINKOMAT_BEHIND_PROXY=true`.** Required so the server trusts
  `X-Forwarded-For` from the Envoy data plane; without it the rate
  limiter and `Secure` cookie logic misbehave. The Envoy Gateway does
  forward the header, but only trusted proxies skip the trust check.

- **`LINKOMAT_COOKIE_SECURE=true`.** Always on behind HTTPS. Don't set
  it false here — iOS PWA install on HTTP silently fails and the
  browser drops the cookie.

- **Gateway listener order.** `80-certificate.yaml` must be Ready before
  `90-gateway-linkomat-listener.yaml` references the secret, else
  Envoy Gateway fails the listener and the HTTPS route stays
  unattached. The `kubectl wait` in the apply block enforces this.

- **Gateway listener ownership.** The new `linkomat-https` listener is
  added via Server-Side Apply under field-manager `linkomat-add`. The
  existing listeners (`http`, `https`, `listomat-https`) remain owned
  by whoever applied them (kubectl client-side). Subsequent client-side
  `kubectl apply` of a full Gateway yaml would still clobber all
  listeners; don't do that. To remove the linkomat listener later:
  `kubectl apply --server-side --field-manager=linkomat-add --prune
  -f 90-gateway-linkomat-listener.yaml` (or delete the listener array
  via json-patch).

- **No initContainer.** Unlike listomat's Node migration script,
  linkomat uses GORM `AutoMigrate` on first request — no separate
  schema step needed. Saves a container start.

- **Health probes use `/healthz`.** Listomat uses `/login` because
  that's its only always-200 path; linkomat has a dedicated
  `/healthz`, so we use that. (`/version` exists too but `/healthz` is
  the canonical liveness signal.)

- **Hetzner DNS-01** is what `lets-encrypt-alexi-ch` issues against.
  The Certificate's `dnsNames` must match the Gateway listener's
  `hostname` exactly, and the listener's `certificateRefs[].name` must
  match the Certificate's `secretName`.
