## Create cluster and delete on AWS

eksctl create cluster --config-file=eks/non-prod-cluster.yaml
eksctl get cluster --profile=gprins
eksctl delete cluster non-prod-cluster --profile=gprins --disable-nodegroup-eviction

## Create and delete cluster on GCP
### using gcloud cli
cd infra
gcloud container clusters create zeus --machine-type n1-standard-2 --num-nodes 1
gcloud container clusters list

gcloud container clusters delete zeus
gcloud config configurations delete zeus
gcloud projects delete zeus-python-app
### using terraform
https://learnk8s.io/terraform-gke

`cd terraform/gcp`
`terraform init`
`terraform plan`
`terraform apply`
`terraform destroy`

Configure kubectl

`export KUBECONFIG="${PWD}/kubeconfig-prod"`

Test deployment

`kubectl apply -f deployment.yaml`

Test the test deployment :)

`kubectl port-forward $(kubectl get pod -l name=hello-kubernetes --no-headers | awk '{print $1}') 8080:8080`
http://localhost:8080/

Or create an service and ingress

`kubectl apply -f service-loadbalancer.yaml`
`kubectl apply -f ingress.yaml`

Remove the Loadballancer service and replace with container-Naive 

`kubectl delete svc/hello-kubernetes`
`kubectl apply -f service-neg.yaml`






## deploy helm charts
cd infra
helm -n dev install  admin-api ./dos-admin-api --values ./dos-admin-api/values.yaml
helm -n dev install epp ./epp-client-api

## delete helm charts
helm -n dev uninstall admin-api
helm -n dev uninstall epp


## making changes to DNS
Note that AWS will require a CNAME and GCP will require an A record.

```AWS_PROFILE=gprins aws route53 change-resource-record-sets \
  --hosted-zone-id Z095704739PYOAA4CKXS9 \
  --change-batch '{"Changes":[{"Action":"UPSERT","ResourceRecordSet":{"Name":"text.aws.apexdomains.net.","Type":"TXT","TTL":300,"ResourceRecords":[{"Value":"\"hello brave new world\""}]}}]}'
  ```

should result in

```{
    "ChangeInfo": {
        "Id": "/change/C10050344KDWFDQVN2GA",
        "Status": "PENDING",
        "SubmittedAt": "2024-06-01T13:33:55.438000+00:00"
    }
}
```

check status with 

`AWS_PROFILE=gprins aws route53 get-change --id C10050344KDWFDQVN2GA`

and then 

`dig txt text.aws.apexdomains.net.`