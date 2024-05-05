FROM alpine:3

# mount the current directory as /work
LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'

WORKDIR /work

RUN apk add --no-cache exa

CMD exa -l