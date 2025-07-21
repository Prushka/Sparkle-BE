docker buildx build --platform linux/amd64 \
 --build-arg GIT_COMMIT=$(git rev-list -1 HEAD) \
 --build-arg GIT_VERSION=$(git describe --tags --dirty --always) \
 -f Dockerfile.api --tag meinya/sparkle-api:latest --push .


docker buildx build --platform linux/amd64 \
 --build-arg GIT_COMMIT=$(git rev-list -1 HEAD) \
 --build-arg GIT_VERSION=$(git describe --tags --dirty --always) \
 -f Dockerfile.subtitles --tag meinya/sparkle-subtitles:latest --push .