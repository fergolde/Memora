# Memora Player API (Option A — existing endpoints)

Read-only contract for external players (e.g. Velodrome) against a running Memora.
Verified live against `Memora dev` on 2026-10-31. Base URL is the Memora origin
(`BASE_URL`, fill in below); all paths are under `/apis/web/v1`.

> **Integrator: set this first**
> `BASE_URL = https://MEMORA_ORIGIN_AQUI`  <!-- TODO(usuario): URL pública de Memora -->

## Auth

1. In the Memora web UI, open the user menu → API keys → generate one key per device.
2. Send it on every request:
   `Authorization: Token <apikey>`
3. Without a key, the read endpoints below still answer **only if** login gate
   is disabled (`MEMORA_LOGIN_GATE` unset/false, the default). `/user` and
   `/user/apikeys` always require session or key (401 otherwise).

## Pagination & periods

- `limit` (default 100, max 500), `page` (default 1).
- **Always send `period`**: `day|week|month|year|all_time`. Omitting it returns
  empty results.
- Extra filters: `year`, `month`, `week`, `from`/`to` (unix), `artist_id`,
  `album_id`, `track_id` depending on endpoint.
- List responses share the envelope:
  `{"items":[...],"total_record_count":N,"items_per_page":N,"has_next_page":bool,"current_page":N}`

## Endpoints

| Method & path | Params | Use in player |
|---|---|---|
| `GET /config` | — | `{"default_theme":"black"}` — connectivity check |
| `GET /top/tracks` | period, limit, page | Top tracks; each item has `id`, `title`, `artists[]`, `musicbrainz_id`, `listen_count`, `duration`, `image{}` |
| `GET /top/albums` | same | Top albums (`title`, `artists[]`, `image{}`) |
| `GET /top/artists` | same | Top artists (`name`, `image{}`) |
| `GET /listens` | period, limit, page | History, e.g. "recently played" |
| `GET /listen-activity` | period | Activity buckets for charts |
| `GET /first-activity` | — | Library start date |
| `GET /now-playing` | — | Currently playing (if scrobbling) |
| `GET /stats` | period | Totals header (pass `period`) |
| `GET /search` | `q` (+ filters) | Unified search |
| `GET /summary` | period | Period digest |
| `GET /artist/{id}` (+`/aliases`, +`/interest`) | — | Detail/relations |
| `GET /album/{id}` (+`/artists`, +`/aliases`, +`/interest`) | — | Detail/relations |
| `GET /track/{id}` (+`/artists`, +`/aliases`, +`/interest`) | — | Detail/relations |

Verified example — `GET /top/tracks?limit=1&period=year` → 200:
```json
{"items":[{"item":{"id":1546,"title":"Bang Bang","artists":[{"id":44,"name":"Santaflow"}],
"musicbrainz_id":null,"listen_count":23,"duration":0,
"image":{"xs":"/image/db4921bd-…/64x64.webp",…}}}],"total_record_count":0,…}
```

## Field units & types (do not assume)

| Field | Type | Unit/notes |
|---|---|---|
| `duration` | int32 | **seconds** (`0` = unknown) |
| `first_listen`, `time_listened` | int64 | **unix epoch, seconds** |
| `listen_count` | int64 | scrobbles in the requested `period` |
| `all_time_rank` | int64 | 1-based rank, all-time |
| `musicbrainz_id` | string\|null | may be `null` — always have a metadata fallback |
| `image.{xs,small,medium,large,xl}` | string | **relative** paths (`/image/…`) — prefix `BASE_URL` |
| `id` (track/album/artist) | int32 | Memora-internal id, NOT a Subsonic id |
| `subsonic_id` (/player only) | string\|null | Subsonic song/album id, `null` when unmatched |
| `match` (/player only) | string | `mbid` \| `metadata` \| `none` |

## Errors & limits

- Errors are JSON `{"error":"..."}`; unauthenticated management/player calls
  return `401` with that body.
- Read endpoints have **no rate limit** (only `POST /login` is throttled).
- No CORS headers on `/apis/web/v1` (native apps unaffected; browsers need a proxy).

## Integration checklist (in this order)

1. Set `BASE_URL` and check `GET {BASE_URL}/apis/web/v1/config` → 200.
2. Create one API key per device in the web UI; send
   `Authorization: Token <apikey>` on every call.
3. `GET /top/tracks?period=all_time&limit=5` → items with counts.
4. Prefix `image.*` with `BASE_URL` before rendering.
5. For playback, use `/player/*` (`subsonic_id`); fall back to Subsonic
   search by artist/title when it is `null`.

## Option C — recommendations, radio, discover (IMPLEMENTED)

Same auth (`Authorization: Token <apikey>`, always required) and the same
`subsonic_id`/`match` enrichment as above.

| Method & path | Params | Notes |
|---|---|---|
| `GET /player/recommendations` | `limit` (def 25, max 100), `seed_artist=` (optional) | Seed artist's top tracks, then month-top fill, deduped |
| `GET /player/radio` | `seed_track=` (Memora track id, required), `limit` (def 25, queue excludes seed) | `{"seed":{…},"queue":[…]}`: seed, same-artist by listens, month-top fill |
| `GET /player/discover` | `days=` (def 30), `min_listens=` (def 5), `limit` (def 25) | Forgotten favorites: ≥ min_listens all-time, none in last `days` |

Scrobbling is handled by Navidrome/Subsonic itself (Memora imports the
listens); no player scrobble endpoint is provided.

## Quirks

- `total_record_count` may be `0` on some endpoints even with items present;
  page with `has_next_page`.

## Next (Option B) — IMPLEMENTED

Playable endpoints under `/apis/web/v1/player` (require session or
`Authorization: Token <apikey>` — unlike the reads above, these ALWAYS
require auth). Every item carries `subsonic_id` (nullable) and
`match: mbid|metadata|none`; a miss never fails the request.

| Method & path | Params | Notes |
|---|---|---|
| `GET /player/top/tracks` | period (required), limit, page | Top + Subsonic song id |
| `GET /player/top/albums` | period (required), limit, page | Top + Subsonic album id |
| `GET /player/recent` | period (required), limit, page | Latest scrobbles, playable |
| `GET /player/search` | `q` (required) | Track search + Subsonic ids |
| `GET /player/resolve` | `mbid=` or `artist=`+`title=`, `type=song\|album` | Single MBID→Subsonic id lookup; always 200 |

Matching: exact `musicBrainzId` first, then normalized artist+title.
Without Subsonic configured everything answers with `subsonic_id: null`.
