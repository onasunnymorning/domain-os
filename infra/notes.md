## Create cluster

eksctl create cluster --config-file=eks/non-prod-cluster.yaml
eksctl get cluster --profile=gprins
eksctl delete cluster non-prod-cluster --profile=gprins --disable-nodegroup-eviction

## deploy helm charts
cd infra
helm -n dev install  admin-api ./dos-admin-api --values ./dos-admin-api/values.yaml

## making changes to DNS
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