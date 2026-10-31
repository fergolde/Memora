# Memora Player API (Option A — existing endpoints)

Read-only contract for external players (e.g. Velodrome) against a running Memora.
Verified live against `Memora dev` on 2026-10-31. Base URL is the Memora origin
(e.g. `https://musica.example.com`); all paths below are under `/apis/web/v1`.

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

## Integration notes (please read)

- `musicbrainz_id` can be `null` — resolve Subsonic songs by
  artist/title/album metadata, not by MBID alone.
- `image.*` URLs are **relative** — prefix them with the Memora origin.
- `duration` can be `0` when unknown.
- `total_record_count` may be `0` on some endpoints even with items present;
  page with `has_next_page`.
- Errors are JSON `{"error":"..."}`; unauthenticated management calls return 401.

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
