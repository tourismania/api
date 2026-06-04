# Code Review: feature/3 — v1

**Date:** 2026-06-05  
**Branch:** `feature/3`  
**Scope:** `git diff main...HEAD` — airports feature (sync-airports CLI + GET /api/v1/airports)  
**Files changed:** 49 files, +3195 −32 lines  
**Method:** 7-angle finder (A–G) × parallel agents → 1-vote verify → top 10 findings

---

## Findings

### 🔴 Critical / High

#### 1. Country filter silently dropped end-to-end
**File:** `internal/application/query/search_airports/handler.go:33`  
**Summary:** The country filter is never forwarded at any layer — not in `AirportFilter`, not in `SearchAirportsParams`, and not in the SQL query. `?country=DE` returns all airports with HTTP 200.  
**Failure scenario:** HTTP GET `/api/v1/airports?country=DE` parses and validates the parameter correctly, but the application query handler (lines 33–37) never sets `AirportFilter.Country`; the repository `Search` method has nothing to forward; the SQL has no country `WHERE` clause. The feature is entirely non-functional.

---

#### 2. `Cache-Control: public` on a JWT-protected endpoint
**File:** `internal/presentation/http/api/v1/airport/search/handler.go:130`  
**Summary:** Sets `Cache-Control: public, max-age=3600` on an endpoint guarded by JWT middleware. Shared caches (CDN, reverse proxy) may serve cached responses without validating the token.  
**Failure scenario:** A CDN caches the first authenticated response keyed on URL only. A subsequent unauthenticated request to the same URL receives the cached body with HTTP 200, bypassing the JWT middleware entirely. Correct directive: `Cache-Control: private` or `no-store`.

---

#### 3. Integration test fails to compile — wrong constructor arity
**File:** `tests/integration/airport_repository_test.go:34`  
**Summary:** `pgrepo.NewAirportRepository(db.New(pool))` — called with 1 argument, but production constructor requires 2: `(queries *db.Queries, pool *pgxpool.Pool)`.  
**Failure scenario:** `go test ./tests/integration/...` fails to build, so the entire Upsert code path (bulk airport writes) is never exercised in CI. Any regression there goes undetected.

---

### 🟡 Medium

#### 4. `lat`/`lon` parameter order inverted in `pool.Exec`
**File:** `internal/infrastructure/persistence/postgres/repository/airport_repository.go:83`  
**Summary:** Signature declares `(lat, lon float64)` but `pool.Exec` passes them reversed as `(lon, lat)`. The SQL read path compensates (`location[1] AS lon, location[2] AS lat`) making the current round-trip appear correct, but the function contract is violated.  
**Failure scenario:** Any direct query against the `location` column (e.g. a PostGIS `ST_MakePoint` expecting `[lat, lon]`, or a BI tool) will receive transposed coordinates. The inconsistency is also actively misleading to future readers and maintainers.

---

#### 5. Russian city name lookup is non-deterministic for multi-airport cities
**File:** `internal/application/command/sync_airports/handler.go:145`  
**Summary:** Russian name is looked up by `r.ICAO` in `cityNamesRU`. Because Go map iteration order is random, cities with multiple airports are permanently stored in English if the first-iterated airport has no Wikidata translation.  
**Failure scenario:** Moscow is served by UUEE, UUWW, UUMO. If UUMO is iterated first and `cityNamesRU["UUMO"]` is empty, the city is upserted as "Moscow". The `seen`-guard fires for UUWW and UUEE — their Russian translation "Москва" is never consulted. After sync, Moscow stays in English permanently.

---

#### 6. Wikidata pagination terminates early on server-side partial pages
**File:** `internal/infrastructure/geo/wikidata/client.go:111`  
**Summary:** `if len(bindings) < pageSize { break }` — if a non-final page returns fewer rows than `pageSize` due to a Wikidata server-side soft limit, pagination stops and the remaining data is silently omitted.  
**Failure scenario:** Wikidata SPARQL returns 9 800 of 10 000 requested rows on an intermediate page due to a per-query row cap. The loop breaks and all airports/cities after that offset are dropped. The sync completes with exit code 0 and no error.

---

#### 7. Rate limiter scoped to all `/api/v1`, fires before JWT
**File:** `internal/presentation/http/router.go:85`  
**Summary:** `r.With(limiter.Handler).Route("/api/v1", ...)` applies the rate limit to every route including `/users` and `/users/me`, contrary to the comment "per-IP cap for the airports endpoint". The limiter also fires before JWT validation.  
**Failure scenario:** A bot hammering `/api/v1/airports` consumes rate-limit tokens for `/api/v1/users/me`, throttling legitimate users on unrelated endpoints. Unauthenticated requests (which return 401) still consume tokens, enabling a denial-of-service against clients sharing the same NAT IP.

---

### 🏛 Architecture

#### 8. Domain repository interface leaks DB surrogate key (`cityID int`)
**File:** `internal/domain/repository/airport_repository.go:27`  
**Summary:** `Upsert` accepts `cityID int` — a database foreign-key concept absent from the domain entity `Airport`, which embeds `City` as a struct.  
**Impact:** Couples the domain interface to the DB auto-increment ID scheme. If city identity changes (e.g. to UUID), the domain interface must change too — inverting the intended dependency direction.

---

#### 9. Login handler bypasses application layer, holds `*db.Queries` directly
**File:** `internal/presentation/http/api/login/handler.go:21`  
**Summary:** The handler holds `*db.Queries` (sqlc-generated infra type) and calls `queries.GetUserByEmail` directly, short-circuiting `application/command` and `application/query` layers used by every other endpoint.  
**Impact:** Adding brute-force lockout, audit logging, or MFA requires modifying the HTTP handler directly. Auth logic is untestable without a real DB. `container.go:123` wires `loginhttp.NewHandler(queries, ...)` with a raw sqlc type — the only Presentation → Infrastructure shortcut in the codebase.

---

#### 10. Dead `logWriter` function blocks lint gate
**File:** `internal/application/command/sync_airports/handler.go:243`  
**Summary:** `logWriter` is defined but never called — all output goes through an inline `log` closure (line 81). `golangci-lint` will flag it as unreachable code (U1000).  
**Impact:** If `golangci-lint run` is a required CI gate (per CLAUDE.md Validation Gates), the PR cannot merge. Also signals an incomplete refactor where the intended nil-safe writer was not threaded through.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical / High | 3 |
| Medium | 4 |
| Architecture | 3 |
| **Total** | **10** |

**Must fix before merge:** findings 1, 2, 3 (country filter broken, public cache on auth endpoint, compile error in tests).
