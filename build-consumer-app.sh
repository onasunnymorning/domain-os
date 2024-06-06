export BRANCH=$(git branch --show-current)
docker build -t geapex/consumer:$BRANCH -f ./cmd/cli/messaging/sub/Dockerfile .