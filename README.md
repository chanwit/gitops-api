# gitops-api 

This repository contains the source of the GitOps Backend for Backstage GitOps plugin ('gitops-profiles`).

## Running with Backstage

Using Docker is the easiest way.

```bash
docker run --init -p 3008:8080 chanwit/gitops-api
```
The plugin requires this backend to run on port 3008.

## Deploy on Kubernetes

This section shows am example to deploy Backstage with this gitops profiles plugin and its GitOps-API backend on Kubernetes.
To try this example, simply copy-n-paste the following yaml to a file named `deployment.yaml` and then run `kubectl apply -f deployment.yaml`.

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: backstage
  name: backstage
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backstage
  template:
    metadata:
      labels:
        app: backstage
    spec:
      containers:
        - image: spotify/backstage:latest
          imagePullPolicy: Always
          name: backstage
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: gitops-api
  name: gitops-api
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitops-api
  template:
    metadata:
      labels:
        app: gitops-api
    spec:
      containers:
        - image: chanwit/gitops-api:latest
          imagePullPolicy: Always
          name: gitops-api
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: backstage
  name: backstage
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: backstage
  type: NodePort
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: gitops-api
  name: gitops-api
  namespace: default
spec:
  ports:
    - port: 3008
      protocol: TCP
      targetPort: 8080
  selector:
    app: gitops-api
  type: ClusterIP
```
