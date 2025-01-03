export BRANCH=$(git branch --show-current)
docker build -t geapex/domain-os:$BRANCH --build-arg GIT_SHA=$BRANCH . \
&& docker push geapex/domain-os:$BRANCH \
&& docker scout quickview
