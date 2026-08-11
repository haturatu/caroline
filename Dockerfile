FROM node:22-alpine AS web-build

WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY vite.config.ts ./
COPY web ./web
RUN npm run build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /caroline ./cmd/caroline && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /caroline-agent ./cmd/caroline-agent

FROM alpine:3.22 AS hub

WORKDIR /app
COPY --from=build /caroline /app/caroline
COPY --from=web-build /web/static /app/static
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/caroline"]

FROM alpine:3.22 AS agent

WORKDIR /app
COPY --from=build /caroline-agent /app/caroline-agent
ENV CAROLINE_AGENT_STATE_DIR=/var/lib/caroline-agent
VOLUME ["/var/lib/caroline-agent"]
ENTRYPOINT ["/app/caroline-agent"]

FROM hub AS release
