# Creating a cluster

`AWS_PROFILE=personal eksctl create cluster -f non-prod-cluster1.yaml`

This will take a while and will create the cluster.

## In case you need kubeconfig at a later stage
AWS_PROFILE=personal eksctl utils write-kubeconfig --cluster non-prod-cluster2 --kubeconfig=/Users/gprins/.kube/non-prod-cluster2.config

Make sure you can access it

```
▶ kubectl get node                                  
NAME                                           STATUS   ROLES    AGE   VERSION
ip-192-168-20-20.us-west-2.compute.internal    Ready    <none>   13m   v1.30.8-eks-aeac579
ip-192-168-32-121.us-west-2.compute.internal   Ready    <none>   13m   v1.30.8-eks-aeac579
ip-192-168-78-33.us-west-2.compute.internal    Ready    <none>   13m   v1.30.8-eks-aeac579
````

# Deploying a DB
Next we drop a database in the same VPC as the EKS cluster. 
I've done this throught the console, as I'm not decided yet on the IAC stack.
Make sure you use the same vpc, and configure security groups to allow traffic between the EKS cluster and the RDS instance.
Test you connectivity by deploying a netshoot container and getting this command to work:
`nc -vz <your db endpoint> 5432`


# Clean up

I recommend uninstalling any helm charts you've installed, and then deleting the cluster as an exercise, unless you're really in a hurry

delete the coredns pdb so you can delete the cluster
`kubectl delete pdb coredns -n kube-system`

`AWS_PROFILE=personal eksctl delete cluster non-prod-cluster1`

