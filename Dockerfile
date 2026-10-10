FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/sword-go .

FROM alpine:3.22

RUN addgroup -S sword && adduser -S -G sword sword \
    && apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /out/sword-go /usr/local/bin/sword-go

ENV SWORD_GO_HTTP_PORT=8088
ENV SWORD_GO_DB_DSN=file:/var/lib/sword-go/sword-go.db?cache=shared&mode=rwc

VOLUME ["/var/lib/sword-go"]

EXPOSE 8088

USER sword

ENTRYPOINT ["sword-go", "web"]
