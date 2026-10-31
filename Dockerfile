FROM --platform=$BUILDPLATFORM node AS frontend

ARG MEMORA_VERSION=dev
ENV VITE_MEMORA_VERSION=$MEMORA_VERSION
ENV BUILD_TARGET=docker

WORKDIR /client
RUN npm install -g corepack
RUN corepack enable && corepack prepare yarn@4 --activate
COPY ./client .
RUN yarn install

RUN yarn run build

RUN find ./build/client -type f \( -name "*.js" -o -name "*.css" -o -name "*.html" -o -name "*.svg" \) -exec gzip -k -9 {} \;

FROM golang:1.25 AS backend

ARG MEMORA_VERSION=dev
ENV CGO_ENABLED=1
ENV GOOS=linux

WORKDIR /app

RUN apt-get update && \
	apt-get install -y libvips-dev pkg-config && \
	rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags "-X main.Version=$MEMORA_VERSION" -o app ./cmd/api


FROM debian:bookworm-slim AS final

WORKDIR /app

RUN apt-get update && \
	apt-get install -y libvips42  && \
	rm -rf /var/lib/apt/lists/*

COPY --from=backend /app/app ./app
COPY --from=frontend /client/build ./client/build
COPY ./client/public ./client/public
COPY ./assets ./assets
COPY ./db ./db

EXPOSE 4110

ENTRYPOINT ["./app"]
