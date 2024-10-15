export BRANCH=$(git branch --show-current)
docker volume rm domain-os_db
docker build -t geapex/domain-os:$BRANCH . && doppler run -- docker compose --profile essential -f docker-compose-ci.yml up --abort-on-container-exit
doppler run -- docker compose rm --force --volumes