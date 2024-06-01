## Create cluster

eksctl create cluster --config-file=eks/non-prod-cluster.yaml
eksctl get cluster --profile=gprins
eksctl delete cluster non-prod-cluster --profile=gprins --disable-nodegroup-eviction

## deploy helm charts
cd infra
helm -n dev install  admin-api ./dos-admin-api --values ./dos-admin-api/values.yaml