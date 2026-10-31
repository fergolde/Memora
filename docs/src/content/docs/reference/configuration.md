---
title: Configuration
description: The available configuration options when setting up Memora.
---

Memora is configured using **environment variables**. This is the full list of configuration options supported by Memora.

The suffix `_FILE` is also supported for every environment variable. This allows the use of Docker secrets, for example: `MEMORA_SUBSONIC_PARAMS_FILE=/run/secrets/subsonic-params` will load the content of the file at `/run/secrets/subsonic-params` for the environment variable `MEMORA_SUBSONIC_PARAMS`.

:::caution
If the environment variable is defined without **and** with the suffix at the same time, the content of the environment variable without the `_FILE` suffix will have the higher priority.
:::

##### MEMORA_DEFAULT_USERNAME

- Default: `admin`
- Description: The username for the user that is created on first startup. Only applies when running Memora for the first time.

##### MEMORA_DEFAULT_PASSWORD

- Default: `changeme`
- Description: The password for the user that is created on first startup. Only applies when running Memora for the first time.

##### MEMORA_DEFAULT_THEME

- Default: `yuu`
- Description: The lowercase name of the default theme to be used by the client. Overridden if a user picks a theme in the theme switcher.

##### MEMORA_LOGIN_GATE

- Default: `false`
- Description: When `true`, Memora will not show any statistics unless the user is logged in.

##### MEMORA_BIND_ADDR

- Description: The address to bind to. The default blank value is equivalent to `0.0.0.0`.

##### MEMORA_LISTEN_PORT

- Default: `4110`
- Description: The port Memora will listen on.

##### MEMORA_ENABLE_STRUCTURED_LOGGING

- Default: `false`
- Description: When set to `true`, will log in JSON format.

##### MEMORA_LOG_LEVEL

- Default: `info`
- Description: One of `debug | info | warn | error | fatal`

##### MEMORA_ARTIST_SEPARATORS_REGEX

- Default: `\s+·\s+`
- Description: The list of regex patterns Memora will use to separate artist strings, separated by two semicolons (`;;`).

##### MEMORA_MUSICBRAINZ_URL

- Default: `https://musicbrainz.org`
- Description: The URL Memora will use to contact MusicBrainz. Replace this value if you have your own MusicBrainz mirror.

##### MEMORA_MUSICBRAINZ_RATE_LIMIT

- Default: `1`
- Description: The number of requests to send to the MusicBrainz server per second. Unless you are using your own MusicBrainz mirror, **do not touch this value**.

##### MEMORA_ENABLE_LBZ_RELAY

- Default: `false`
- Description: Set to `true` if you want to relay requests from the ListenBrainz endpoints on your Memora server to another ListenBrainz compatible server.

##### MEMORA_LBZ_RELAY_URL

- Required: `true` if relays are enabled.
- Description: The URL to which relayed requests will be sent to.

##### MEMORA_LBZ_RELAY_TOKEN

- Required: `true` if relays are enabled.
- Description: The user token to send with the relayed ListenBrainz requests.

##### MEMORA_CONFIG_DIR

- Default: `/etc/memora`
- Description: The location where import folders and image caches are stored.

##### MEMORA_FORCE_TZ

- Description: A canonical IANA database time zone name (https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) that Memora will use to serve all clients. Overrides any timezones requested via a `tz` cookie or `tz` query parameter. Memora will fail to start if this value is invalid.

##### MEMORA_DISABLE_DEEZER

- Default: `false`
- Description: Disables Deezer as a source for finding artist and album images.

##### MEMORA_DISABLE_COVER_ART_ARCHIVE

- Default: `false`
- Description: Disables Cover Art Archive as a source for finding album images.

##### MEMORA_DISABLE_MUSICBRAINZ

- Default: `false`

##### MEMORA_SUBSONIC_URL

- Required: `true` if MEMORA_SUBSONIC_PARAMS is set
- Description: The URL of your subsonic compatible music server. For example, `https://navidrome.mydomain.com`.

##### MEMORA_SUBSONIC_PARAMS

- Required: `true` if MEMORA_SUBSONIC_URL is set
- Description: The `u`, `t`, and `s` authentication parameters to use for authenticated requests to your subsonic server, in the format `u=XXX&t=XXX&s=XXX`. An easy way to find them is to open the network tab in the developer tools of your browser of choice and copy them from a request.
  :::caution
  If Memora is unable to validate your Subsonic configuration, it will fail to start. If you notice your container isn't running after
  changing these parameters, check the logs!
  :::

##### MEMORA_LASTFM_API_KEY

- Required: `false`
- Description: Your LastFM API key, which will be used for fetching images if provided. You can get an API key [here](https://www.last.fm/api/authentication),

##### MEMORA_SKIP_IMPORT

- Default: `false`
- Description: Skips running the importer on startup.

##### MEMORA_DISABLE_RATE_LIMIT

- Default: `false`
- Description: When enabled, disables the rate limiter that Memora has on the `/apis/web/v1/login` endpoint.

##### MEMORA_THROTTLE_IMPORTS_MS

- Default: `0`
- Description: The amount of time to wait, in milliseconds, between listen imports. Can help when running Memora on low-powered machines.

##### MEMORA_IMPORT_BEFORE_UNIX

- Description: A unix timestamp. If an imported listen has a timestamp after this, it will be discarded.

##### MEMORA_IMPORT_AFTER_UNIX

- Description: A unix timestamp. If an imported listen has a timestamp before this, it will be discarded.

##### MEMORA_FETCH_IMAGES_DURING_IMPORT

- Default: `false`
- Description: When true, images will be downloaded and cached during imports.

##### MEMORA_CORS_ALLOWED_ORIGINS

- Default: No CORS policy
- Description: A comma separated list of origins to allow CORS requests from. The special value `*` allows CORS requests from all origins.

:::danger
Environment variables below this notice are deprecated, and will not work with current versions of Memora.
:::

##### MEMORA_SQLITE_ENABLED

- Deprecated as of: `v0.2.1`
- Deprecated because: SQLite is the only supported database as of `v0.2.1`.
- Default: `false` (will always be enabled in `v0.2.0` onwards)
- Description: Enables SQLite instead of PostgreSQL as the database engine. Automatically migrates data from PostgreSQL if `MEMORA_DATABASE_URL` is also set on versions `v0.1.8` to `v0.1.10`.

##### MEMORA_DATABASE_URL

- Deprecated as of: `v0.2.1`
- Deprecated because: PostgreSQL is no longer a supported database as of `v0.2.1`.
- Required: `true` if you have not yet migrated to SQLite (PostgreSQL support will be deprecated in `v0.2.0` onwards)
- Description: A Postgres connection URI. See https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS for more information. Having this set will cause automatic migration to SQLite on versions `v0.2.X`. On `v0.3.X` or higher, having this set will cause the application to fail to start.
