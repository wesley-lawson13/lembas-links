# Lembas Links — AWS EC2 Deployment

## Context

The README's roadmap is: *"learn AWS by redeploying the project there: first a manual EC2
deployment, then codifying that infrastructure with Terraform, then automating deployments
with GitHub Actions, and finally adding observability."* This plan covers step one — the
manual EC2 deployment — and deliberately shapes the infrastructure so steps two and three
are additive rather than a rewrite.

Two earlier drafts each missed something. The first (4 instances, Redis in a public subnet
with no restrictions, Elastic IPs "so the instance doesn't lose its contents") had inverted
security-group directions and conflated a static IP with data durability. The second was
sound on VPC fundamentals but was written for a single-instance app — it collapsed to one
security group with two inbound rules, which skips security-group-to-security-group
referencing, the most valuable AWS networking concept for a backend engineer.

**Constraints driving this plan:** two instances maximum, no managed NAT gateway, keep it
simple — while still exercising SG-to-SG rules. The initial $10/month target proved
incompatible with two instances (the on-demand floor is ~$17); ~$17–21 was accepted once it
was clear the cheaper reductions cost nothing in learning and the pricier ones bought none.

**Outcome:** a custom VPC, two `t3.micro` instances, five security groups, one public
origin, and roughly a dozen Terraform resources waiting to be written in Week 2.

---

## The key idea: security groups are role labels, not per-instance firewalls

An instance can carry up to five security groups. That means the four-SG design does **not**
require four instances — attach multiple SGs to each box and you get identity-based
firewalling on two.

| Security group | Inbound rule | Attached to |
|---|---|---|
| `sg-web` | 80, 443 ← `0.0.0.0/0` | app |
| `sg-ssh` | 22 ← `YOUR_IP/32` | app, data |
| `sg-api` | **none** — pure identity label | app |
| `sg-postgres` | 5432 ← **source: `sg-api`** | data |
| `sg-redis` | 6379 ← **source: `sg-api`** | data |

`sg-api` having zero inbound rules is the point, not an oversight. nginx reaches the API over
`127.0.0.1` so it needs no ingress — the group exists purely so `sg-postgres` and `sg-redis`
have something to name. Rules reference *identity*, not addresses, so instances can be
replaced and their IPs can change without touching a rule.

Outbound: leave at the default allow-all. NACLs: leave at default allow-all — they are
stateless, and hand-editing them is a classic way to block return traffic and lose an hour.

---

## Network

```
VPC  10.0.0.0/16          enable_dns_hostnames = true   ← defaults to FALSE in Terraform
├── subnet-public-a   10.0.1.0/24   us-east-1a   ← both instances live here
├── subnet-public-b   10.0.2.0/24   us-east-1b   ← intentionally EMPTY
├── internet gateway
└── route table:  10.0.0.0/16 → local
                  0.0.0.0/0   → igw        ← this route is what makes a subnet "public"
```

`subnet-public-b` stays empty and costs nothing. An ALB requires subnets in ≥2 AZs, and so
does an RDS subnet group — defining it now means Week 3 is additive instead of a renumbering
exercise.

No private subnet in Week 1. A private-subnet instance has no route to the internet, so it
cannot `docker pull postgres:15` without a NAT gateway (~$32/mo, over budget). See
*Deferred* for the free path to a private tier.

Tag every resource `Project = lembas-links` so Billing can filter by tag.

---

## Instances

Both are **`t4g.micro` (Graviton, arm64)** — see *Build strategy* for why.

**`lembas-app`** — public subnet A, Elastic IP, `sg-web` + `sg-ssh` + `sg-api`
- nginx (`:80`) — serves the built SPA and reverse-proxies API paths to `127.0.0.1:8080`
- Go API (`:8080`, published only on loopback)

**`lembas-data`** — public subnet A, auto-assigned public IP, `sg-ssh` + `sg-postgres` + `sg-redis`
- `postgres:15` + `redis:7-alpine`, both published on the **private IP only**, not `0.0.0.0`

The data box sits in a *public* subnet but is not publicly reachable — its only ingress is
5432/6379 from `sg-api` and 22 from your IP. **Public subnet is a routing decision; the
security group is the security boundary.** It keeps a public IP solely for outbound egress
(pulling images, `apt`).

The Elastic IP on the app box is not optional. Short links are promises — an auto-assigned
public IP changes on stop/start, which would break every link already handed out and
invalidate `BASE_URL`.

Cross-host addressing uses the data box's **private IP** directly in `DATABASE_URL` /
`REDIS_URL`. It is stable for the life of the instance and changes only on terminate/recreate.
(Note: `enable_dns_hostnames` governs *public* DNS names and is unrelated to this; internal
`ip-10-0-1-x.ec2.internal` resolution comes from `enableDnsSupport`, which defaults to true.
A Route 53 private hosted zone — `db.lembas.internal` — is the clean Week 2 upgrade.)

### Build strategy — build on the Mac, never on the instance

`t4g.micro` has 1 GB RAM and 2 burstable vCPUs. `go build` (golang-migrate drags in quic-go
and mongo-driver) plus `npm install` will thrash. T3/T4g default to *unlimited* mode, so
instead of throttling you quietly accrue surplus-credit charges.

Build locally instead. The commonly-cited blocker — "arm64 Mac can't build for the
instance" — does not apply here:
- Go cross-compiles with no toolchain setup (`GOOS=linux GOARCH=amd64 go build`).
- `vite build` emits static JS/CSS/HTML, which has no architecture at all.

Choosing **`t4g.micro` (arm64)** removes even that: the local machine is `arm64`, so Docker
builds are native-speed with no QEMU emulation, and t4g is ~19% cheaper than t3. Every
required image publishes arm64 — `golang:1.25-alpine`, `node:22-alpine`, `postgres:15`,
`redis:7-alpine`, `nginx:alpine`.

Ship images rather than source: push to ECR, or `docker save | ssh … docker load` for Week 1.
Instances only ever pull. This is also the shape GitHub Actions needs in Week 3.

Still add ~2 GB of swap on both boxes as a safety margin for runtime, not for builds.

---

## One origin, no CORS

nginx on the app box serves the SPA and proxies API paths, so the browser only ever talks to
one address. This removes CORS entirely, needs one Elastic IP instead of two, and makes the
later certbot/ACM step a single certificate.

The wrinkle is that `GET /:slug` (`api/main.go:100`) sits at the root and collides with the
SPA routes `/` and `/stats/:slug`. nginx's longest-prefix matching resolves it cleanly — no
regex needed:

```nginx
root /usr/share/nginx/html;

location = /             { try_files /index.html =404; }
location = /favicon.ico  { try_files $uri =404; }
location /assets/        { try_files $uri =404; }
location /stats/         { try_files $uri /index.html; }   # SPA deep link

location /session { proxy_pass http://127.0.0.1:8080; }
location /links   { proxy_pass http://127.0.0.1:8080; }
location /health  { proxy_pass http://127.0.0.1:8080; }
location /swagger { proxy_pass http://127.0.0.1:8080; }

location /        { proxy_pass http://127.0.0.1:8080; }    # catch-all → slug redirects
```

Because everything is same-origin, `VITE_API_BASE_URL` becomes the **empty string** and
`frontend/src/api.ts` issues relative fetches (`/session`, `/links`). `CORS_ALLOWED_ORIGINS`
can be left empty — `api/main.go:69` already skips registering the CORS middleware entirely
when it is unset.

---

## Code changes (blocking — the deploy fails without these)

### 1. Consolidate the two Dockerfiles — `docker-compose.yml`, delete `api/Dockerfile`

`api/migrate/migrate.go` hardcodes `file:///db/migrations` and reads `/db/seeds/quotes.sql`,
but `api/Dockerfile` never copies them — Compose supplies them via the `./db:/db` bind mount.
Building that image on EC2 hits `log.Fatalf` at boot. The root `Dockerfile` already does the
right thing (`COPY db/migrations/`, `COPY db/seeds/`) and is multi-stage.

Point Compose at the root Dockerfile so dev and prod build the same image:

```yaml
api:
  build:
    context: .
    dockerfile: Dockerfile
```

### 2. Trusted proxies — `api/main.go`

`SetTrustedProxies` is never called, so Gin trusts all proxies and `c.ClientIP()` returns a
client-supplied `X-Forwarded-For`. Anyone can spoof it for unlimited rate-limit buckets,
defeating both the per-IP limit and the `SESSION_RATE_LIMIT` backstop on free key minting
(`api/middleware/rate.go:21,72`), and poisoning click analytics (`api/handlers/redirect.go:25`).

nginx is on the same host, so:

```go
r := gin.Default()
if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil { log.Fatal(err) }
```

nginx must also forward the header: `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`

### 3. Redis connection retry — `api/db/redis.go`

Postgres retries 10× with 2s sleeps; Redis does a single ping then `log.Fatalf`. With the
data box now a *separate machine*, the app box will regularly boot first and crash-loop.
Mirror the retry loop from `db.NewPool` in `api/db/db.go`.

### 4. Real health check — `api/main.go:86`

Currently returns hardcoded `"database":"connected","cache":"connected"` without touching
either. Replace with an actual `pool.PingContext` + `redis.Ping` under a ~2s timeout,
returning 503 on failure. Needed for your own debugging now, and required before an ALB in
Week 3.

### 5. Production frontend image — new `frontend/Dockerfile.prod` + `frontend/nginx.conf`

`frontend/Dockerfile` runs `npm run dev` against a bind mount and cannot ship. Multi-stage:
node stage runs `npm run build`, nginx stage serves `dist/` with the config above.
`VITE_API_BASE_URL` is inlined by Vite at **build** time, so pass it as a build ARG.

Add a fallback in `frontend/src/api.ts:12` so an unset var doesn't produce `undefined/session`:

```ts
const API_BASE = (import.meta.env.VITE_API_BASE_URL as string) ?? ''
```

### 6. Compose files for each box — `docker-compose.app.yml`, `docker-compose.data.yml`

Split the current single file. Both need `restart: unless-stopped` (the current compose has
no restart policy, so nothing survives a reboot). The API publishes to `127.0.0.1:8080:8080`,
not `0.0.0.0`. Postgres/Redis publish on the data box's private IP.

### 7. Production env values

| Variable | Value |
|---|---|
| `BASE_URL` | `http://<app-elastic-ip>` — unvalidated; blank silently ships broken short links |
| `CORS_ALLOWED_ORIGINS` | *(empty — same origin)* |
| `DATABASE_URL` | `postgres://…@<data-private-ip>:5432/lembas_links?sslmode=disable` |
| `REDIS_URL` | `redis://<data-private-ip>:6379` |
| `GIN_MODE` | `release` — never set anywhere today, so prod runs in debug mode |

`sslmode=disable` stays correct here (self-hosted Postgres serves no TLS); it must change to
`require` if Postgres moves to RDS. Do **not** copy the local `.env` to the box — it contains
a commented plaintext production key.

### 8. Credentials on the data box — `docker-compose.data.yml`

Now that Postgres and Redis are reachable over the network rather than a Docker bridge, add:

- `redis-server --requirepass <generated>`, and `REDIS_URL=redis://:<pass>@<private-ip>:6379`
  (`redis.ParseURL` in `api/db/redis.go` already handles the password form).
- A generated `POSTGRES_PASSWORD`, not a memorable one.
- Publish both on the private IP only (`<private-ip>:5432:5432`), never `0.0.0.0`.

The argument is **not** wire encryption — traffic between two ENIs in one subnet is a weak
threat model, and an attacker on the app box can already read `DATABASE_URL` from the env.
The argument is insurance against a security-group misconfiguration, and it is stronger for
this app specifically: `api/middleware/rate.go:39` builds keys as `rate:key:<plaintext API
key>`, so an exposed Redis leaks **live credentials**, not just a cache. Hashing that key
material is the real fix and stays in *Deferred*.

---

## Data durability — decide before you deploy

Postgres data lives on the data box's EBS **root** volume, which is deleted on instance
termination by default. `migrate.SeedQuotesIfEmpty` (`api/main.go:51`) reloads the 340-slug
pool automatically, but nothing reseeds `urls`, `api_keys`, or `clicks` — a teardown means
every short link anyone created returns 404 forever.

Recommended: attach a **separate 10 GB `gp3` EBS volume**, mount it at
`/var/lib/postgresql/data`, and keep it out of the destroy path. ~$0.80/month, survives
instance replacement *and* `terraform destroy`, and it teaches the EBS lifecycle distinction
(root volumes are ephemeral by default, attached volumes are not). Add a snapshot schedule
later for an actual backup story.

---

## Cost

**First, confirm which free tier the account is on** (Billing console). Accounts created before
~mid-2025 get the classic 12-month tier: 750 instance-hours + 750 IPv4-hours per month.
Accounts created after get a **credit-based plan** ($100 + up to $100 more) with *no* hourly
allowance. A mention of "$200 in credits" indicates the credit plan, and every line below
assumes it — nothing is free, everything is on-demand.

us-east-1 on-demand, 730 h/month:

| Setup | Monthly | $200 credits last |
|---|---|---|
| 2 × `t3.micro`, 2 public IPs, 16 GB gp3 | $23.76 | 8.4 months |
| 2 × `t4g.micro`, 2 public IPs, 16 GB gp3 | $20.84 | 9.6 months |
| **2 × `t4g.micro`, data box NAT'd behind app box (1 public IP)** | **$17.19** | **11.6 months** |
| Same, running ~8 h/day weekdays via a stop/start schedule | $7.79 | 25 months |

Unit rates to check against the AWS Pricing Calculator: `t3.micro` $0.0104/h, `t4g.micro`
$0.0084/h, public IPv4 $0.005/h (charged even when attached, since Feb 2024), gp3 $0.08/GB-mo.

**Two instances cannot hit $10/month on-demand.** The floor is ~$17. Getting to $10 means
either a stop/start schedule or dropping back to one instance — and one instance loses the
SG-to-SG lesson this whole design exists to teach.

**The $17.19 row is the target, and not because it is cheapest.** Neither reduction
compromises the learning; two of them improve it:

- `t4g.micro` over `t3.micro` adds Graviton/arm64 to the picture — a real production choice,
  a defensible interview answer, and native-speed builds from an arm64 laptop.
- The NAT instance is the **richest single exercise in this deployment**, not a cost tweak.
  It is the only step that builds an actual private subnet, and it teaches a route table
  targeting an ENI, kernel IP forwarding, iptables MASQUERADE, and disabling the
  source/destination check — an AWS-specific concept that never comes up otherwise. The
  $23.76 row skips private subnets entirely.

The stop/start schedule is the one reduction to decline: it saves the most, teaches the least,
and a demo that is powered off when someone clicks the link is a bad trade during a job search.
Revisit it afterwards.

**Set the AWS Budget on day one, not later.** Two budgets are free; alert at 50/80/100% of $25
so the real number has headroom to warn rather than breach silently.

---

## Phasing

The NAT instance is the destination, not the starting point. A first deploy that combines a
private subnet, a custom route table, IP forwarding, five security groups, a compose split,
and a new nginx config gives you no way to tell which layer is failing.

| Phase | Scope | Monthly |
|---|---|---|
| **1** | `t4g.micro` from the start, **both boxes in the public subnet**, five SGs, single origin. Verify end to end. | ~$20.84 |
| **1.5** | Move the data box to a private subnet; make the app box a NAT instance. One discrete change, verified on its own. | ~$17.19 |
| **2** | Write the Terraform — **against the phase-1.5 architecture**. | — |
| **3+** | GitHub Actions, then ALB + ACM + domain, then observability. | — |

Phase 1.5 comes **before** Terraform deliberately. Codify the final architecture once rather
than writing it for an intermediate state and immediately rewriting it.

Phase 1.5 verification: from the data box, `curl -I https://registry-1.docker.io` succeeds
(egress works through the NAT). From your laptop, any connection to the data box times out,
and it no longer has a public IP to target.

---

## Deferred (and why)

- **Managed NAT gateway** — ~$32/mo, more than every other line combined. Phase 1.5 uses a
  **NAT instance** on the app box instead (IP forwarding, iptables MASQUERADE, source/dest
  check disabled, the private subnet's `0.0.0.0/0` routed at its ENI). Same lesson, ~$0.
  Deferred only in the sense that a managed NAT is never worth it at this scale.
- **RDS** — the other free route to a private subnet: RDS needs no outbound internet, so it
  needs no NAT. Buys private subnets, a subnet group, automated backups, and durable data
  across `terraform destroy` for less than the NAT gateway alone.
- **ALB + ACM** — needs 2 AZs (already provisioned) and a working `/health` (item 4). ~$16–22/mo.
- **HTTPS** — certbot on nginx once a domain exists. ACM certs cannot be installed on a bare
  EC2 instance, only on an ALB or CloudFront.
- **Graceful shutdown** — `r.Run()` has no signal handling and `asyncRecordClick`
  (`api/handlers/redirect.go:74`) fires untracked goroutines, so every redeploy drops
  in-flight redirects and pending click writes. Worth fixing, not blocking.
- **Raw API keys in Redis key names** — `rate:key:<plaintext>` (`api/middleware/rate.go:39`).
  Postgres correctly stores only SHA-256; hash before using as a Redis key.
- **Swagger `@host`** — `api/main.go:4` still says `lembas-links-production.up.railway.app`;
  update and re-run `make docs`. Delete `railway.toml`.
- **Multi-AZ, autoscaling, observability** — out of scope; `subnet-public-b` is the hook.

---

## Verification

1. **Security groups actually gate traffic.** From the app box, `nc -zv <data-private-ip> 5432`
   succeeds. From your laptop, the same command against the data box's *public* IP times out.
   That contrast is the whole lesson — confirm both directions.
2. **Boot order tolerance.** Reboot the data box while the app box is running. The API should
   retry and recover rather than crash-loop (item 3).
3. **End-to-end smoke test.** `scripts/e2e_smoke_test.sh` already takes `API_URL` and `ORIGIN`
   from the environment: `API_URL=http://<eip> ORIGIN=http://<eip> make e2e`.
4. **SPA deep link.** Load `http://<eip>/stats/<slug>` directly in a fresh tab — a 404 means
   the `try_files` fallback is wrong.
5. **Slug redirect at the root.** `curl -I http://<eip>/<slug>` returns 302 with a `Location`
   header, confirming the nginx catch-all reaches the API rather than serving `index.html`.
6. **Rate limiting is not spoofable.** `curl -H 'X-Forwarded-For: 1.2.3.4' http://<eip>/health`
   repeatedly; the limiter must still count these against your real IP (item 2).
7. **Health reflects reality.** Stop the Postgres container; `/health` must return 503, not 200.
8. **Restart survival.** Reboot both instances. Containers come back on their own
   (`restart: unless-stopped`) and existing short links still resolve.

---

## Documentation deliverables

Written after the deploy is verified, so they describe what actually happened rather than what
was intended.

### 1. `README.md` — two edits

**New `## Deployment` section**, placed after `## Tech Stack` and added to the table of
contents. Contents: an ASCII diagram of the VPC (subnets, IGW, the two instances, which SGs
attach where), the five-security-group table with `sg-api` called out as an identity-only
group, the live URL, the production env vars that differ from local, and the deploy procedure
(build images on the Mac → ship → `docker compose up -d` per box). Keep the existing local
Docker Compose instructions intact and unchanged — they remain the way to run this locally.

**Update `## Learning Objectives and Future Plans`.** Line 75 currently reads:

> Currently, my next goal is to learn AWS by redeploying the project there: first a manual EC2
> deployment, then codifying that infrastructure with Terraform, then automating deployments
> with GitHub Actions, and finally adding observability.

Rewrite so the manual EC2 deployment is described in the past tense as completed, with
Terraform / GitHub Actions / observability as what remains. Add a bullet to the skills list
covering AWS networking fundamentals — VPC, subnets, route tables, internet gateways, and
identity-based security group rules. Also update the intro paragraph, which currently ends
the project's arc at containerization with Docker Compose.

### 2. `.claude/decisions/AWS-deploy.md`

The decision record for what was built. One section per decision, each stating the choice,
the alternatives rejected, and the reasoning — written so a reader can tell *why*, not just
*what*. Cover at minimum:

- Custom VPC over the default VPC
- Two instances rather than one or four — and the SG-as-role-label insight that made two
  sufficient to exercise SG-to-SG referencing
- Five security groups across two boxes, including why `sg-api` has no inbound rules
- Public subnets with SG isolation instead of a private subnet, and the NAT gateway's ~$32/mo
  as the deciding factor
- Single origin with nginx reverse-proxying, and how the `GET /:slug` vs `/stats/:slug`
  collision resolves via longest-prefix matching — plus the consequence that CORS disappears
- `t4g.micro`/Graviton for native arm64 builds and lower cost
- Self-hosted Postgres/Redis over RDS/ElastiCache, and the durability tradeoff accepted
- The empty second subnet as a deliberate hook for a future ALB and RDS subnet group
- Each blocking code change and the AWS-specific reason it was required

### 3. `.claude/decisions/AWS-scalability.md`

How this deployment would change to follow production best practice, framed as the gap
analysis an interviewer would ask for. Roughly:

- **Availability** — single AZ, single instance per role, no health-based replacement. Path:
  ALB across the two subnets already provisioned, an Auto Scaling Group with a launch template,
  instances in private subnets.
- **Data** — Postgres on an EBS root volume with no backups and no failover. Path: RDS
  Multi-AZ, automated backups, PITR. Note that RDS also delivers private subnets for free,
  because it needs no NAT.
- **Cache** — single Redis with no persistence tuning, and the API treats it as a *hard boot
  dependency* (`api/db/redis.go` calls `log.Fatalf` on a failed ping). Path: ElastiCache with
  a replica, and degrade to Postgres-only rather than refusing to boot. Note that
  `SessionRateLimit` deliberately fails closed (`api/middleware/rate.go`), so a cache outage
  currently blocks all new-visitor onboarding.
- **Statelessness** — what already scales horizontally (the API holds no local state) versus
  what does not (migrations run in-process on every boot; concurrent replicas would contend on
  golang-migrate's lock). Path: a separate migration task in the deploy pipeline.
- **Request path** — per-redirect goroutines writing two rows against a 25-connection pool
  (`api/handlers/redirect.go:74`, `api/db/db.go`); no index on `urls.api_key`; `clicks` and
  `api_keys` grow unbounded. Path: batch or queue click writes, add the missing index, add a
  retention policy.
- **Deployment** — manual SSH, no rollback, no graceful shutdown. Path: Terraform,
  GitHub Actions, immutable images, `srv.Shutdown` with connection draining.
- **Observability** — `log.Printf` to stdout with no structure or request IDs. Path: structured
  logging, CloudWatch Logs, metrics on redirect latency and cache hit rate, alarms.
- **Security** — Swagger public, no security headers, no WAF, no secret manager. Path: SSM
  Parameter Store, security-header middleware, WAF rate rules ahead of the app-level limiter.

Be explicit about which gaps are *deliberate* for a demo and which are genuine debt.

### 4. `.claude/learning/AWS-learning-outcomes.md`

Create `.claude/learning/` — it does not exist yet.

A study document, written to be re-read before an interview, walking the AWS fundamentals this
deployment exercised and the reasoning that led to each choice. Structure it around the
concepts rather than the chronology:

- **VPC and CIDR** — why a private RFC1918 range, why `/16` and `/24`, why room to grow matters
- **Subnets** — that "public" is not a property of the subnet but of its route table, and that
  AZ placement is a fault-tolerance decision
- **Internet gateway and route tables** — `0.0.0.0/0 → igw` as the single line that makes a
  subnet public; `local` routes as implicit
- **NAT** — why a private instance has no egress, what a NAT gateway costs, and why a NAT
  instance or a managed service that needs no egress (RDS) are the cheap ways out
- **Security groups vs NACLs** — stateful vs stateless, why NACLs stay at default, and the
  central insight: SGs are role labels attached to instances, referenced by identity, so a
  group with zero rules can still be meaningful
- **EC2** — instance families, burstable credits and unlimited mode, Graviton/arm64, and why
  the build host's architecture matters
- **EBS** — root volumes deleted on termination vs attached volumes that persist; snapshots as
  the actual backup story
- **Elastic IPs** — a static address, *not* durability; the conflation of the two was a real
  early error in this project's planning, and short links made a stable address genuinely
  load-bearing
- **Cost mechanics** — what actually accrues (instance-hours, IPv4-hours, GB-months), why the
  free tier changed in 2025, and why the first estimate in this plan was off by roughly half

Close with a short "misconceptions corrected" section drawn from this planning history: that a
private subnet is a security boundary (it is not — the SG is); that an Elastic IP preserves
instance contents (it does not); that a frontend server proxies browser traffic to the API (it
does not — the SPA runs in the user's browser and calls the API directly, which is why the API
is inherently internet-facing); and that an arm64 Mac cannot build for a Linux server (it can,
trivially, for Go and for static bundles alike).
