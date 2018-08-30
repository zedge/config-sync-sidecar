# Kubernetes Config Sync Sidecar

This is a service for keeping a local directory in sync with the contents of a config map.
It watches a given config map for changes, and immediately rsyncs the contents of it to
a given directory.

Normally, you can just use config maps directly through volume mounts,
but if this does not work for you for some reason, you can add a
sidecar in your pods with this service.

## Use case

This service is in user for syncing [experiment config](https://gitlab.com/zedge/data-warehouse/experiment-controller) 
from kubernetes to the [php frontend](https://github.com/zedge/frontend). Set up the service account in kubernetes by 
applying the manifest:
```bash
kubectl apply -f manifests/service-account.yaml
```

## Example

Here, the contents of the `zedge-services` config map in the current namespace will be mirrored into the
`/srv/services` directory:

```yaml
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  labels:
    app: my-service
  name: my-service
spec:
  selector:
    matchLabels:
      app: my-service
  template:
    metadata:
      labels:
        app: my-service
    spec:
      serviceAccountName: my-service
      automountServiceAccountToken: true
      containers:
        # Your main service
        - name: echoheaders
          image: k8s.gcr.io/echoserver:1.4
          ports:
            - name: http
			  containerPort: 8080
		  volumeMounts:
			- name: services
			  mountPath: /srv/services
          securityContext:
              runAsUser: 48
        # The config-sync sidecar
        - name: config-sidecar
          env:
          - name: SERVICE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: CONFIG_MAP_NAME
            value: zedge-services
          - name: LOG_LEVEL
            value: INFO
          - name: OUTPUT_DIR
            value: /srv/services
          image: us.gcr.io/zedge-dev/config-sync-sidecar:b9d4265a7d764b8fb01e07e97a2b15faf5f8f092
          imagePullPolicy: IfNotPresent
          resources:
              requests:
                cpu: 10m
                memory: 64Mi
          securityContext:
              runAsUser: 48  # must be the same as your main container, since files are written 0600!
        volumeMounts:
          - mountPath: /srv/services
            name: services


      volumes:
        - emptyDir: {}
          name: services
  
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: configmap-reader
rules:
- apiGroups:
    - ""
  resources:
    - configmaps
  verbs:
    - get
    - watch
    - list
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-service-configmap-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: configmap-reader
subjects:
  - kind: ServiceAccount
    name: my-service

```

## How to access kubernetes API server outside of a kubernetes cluster

**Note that a KUBECONFIG is essentially a secret!**


Find the secret for the service account

```bash
kubectl get secret | grep <service account name...>
kubectl get secret config-sync-sidecar-token-4mr94 -o json | jq -r '.data["ca.crt"]' | base64 -d > config-api-server.crt
```

Find API server URL with next command, look for https://...

```bash
kubectl config view | less

# (test-cluster is currently https://35.192.74.79 )
```

Set  cluster config with API endpoint to kubernetes master and CA to verify talking with correct API server

```bash
KUBECONFIG=/tmp/fooo kubectl config set-cluster --server https://35.192.74.79 --certificate-authority=config-api-server.crt --embed-certs=true test-cluster
```

Set credentials for user config-sync

```bash
KUBECONFIG=/tmp/fooo kubectl config set-credentials config-sync --token $(kubectl get secret config-sync-sidecar-token-4mr94 -o json | jq -r '.data["token"]' | base64 -d)
```

Name a context, ie env:test with the above configuration

```bash
KUBECONFIG=/tmp/fooo kubectl config set-context env:test --cluster test-cluster --user config-sync
```

Set default context for a kubeconfig

```bash
KUBECONFIG=/tmp/fooo kubectl config use-context env:test 
```

Verify it works:

```bash
KUBECONFIG=/tmp/fooo kubectl get configmap
```
