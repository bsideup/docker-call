FROM golang:1.22 AS workspace

    # mount the current directory as /work
    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'

    WORKDIR /work

    COPY go.mod go.sum ./

    RUN go mod download

FROM workspace AS tidy

    CMD ["go", "mod", "tidy"]

FROM workspace as build

        COPY . .
    
        RUN CGO_ENABLED=0 go build -ldflags="-extldflags=-static" .

FROM docker:cli as smoke-test

    LABEL com.docker.runtime.mounts.docker='type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock'
    LABEL com.docker.runtime.mounts.project='type=bind,source=${workdir},target=${workdir}'

    COPY --from=build /work/docker-call /root/.docker/cli-plugins/docker-call

    CMD ["sh", "-c", "docker call -w $workdir file://examples/exa.Dockerfile"]