export BRANCH=$(git branch --show-current)
export GIT_SHA=$(git rev-parse $BRANCH)
docker build --pull=false -t domain-os:$BRANCH  .
docker tag domain-os:local geapex/domain-os:$BRANCH

docker compose --profile essential -f docker-compose.yml up
