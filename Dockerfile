FROM node:22-alpine AS web-build

WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY index.html ./
COPY vite.config.ts ./
COPY src ./src
RUN npm run build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /caroline ./cmd/caroline

FROM alpine:3.22

WORKDIR /app
COPY --from=build /caroline /app/caroline
COPY --from=web-build /web/static /app/static
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/caroline"]
