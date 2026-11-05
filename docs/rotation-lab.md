# Browser rotation lab

> Status: Implemented on the feature branches; deployment not performed · Last updated: 2026-11-05

The optional `/lab` page demonstrates rotation refinement on a visitor's actual
Enka showcase build. Both candidate search and fight evaluation run in a Web
Worker on the visitor's device. The Go server fetches Enka profiles and provides
the selected build, main-library rotation and static assets; it does not run a
search or simulation for lab requests.

The lab is implemented on `feat/browser-rotation-lab` in Gscouter and
gscouter-optimizer. It is separate from the production site's normal scoring
path and disabled unless `-rotation-lab DIR` or `GSCOUTER_ROTATION_LAB=DIR` is set.
No production engine pin, deployment timer or published image is changed.

## Scope

- Select four characters from an Enka showcase. The first version requires a
  supported matching main-library team and begins with that team's rotation.
- The GSO pilot's dictionary proposals refine the rotation with fixed equipment.
  Four-member builds keep their actual constellations and equipment.
- Search estimates and the independent final baseline/candidate comparison are
  reported separately. Results are bounded searches, not claims of optimal play.
- Native GSO's old process-CPU budgets are not portable to the browser. The lab
  uses evaluation limits and a separate elapsed-time guard; runtime varies with
  the visitor's device. Worker termination stops local computation.
- The WASM embeds `gscouter-engine` at
  `b13b7864163e75852d4b9f282be009c94369ab29`, the main revision selected for this
  implementation. Search and final evaluation use this same engine. Updating
  the pin requires rebuilding and revalidating the browser crate and assets.
- The lab objective is a 450-second fight excluding the first 150 seconds, with
  the frozen target/energy conditions in `lab-static/manifest.json`. These
  numbers should not be compared directly with normal Gscouter scores, which
  use a different scoring window.

## Build locally

Requirements: Git, Python **3.11+**, Rust with Cargo and the
`wasm32-unknown-unknown` standard library, and Go 1.27+ (or Go's automatic
toolchain download). Node 22+ is used by the WASM smoke check. Neither a GPU nor
an inference server is needed.

Keep the two feature checkouts beside one another, for example:

```text
projects/
  gscouter-lab/                 # Gscouter feat/browser-rotation-lab
  gscouter-optimizer-browser/   # optimizer feat/browser-rotation-lab
```

From the Gscouter checkout:

```sh
rustup target add wasm32-unknown-unknown
bash scripts/build-rotation-lab.sh ../gscouter-optimizer-browser
go run ./cmd/server -addr 127.0.0.1:8080 -rotation-lab ./lab-static
```

Open `http://127.0.0.1:8080/lab`. A public UID must have its showcase details
enabled in the game. Browser requests to the lab server need access to
Enka.Network; fight calculations need no network once the inputs and assets
have loaded. The lab does not require the old Gscouter Go WASM build.

The build command accepts an optional output directory as its second argument:

```sh
bash scripts/build-rotation-lab.sh /path/to/gso-browser /path/to/lab-static
```

`GSCOUTER_LAB_BUILD_DIR` optionally relocates Cargo's build caches. The command
extracts the action dictionary from exactly `teams/*/rotation.gcsl`, never
`alt/` rotations, then builds the new browser crate. The historical native CLI
is used only to extract dictionary data, not to evaluate fights.

The generated directory contains:

| File | Purpose |
|---|---|
| `gso.wasm` | Browser inference and its embedded latest-pinned GSE |
| `dictionary.json` | Main-library action blocks for the pilot |
| `manifest.json` | Engine revision, source revisions/content identities, hashes and scoring conditions |

`lab-static/` is ignored by Git. The manifest records a dirty flag and source
content hashes so local source changes are not misrepresented as clean commit
builds. For a reproducible release, commit both source branches, rebuild, retain
the three assets together and keep both Git SHAs. Stop the lab before replacing
its assets; the server validates the matching files at startup.

## Build the separate container

Build assets first on a machine with Rust/Python, then run from Gscouter:

```sh
docker build -f Dockerfile.lab -t gscouter-rotation-lab:local .
docker run --rm -p 127.0.0.1:8081:8080 gscouter-rotation-lab:local
```

Open `http://127.0.0.1:8081/lab`. This direct local port exposes the regular Go
server's other routes too. Use the restricted reverse-proxy configuration below
for public lab hosting so ordinary server-side scoring is not exposed.
`Dockerfile.lab` builds the Go server and
packages the existing validated lab assets. It does not download a moving GSO
branch, publish an image or change `ghcr.io/cenoda/gscouter:latest`. The image is
built for the Docker builder's architecture; build on the deployment server or
use the appropriate Docker buildx platform configuration when transferring it.

For another server, transfer the image (or use a registry under a separate tag):

```sh
docker save gscouter-rotation-lab:local | gzip > gscouter-rotation-lab.tar.gz
# Copy the archive and deploy/lab directory to the destination, then:
gzip -dc gscouter-rotation-lab.tar.gz | docker load
```

No credentials or secret API tokens are required by the lab. The image contains
public library data and application code, not user profiles.

## HTTPS on a dedicated host

Use a domain/subdomain whose DNS A/AAAA record points to the host. Allow inbound
TCP 80/443 (and optionally UDP 443 for HTTP/3). Ports 80/443 must be free; this
profile should not be started on a host where production Caddy already owns
them.

```sh
cd deploy/lab
cp .env.example .env
# Edit LAB_DOMAIN=lab.your-domain.example; retain or set LAB_IMAGE.
docker compose --profile https up -d
docker compose --profile https logs -f
```

Open `https://<LAB_DOMAIN>/lab`; `/` redirects there. The public proxy serves
only `/lab`, its child routes and the site's favicon assets; normal Gscouter
scoring routes return 404. Caddy obtains and renews
the TLS certificate. The separate Compose project and network are named
`gscouter-rotation-lab`, and it has its own Caddy volumes. No production short
links, usage database or auto-update service is attached.

To deploy an updated image under the same tag:

```sh
docker compose --profile https up -d --force-recreate lab
```

To stop this deployment without touching production:

```sh
docker compose --profile https down
```

Retain the volumes to preserve Caddy certificates. Keep previous source SHAs
and images to roll back a release.

## Behind an existing reverse proxy

Run only the lab service (`docker compose up -d lab`), then route a separate
hostname to `lab:8080` on the `gscouter-rotation-lab` Docker network. Configure
the existing proxy to join that network persistently in its own Compose file.
For a Caddy container on that network, a site block is:

```caddyfile
lab.your-domain.example {
    encode gzip
    @root path /
    redir @root /lab 302
    @lab path /lab /lab/* /favicon.ico /favicon.svg /apple-touch-icon.png
    handle @lab {
        reverse_proxy lab:8080
    }
    handle {
        respond "Not found" 404
    }
}
```

Do not start the dedicated-host Caddy profile in this arrangement. The existing
Gscouter deployment's update timer **replaces its Caddyfile from main every five
minutes**. Editing that live file alone is not persistent. Use a separately
maintained proxy configuration or adjust the configuration source/update
workflow deliberately before adding the site block. A dedicated host avoids
any change to the production deployment.

## Validation and operating limits

Run the browser-core tests in the optimizer checkout and the web-handler tests
in Gscouter before deployment:

```sh
cargo test --locked --manifest-path ../gscouter-optimizer-browser/browser/Cargo.toml
go test ./internal/web ./cmd/server
node scripts/smoke-rotation-lab.mjs ./lab-static ../gscouter-optimizer-browser --native-parity
```

Local execution QA on 2026-11-05 covered the browser core's six tests and the
native pilot's 19 tests. The shipped WASM smoke completed 192 actual fight
evaluations and five search decisions with a 256-fight search cap, then finished
the independent 16-sample audit. This short run retained its baseline. The
native replay produced the same selected GCSL and counters, and the baseline,
selected and paired statistics matched within `max(1e-7, abs(native)*1e-10)`.
The smoke also checked requested finish, audit completion and no extra work
after completion.

Chromium integration QA used a synthetic C0 Enka profile through the real HTTP
handlers and real WASM, including start, progress, completion, immediate stop,
restart and requested finish. Request inspection confirmed that results were
not uploaded for remote calculation. This is execution and integration QA,
not a rotation-effectiveness benchmark or a live Enka-service availability
check. Docker was unavailable in the development environment: Compose YAML
and routing configuration were inspected, but container startup and public TLS
must be verified on the deployment host.

Keep the exact manifest with reported failures, plus the selected team, seed,
evaluation limit and browser version. Enka may reject a hidden showcase or
temporarily rate-limit profile requests. An unsupported build or rotation is
shown as an error rather than silently evaluated on another engine.

The browser may throttle a background tab or terminate a worker on a low-memory
device. Cancellation and elapsed-time limits therefore stop work instead of
promising a fixed completion time. A live search estimate is not a completed
independent audit; retain the final report's status when sharing results.
