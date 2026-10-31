---
title: Installation
description: Guide on how to install Memora to start tracking your listening history.
---

## Docker

By far the easiest way to get up and running with Memora is using docker. Here is an example Docker Compose file to get you up and running in minutes:

```yaml title="compose.yaml"
services:
  memora:
    image: fergolde/memora:latest
    container_name: memora
    ports:
      - "4110:4110"
    volumes:
      - ./memora:/etc/memora
    restart: unless-stopped
```

:::note
Memora uses SQLite for all new installations as of `v0.2.1`. If you are running a version older than that, you will need to set `MEMORA_SQLITE_ENABLED=true`.
:::

You can find a full list of configuration options in the [configuration reference](/reference/configuration).

## Build from source

If you don't want to use docker, you can also build the application from source.

**Note that no support is provided when running Memora built from source.**

First, you need to install dependencies. Memora relies on `make`, `yarn` for building the client, and `libvips-dev` to process images.

```sh
sudo apt install libvips-dev make npm
sudo npm install --global yarn
```

If you aren't installing on an Ubuntu or Debian based system, you can easily find ways to install make, npm, and yarn by googling, and you can find other ways to install `libvips-dev` on the [libvips wiki](https://github.com/libvips/libvips/wiki/).

Then, clone the repository and execute the build command using the included Makefile:

```sh
git clone https://github.com/fergolde/Memora && cd Memora
make build
```

When the build is finished, you can run the executable at the root of the directory.

```sh
./memora
```

Then, navigate your browser to `localhost:4110` to enter your Memora instance.
