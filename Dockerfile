ARG Version
ARG Revision

FROM docker.io/golang:1.27.0-trixie@sha256:df98008ecd2b0ecc9f0a94d1b07e3564a9c92b555369b33d9b5f60d0765b2db7 AS builder

ARG Version=0.0.1
ARG Revision=beta01

WORKDIR /app

COPY ./ /app
RUN CGO_ENABLED=0 go build -mod=readonly -buildvcs=false \
    -ldflags="-s -w -X github.com/Ensono/eirctl/cmd/eirctl.Version=${Version} -X github.com/Ensono/eirctl/cmd/eirctl.Revision=${Revision} -extldflags -static" \
    -o bin/eirctl cmd/main.go

FROM docker.io/alpine:3@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

COPY --from=builder /app/bin/eirctl /usr/bin/eirctl

ENTRYPOINT ["eirctl"]
