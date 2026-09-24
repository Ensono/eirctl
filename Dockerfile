ARG Version
ARG Revision

FROM docker.io/golang:1.27.1-trixie@sha256:433790e515d27dc6003e847e644cc0af956985cf315c1c58a3b73ee2dd305183 AS builder

ARG Version=0.0.1
ARG Revision=beta01

WORKDIR /app

COPY ./ /app
RUN CGO_ENABLED=0 go build -mod=readonly -buildvcs=false \
    -ldflags="-s -w -X github.com/Ensono/eirctl/cmd/eirctl.Version=${Version} -X github.com/Ensono/eirctl/cmd/eirctl.Revision=${Revision} -extldflags -static" \
    -o bin/eirctl cmd/main.go

FROM docker.io/alpine:3@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

COPY --from=builder /app/bin/eirctl /usr/bin/eirctl

ENTRYPOINT ["eirctl"]
