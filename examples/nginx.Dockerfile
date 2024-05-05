FROM nginx:1.21

    LABEL com.docker.runtime.mounts.project='type=bind,source=.,target=/usr/share/nginx/html/'
    LABEL com.docker.runtime.ports.http='8080:80'

    CMD ["nginx", "-g", "daemon off;"]