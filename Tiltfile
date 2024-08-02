# Tiltfile

## FUNCTIONS
# UpdateValueFile parses a values_template.yaml file and substitutes the Doppler secrets
def updateValueFile():
    # run the dopper substitute command to parse the secrets into the values.yaml file
    cmd = 'doppler secrets substitute ./helmcharts/event-consumer/values_template.yaml > ./helmcharts/event-consumer/values.yaml'
    local(cmd)


## INSTALL KAFKA
# Load the Helm extension
load('ext://helm_resource', 'helm_resource', 'helm_repo')
# Install the Bitnami Helm repository and install Kafka for messaging
helm_repo('bitnami', 'https://charts.bitnami.com/bitnami')
helm_resource('kafka', 'bitnami/kafka', resource_deps=['bitnami'])

## INSTALL EVENT CONSUMER
# Build Event Consumer container for streaming events
docker_build('geapex/consumer', '.', dockerfile='cmd/cli/messaging/sub//Dockerfile')
# Get the password to connect to the freshly deployed kafka service
# Wait first to ensure the kafka service is deployed
exec_action(['kubectl', 'get' ,'secret', 'kafka-user-passwords'])
# Run the kubectl command to get the secret
result = local("kubectl get secret kafka-user-passwords --namespace default -o jsonpath='{.data.client-passwords}' | base64 -d | cut -d , -f 1", quiet=True)
# Run the doppler command to set the secret
cmd = 'doppler secrets set KAFKA_SASL_PASSWORD=%s' % result
local(cmd, quiet=True)
# Use Doppler substitute to generate a values.yaml file for the event-consumer Helm chart
updateValueFile()
# Read the Helm Chart and convert to k8s YAML
yaml = helm(
  'helmcharts/event-consumer',
  name='event-consumer',
  values=['./helmcharts/event-consumer/values.yaml']
)
# Apply the k8s YAML
k8s_yaml(yaml)

## INSTALL API
# Build the API container
docker_build('geapex/domain-os', '.', dockerfile='./Dockerfile_Tilt')
k8s_yaml(helm('helmcharts/admin-api', name='api'))
