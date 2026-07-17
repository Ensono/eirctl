ARG Version
ARG Revision

FROM docker.io/golang:1.26.5-trixie@sha256:117e07f49461abb984fc8aef661432461ff43d06faa22c3b73af6a49ce325cb9 AS builder

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
