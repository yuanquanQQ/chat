FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server ./server
RUN CGO_ENABLED=0 go build -o /chat-server ./server

FROM alpine:3.22
RUN adduser -D -u 10001 chat
USER chat
COPY --from=build /chat-server /chat-server
ENTRYPOINT ["/chat-server"]
