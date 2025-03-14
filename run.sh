export BRANCH=$(git branch --show-current)
export GIT_SHA=$(git rev-parse $BRANCH)
echo "Building image for branch $BRANCH with commit $GIT_SHA"
docker build -t geapex/domain-os:$BRANCH --build-arg GIT_SHA=$GIT_SHA . && doppler run -- docker compose --profile essential -f docker-compose.yml up # --watch
doppler run -- docker compose rm --force --volumes
# the above stopped working for some reason, so I'm using the following instead
docker container rm domain-os-db-1
# uncomment the following to remove the volume (and all data)
# docker volume rm domain-os_db
