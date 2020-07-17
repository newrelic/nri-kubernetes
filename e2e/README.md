# E2E tests

## Setup E2E server

You might have to run the following command, to make sure Tiller has the right privileges.

```shell script
cat << EOF | kubectl apply -f - 
kind: ServiceAccount
apiVersion: v1
metadata:
  name: tiller
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tiller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller
    namespace: kube-system
EOF
```
```
helm init --service-account tiller

# or if you installed Helm2 using Brew
/usr/local/opt/helm@2/bin/helm init --service-account tiller
```
