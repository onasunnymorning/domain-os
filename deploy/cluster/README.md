# Creating a cluster

`AWS_PROFILE=personal eksctl create cluster -f non-prod-cluster1.yaml`

This will take a while and will create the cluster.

## In case you need kubeconfig at a later stage
AWS_PROFILE=personal eksctl utils write-kubeconfig --cluster non-prod-cluster1 --kubeconfig=/Users/gprins/.kube/non-prod-cluster1.config

Make sure you can access it

```
▶ kubectl get node                                  
NAME                                           STATUS   ROLES    AGE   VERSION
ip-192-168-20-20.us-west-2.compute.internal    Ready    <none>   13m   v1.30.8-eks-aeac579
ip-192-168-32-121.us-west-2.compute.internal   Ready    <none>   13m   v1.30.8-eks-aeac579
ip-192-168-78-33.us-west-2.compute.internal    Ready    <none>   13m   v1.30.8-eks-aeac579
````

Next we drop a database in the same VPC as the EKS cluster. 
I've done this throught the console, as I'm not decided yet on the IAC stack.


# Clean up

I recommend uninstalling any helm charts you've installed, and then deleting the cluster as an exercise, unless you're really in a hurry


`AWS_PROFILE=personal eksctl delete cluster non-prod-cluster1`
