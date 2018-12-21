FROM golang:1.11 as build-env

WORKDIR /app

# add only dependency files first
ADD src/go.mod .
ADD src/go.sum .

RUN go mod download

# add source code
ADD src/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /bin/main
RUN ls -a /bin/main

# build minimal image with only binary
FROM scratch
COPY --from=build-env /bin/main main

EXPOSE 80
# TODO: port 443 for https

ENTRYPOINT ["./main"]