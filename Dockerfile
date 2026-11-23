ARG Version
ARG Revision

FROM docker.io/golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d AS builder

ARG Version=0.0.1
ARG Revision=beta01

WORKDIR /app

COPY ./ /app
RUN CGO_ENABLED=0 go build -mod=readonly -buildvcs=false \
    -ldflags="-s -w -X github.com/Ensono/eirctl/cmd/eirctl.Version=${Version} -X github.com/Ensono/eirctl/cmd/eirctl.Revision=${Revision} -extldflags -static" \
    -o bin/eirctl cmd/main.go

FROM docker.io/alpine:3@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

COPY --from=builder /app/bin/eirctl /usr/bin/eirctl

ENTRYPOINT ["eirctl"]
