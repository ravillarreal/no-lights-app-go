# Multi-stage build: compiles both binaries (api + bot) into one lean image.
# The web frontend is embedded via go:embed, so the api binary is self-contained.

FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache de dependencias en una capa separada
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -H app
COPY --from=build /out/api /out/bot /usr/local/bin/
USER app
EXPOSE 8000

CMD ["api"]
