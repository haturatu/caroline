FROM node:22-alpine AS web-build

WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /caroline .

FROM alpine:3.22

WORKDIR /app
COPY --from=build /caroline /app/caroline
COPY static /app/static
COPY --from=web-build /web/static/app.js /app/static/app.js
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/caroline"]
