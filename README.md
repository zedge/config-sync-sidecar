# Kubernetes Config Sync Sidecar

This is a service for keeping a local directory in sync with the contents of a config map.
It watches a given config map for changes, and immediately rsyncs the contents of it to
a given directory.

Normally, you can just use config maps directly through volume mounts, but if this does
not work for you for whatever reason, you can add a config sync sidecar in your pods.

## Example

Here, the contents of the `my-configmap` config map in the current namespace will be synced to the
`/config` directory:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: my-app
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-app
      automountServiceAccountToken: true
      containers:
        # Your main service
        - name: echoheaders
          image: k8s.gcr.io/echoserver:1.4
          ports:
            - name: http
              containerPort: 8080
          volumeMounts:
            - name: config
              mountPath: /config
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
        # The config-sync sidecar
        - name: config-sidecar
          env:
          - name: SERVICE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: CONFIG_MAP_NAME
            value: my-configmap
          - name: LOG_LEVEL
            value: INFO
          - name: OUTPUT_DIR
            value: /config
          image: quay.io/zedge/config-sync-sidecar:latest
          imagePullPolicy: Always
          resources:
            requests:
              cpu: 10m
              memory: 64Mi
          volumeMounts:
            - mountPath: /config
              name: config
      volumes:
        - emptyDir: {}
          name: config
  
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
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
  name: my-app-configmap-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: configmap-reader
subjects:
  - kind: ServiceAccount
    name: my-app
```
