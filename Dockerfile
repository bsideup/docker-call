FROM golang:1.22 AS workspace

    # mount the current directory as /work
    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'

    VOLUME "/root/.cache/go-build"

    WORKDIR /work

    COPY go.mod go.sum ./

    RUN go mod download

FROM workspace as build

    COPY . .

    RUN CGO_ENABLED=0 go build -ldflags="-extldflags=-static" .

    CMD "true"

FROM workspace AS tidy

    CMD ["go", "mod", "tidy"]

FROM docker:cli as smoke-test

    LABEL com.docker.runtime.mounts.docker='type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock'
    LABEL com.docker.runtime.envs.WORKDIR='${workdir}'
    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=${workdir}'

    COPY --from=build /work/docker-call /root/.docker/cli-plugins/docker-call

    CMD ["sh", "-c", "docker call -w $WORKDIR file://examples/exa.Dockerfile"]