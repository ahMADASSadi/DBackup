FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dbackup ./cmd/server

FROM postgres:17-bookworm
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends default-mysql-client ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && id -u app >/dev/null 2>&1 || useradd --uid 10001 --create-home --shell /usr/sbin/nologin app
COPY --from=build /out/dbackup /usr/local/bin/dbackup
COPY docker-entrypoint.sh /usr/local/bin/dbackup-entrypoint.sh
RUN chmod 755 /usr/local/bin/dbackup /usr/local/bin/dbackup-entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dbackup-entrypoint.sh"]
