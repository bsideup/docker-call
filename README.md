# Docker Call CLI plugin - when `docker run` meets `docker build`

This project (not affiliated with Docker Inc.) is a PoC of a Docker CLI plugin that allows sourcing `docker run` flags (such as volumes, ports, network and other runtime parameters) from Docker images, as well as running Dockerfiles directly.

## Install
Run `make install` or `go build -o $$HOME/.docker/cli-plugins/docker-call .`.

## Examples

### Simple
Here is the simplest example:
```shell
$  bat examples/exa.Dockerfile
───────┬──────────────────────────────────────────────────────────────────────────
       │ File: examples/exa.Dockerfile
───────┼──────────────────────────────────────────────────────────────────────────
   1   │ FROM alpine:3
   2   │ 
   3   │ # mount the current directory as /work
   4   │ LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'
   5   │ 
   6   │ WORKDIR /work
   7   │ 
   8   │ RUN apk add --no-cache exa
   9   │ 
  10   │ CMD exa -l
───────┴──────────────────────────────────────────────────────────────────────────

$ docker call file://examples/exa.Dockerfile
[+] Building 0.3s (7/7) FINISHED                   docker:desktop-linux
 => [internal] load build definition from exa.Dockerfile           0.0s
 => => transferring dockerfile: 225B                               0.0s
 => [internal] load metadata for docker.io/library/alpine:3        0.3s
 => [internal] load .dockerignore                                  0.0s
 => => transferring context: 52B                                   0.0s
 => [1/3] FROM docker.io/library/alpine:3@sha256:c5b1261d6d3e4307  0.0s
 => CACHED [2/3] WORKDIR /work                                     0.0s
 => CACHED [3/3] RUN apk add --no-cache exa                        0.0s
 => exporting to image                                             0.0s
 => => exporting layers                                            0.0s
 => => writing image sha256:146c1fd20a955eea0ea7d40f355cb13d5535e  0.0s

drwxr-xr-x    - root  5 May 20:22 examples
.rw-r--r-- 3.5k root  5 May 20:31 go.mod
.rw-r--r--  72k root 28 Apr 17:04 go.sum
.rw-r--r-- 1.1k root  5 May 20:21 LICENSE.md
.rw-r--r-- 4.3k root  5 May 20:53 main.go
.rw-r--r--   62 root  2 May 12:52 Makefile
.rw-r--r-- 2.0k root  5 May 21:05 README.md
```

This roughly translates into:
```shell
docker run -it --rm -v .:/work $(docker build -q examples/exa.Dockerfile)
```

### Nginx

Now let's try something a bit more advanced:
```shell
bat examples/nginx.Dockerfile
───────┬────────────────────────────────────────────────────────────────────────────────────────────
       │ File: examples/nginx.Dockerfile
───────┼────────────────────────────────────────────────────────────────────────────────────────────
   1   │ FROM nginx:1.21
   2   │ 
   3   │     LABEL com.docker.runtime.mounts.html='type=bind,source=.,target=/usr/share/nginx/html/'
   4   │     LABEL com.docker.runtime.ports.http='8080:80'
   5   │ 
   6   │     CMD ["nginx", "-g", "daemon off;"]
───────┴────────────────────────────────────────────────────────────────────────────────────────────

$ docker call file://examples/nginx.Dockerfile
```

And in a separate terminal:
```shell
$ curl -sSL localhost:8080/README.md | head -n 1
# Docker Call CLI plugin - when `docker run` meets `docker build`
```

### Advanced
While the concept is simple, it enables some very interesting use cases. Let's have a look at this project's Dockerfile:
```Dockerfile
FROM golang:1.22 AS workspace

    # mount the current directory as /work
    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'

    WORKDIR /work

    COPY go.mod go.sum ./

    RUN go mod download

FROM workspace as build

        COPY . .
    
        RUN CGO_ENABLED=0 go build -ldflags="-extldflags=-static" .

FROM docker:cli as smoke-test

    LABEL com.docker.runtime.mounts.docker='type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock'
    LABEL com.docker.runtime.mounts.project='type=bind,source=${workdir},target=${workdir}'

    COPY --from=build /work/docker-call /root/.docker/cli-plugins/docker-call

    CMD ["sh", "-c", "docker call -w $workdir file://examples/exa.Dockerfile"]
```

As you can see, this is a multistage Dockerfile, formatted [to my preference](https://x.com/bsideup/status/1784262018834334196).

Let's use call `go mod tidy` without any Go installed on our machine:
```shell
$ docker call file://Dockerfile#workspace -- go mod tidy 2>/dev/null | head -n 5
go: downloading github.com/google/go-cmp v0.6.0
go: downloading github.com/stretchr/testify v1.9.0
go: downloading github.com/creack/pty v1.1.18
go: downloading go.uber.org/goleak v1.3.0
go: downloading github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
```

Note the `file://Dockerfile#workspace` syntax that allows selecting the build stage (similar to `docker build --target=workspace -f Dockerfile`).

Now let's run the smoke test, which will start a container with Docker CLI, install Docker Call CLI plugin in it, mount the Docker socket, mount project's workdir and execute `exa` from previous example, all with a single, simple invocation:
```shell
$ docker call file://Dockerfile#smoke-test
[+] Building 0.2s (7/7) FINISHED                                             docker:default
 => [internal] load build definition from exa.Dockerfile                               0.0s
 => => transferring dockerfile: 225B                                                   0.0s
 => [internal] load metadata for docker.io/library/alpine:3                            0.2s
 => [internal] load .dockerignore                                                      0.0s
 => => transferring context: 52B                                                       0.0s
 => [1/3] FROM docker.io/library/alpine:3@sha256:c5b1261d6d3e43071626931fc004f70149ba  0.0s
 => CACHED [2/3] WORKDIR /work                                                         0.0s
 => CACHED [3/3] RUN apk add --no-cache exa                                            0.0s
 => exporting to image                                                                 0.0s
 => => exporting layers                                                                0.0s
 => => writing image sha256:146c1fd20a955eea0ea7d40f355cb13d5535ec26657427e6e0a8a5a8e  0.0s
WARNING: current commit information was not captured by the build: git was not found in the system: exec: "git": executable file not found in $PATH
.rwxr-xr-x  24M root  5 May 20:30 docker-call
.rw-r--r--  787 root  5 May 21:17 Dockerfile
drwxr-xr-x    - root  5 May 21:09 examples
.rw-r--r-- 3.5k root  5 May 20:31 go.mod
.rw-r--r--  72k root 28 Apr 17:04 go.sum
.rw-r--r-- 1.1k root  5 May 20:21 LICENSE.md
.rw-r--r-- 4.3k root  5 May 20:53 main.go
.rw-r--r--   62 root  2 May 12:52 Makefile
.rw-r--r-- 5.8k root  5 May 21:21 README.md
```

This roughtly translates into... err... I won't even try to come up with the full `docker run` command 😅

### Auto-volumes
Docker Call with automatically mount the volumes defined in your Dockerfile. This is especially handy for caches.

Let's build https://github.com/testcontainers/testcontainers-java without Java on our machine! Here is our Dockerfile:
```Dockerfile
FROM bellsoft/liberica-openjdk-alpine:11 as gradle

    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/work'

    RUN apk add git findutils

    VOLUME /root/.gradle
    VOLUME /work/.gradle

    ENV GRADLE_OPTS="-Dorg.gradle.daemon=false -Dorg.gradle.console=rich -Dorg.gradle.warning.mode=none"

    WORKDIR /work

FROM gradle as build-only

    CMD ./gradlew :testcontainers:build -x check

FROM gradle as lint

    RUN apk add --update npm

    VOLUME /root/.npm

    CMD ./gradlew :testcontainers:spotlessCheck
```

If we run it for the first time, it will take a bit of time (cold start), but subsequent runs will use the cache automagically:
```shell
$ time docker call file://Dockerfile#lint
<...output...>
<... downloads the dependencies... >
docker call file://Dockerfile#lint  0.13s user 0.14s system 0% cpu 46.893 total

$ time docker call file://Dockerfile#lint
<...output...>
docker call file://Dockerfile#lint  0.12s user 0.12s system 2% cpu 8.724 total
```

Same for build:
```shell
$ time docker call file://Dockerfile#build-only
<...output...>
<...performs the build...>
BUILD SUCCESSFUL in 5s
6 actionable tasks: 6 up-to-date
docker call file://Dockerfile#build-only  0.13s user 0.09s system 1% cpu 14.630 total

$ time docker call file://Dockerfile#build-only
<...output...>
<...performs the build, cached...>
BUILD SUCCESSFUL in 5s
6 actionable tasks: 6 up-to-date
docker call file://Dockerfile#build-only  0.12s user 0.11s system 3% cpu 7.005 total
```

### Actions as images
Docker Call always you to publish your steps (actions?) as images:
```shell
$ docker build --push --target=workspace -t bsideup/go-workspace .
[+] Building 0.3s (9/9) FINISHED                      docker:desktop-linux
 => [internal] load build definition from Dockerfile                  0.1s
 => => transferring dockerfile: 826B                                  0.0s
 => [internal] load metadata for docker.io/library/golang:1.22        0.2s
 => [internal] load .dockerignore                                     0.0s
 => => transferring context: 52B                                      0.0s
 => [workspace 1/4] FROM docker.io/library/golang:1.22@sha256:d5302d  0.0s
 => [internal] load build context                                     0.0s
 => => transferring context: 55B                                      0.0s
 => CACHED [workspace 2/4] WORKDIR /work                              0.0s
 => CACHED [workspace 3/4] COPY go.mod go.sum ./                      0.0s
 => CACHED [workspace 4/4] RUN go mod download                        0.0s
 => exporting to image                                                0.0s
 => => exporting layers                                               0.0s
 => => writing image sha256:adc0186522cc1d962ec52bd1b8b4c5a18b264d9d  0.0s
 => => naming to docker.io/bsideup/go-workspace                       0.0s

$ docker call bsideup/go-workspace -- go version
go version go1.22.2 linux/arm64

$ docker build --push -t bsideup/exa -f examples/exa.Dockerfile .
$ docker call -w examples/ bsideup/exa
.rw-r--r-- 182 root  5 May 21:01 exa.Dockerfile
.rw-r--r-- 201 root  5 May 21:13 nginx.Dockerfile
```

## Supported labels

| Label | Corresponding Docker CLI flag | Example |
| --- | --- | --- |
| `com.docker.runtime.mounts.$name` | `--mount` | `type=bind,source=${workdir},target=/work` |
| `com.docker.runtime.ports.$name` | `-p` | `8080:80` |
| `com.docker.runtime.network` | `--network` | `host` |

All labels support the following variables:
- `workdir` workdir passed to `docker call` via `-w` flag, defaults to `docker call`'s workdir