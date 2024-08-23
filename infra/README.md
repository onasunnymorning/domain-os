# Setting up the infrastructure

You will need a kubernetes cluster to deploy to

## Create a kubernetes cluster
Run the following command in a screen (it takes a while)
`eksctl create cluster` (optionally add a config file)

Wait for this to finish and copy the kubeconfig (.kube/config) to your local machine
```
scp raspi.local:~/.kube/config ~/.kube/eks.yaml
export KUBECONFIG=~/.kube/eks.yaml
export AWS_PROFILE=personal # or whatever your profile is
```


## Deploy the common service
### Deploy Kafka
First create a namespace for kafka
`kubectl create namespace kafka`

Then deploy the kafka cluster using bitnami helm chart
`helm install kafka bitnami/kafka -n kafka`

Wait for this to finish and the pods to become 'Running'

`kubectel get po -n kafka`


### Deploy DB
First create a namespace for the db
`kubectl create namespace db`

Then deploy the db using bitnami postgres helm chart
`helm install db bitnami/postgresql -n db`

Wait for this to finish