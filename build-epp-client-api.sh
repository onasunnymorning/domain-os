export BRANCH=$(git branch --show-current)
docker build -t geapex/epp-client-api:$BRANCH -f ./cmd/api/epp-client/Dockerfile --build-arg GIT_SHA=$BRANCH . \
&& docker scout quickview
