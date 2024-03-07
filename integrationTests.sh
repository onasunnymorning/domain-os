export BRANCH=$(git branch --show-current)
docker volume rm domain-os_db
docker build -t geapex/domain-os:$BRANCH . && docker compose -f docker-compose-ci.yml up --abort-on-container-exit
docker compose rm --force --volumes