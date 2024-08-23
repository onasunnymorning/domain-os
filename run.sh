export BRANCH=$(git branch --show-current)
docker build -t geapex/domain-os:$BRANCH . && doppler run -- docker compose -f docker-compose.yml up --watch
doppler run -- docker compose rm --force --volumes