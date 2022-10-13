FROM golang:1.19 AS build
WORKDIR /go/src/github.com/threefoldtech/tf-pinning-service
COPY auth ./auth
COPY config ./config
COPY database ./database
COPY ipfs-controller ./ipfs-controller
COPY logger ./logger
COPY pinning-api ./pinning-api
COPY scripts ./scripts
COPY services ./services
COPY main.go go.mod ./
RUN mkdir -p /data/db/ /data/log/
RUN go get -d -v ./... && CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o tfpinsvc .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o add_test_tokens ./scripts/add_test_tokens.go

FROM scratch AS runtime
WORKDIR /app/
ENV GIN_MODE=release
ENV TFPIN_DB_DSN="/data/db/pins.db"
COPY --from=build /go/src/github.com/threefoldtech/tf-pinning-service/tfpinsvc ./
COPY --from=build /go/src/github.com/threefoldtech/tf-pinning-service/add_test_tokens ./
COPY --from=build /data /data

CMD ["./tfpinsvc"]
