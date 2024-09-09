# deploy a simple one node k3s cluster with the cluster pulumi stack
```
cd cluster
pulumi up
```

# config the host using ansible
```
cd ../ansible
ansible-playbook -u ubuntu -i inventory.ini -u root -k playbook.yml
``` 

# install the Rabbit MQ operator
`kubectl apply -f https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator.yml`
```
namespace/rabbitmq-system created
customresourcedefinition.apiextensions.k8s.io/rabbitmqclusters.rabbitmq.com created
serviceaccount/rabbitmq-cluster-operator created
role.rbac.authorization.k8s.io/rabbitmq-cluster-leader-election-role created
clusterrole.rbac.authorization.k8s.io/rabbitmq-cluster-operator-role created
clusterrole.rbac.authorization.k8s.io/rabbitmq-cluster-service-binding-role created
rolebinding.rbac.authorization.k8s.io/rabbitmq-cluster-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/rabbitmq-cluster-operator-rolebinding created
deployment.apps/rabbitmq-cluster-operator created
```

# install the Rabbit MQ cluster
`cd ../../helm`
`helm install rabbitmq --namespace rabbitmq-system ./rabbitmq`

# install the Prometheus operator and a full monitoring stack
https://www.rabbitmq.com/kubernetes/operator/operator-monitoring#config-perm


kubectl apply --filename https://raw.githubusercontent.com/rabbitmq/cluster-operator/main/observability/prometheus/monitors/rabbitmq-servicemonitor.yml

kubectl apply --filename https://raw.githubusercontent.com/rabbitmq/cluster-operator/main/observability/prometheus/monitors/rabbitmq-cluster-operator-podmonitor.yml


finally add this role to the prometheus operator to be able to discover the new rabbitmq monitors

 kubectl apply -f infra/prometheus/prometheus-roles.yaml
clusterrole.rbac.authorization.k8s.io/prometheus created
clusterrolebinding.rbac.authorization.k8s.io/prometheus created