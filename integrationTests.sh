export BRANCH=$(git branch --show-current)
docker volume rm domain-os_db
docker build -t geapex/domain-os:$BRANCH --build-arg GIT_SHA=$BRANCH . && doppler run -- docker compose --profile essential -f docker-compose-ci.yml up --abort-on-container-exit
doppler run -- docker compose rm --force --volumes
# the above stopped working for some reason, so I'm using the following instead
docker container rm domain-os-db-1
docker volume rm domain-os_db
