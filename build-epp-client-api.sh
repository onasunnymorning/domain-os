export BRANCH=$(git branch --show-current)
docker build -t geapex/epp-client-api:$BRANCH -f ./cmd/api/epp-client/Dockerfile . \
&& docker scout quickview