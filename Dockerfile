ARG Version
ARG Revision

FROM docker.io/golang:1.24.3-bookworm AS builder

ARG Version=0.0.1
ARG Revision=beta01

WORKDIR /app

COPY ./ /app
# RUN ldflags="-s -w -X \"github.com/Ensono/eirctl/cmd/eirctl.Version=${Version}\" -X \"github.com/Ensono/eirctl/cmd/eirctl.Revision=${Revision}\" -extldflags -static" \
RUN CGO_ENABLED=0 go build -mod=readonly -buildvcs=false \
    -ldflags="-s -w -X github.com/Ensono/eirctl/cmd/eirctl.Version=${Version} -X github.com/Ensono/eirctl/cmd/eirctl.Revision=${Revision} -extldflags -static" \
    -o bin/eirctl cmd/main.go

FROM docker.io/alpine:3

COPY --from=builder /app/bin/eirctl /usr/bin/eirctl

ENTRYPOINT ["eirctl"]
