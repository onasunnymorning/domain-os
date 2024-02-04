export BRANCH=$(git branch --show-current)
docker build -t geapex/domain-os:$BRANCH . && docker compose -f docker-compose.yml up --abort-on-container-exit
docker compose rm --force --volumes