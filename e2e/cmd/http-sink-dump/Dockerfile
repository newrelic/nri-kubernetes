FROM golang:alpine3.14

ADD ./http-sink-dump.go /cmd/http-sink-dump.go

ENTRYPOINT ["go", "run", "/cmd/http-sink-dump.go"]