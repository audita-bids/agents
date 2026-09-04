FROM golang:1.26-alpine AS builder

RUN apk add -U --no-cache ca-certificates git

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOPRIVATE=github.com/audita-bids/*

WORKDIR /go/src/github.com/audita-bids/agents

COPY go.mod go.sum ./
RUN --mount=type=secret,id=gh_token \
    --mount=type=cache,target=/go/pkg/mod \
    git config --global url."https://x-access-token:$(cat /run/secrets/gh_token)@github.com/".insteadOf "https://github.com/" \
 && go mod download \
 && rm -f /root/.gitconfig

COPY . .


RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /agents ./cmd/server

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /agents /usr/bin/agents

USER 65532:65532

EXPOSE 9090 19090

ENTRYPOINT ["agents"]
