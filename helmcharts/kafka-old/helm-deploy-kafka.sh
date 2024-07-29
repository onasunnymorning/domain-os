helm upgrade --install kafka oci://registry-1.docker.io/bitnamicharts/kafka
# helm template kafka oci://registry-1.docker.io/bitnamicharts/kafka > ./tempate/bitnami-kafka.yaml