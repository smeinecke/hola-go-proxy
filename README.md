> [!IMPORTANT]
> This is an **unofficial fork** of a project originally developed by Vladislav Yarmak ([@Snawoot](https://github.com/Snawoot)).
> The repository was reconstructed from publicly available sources.
>
> It is **not affiliated with, endorsed by, or maintained by the original author**.

# hola-proxy

Standalone Hola proxy client. Just run it and it'll start a plain HTTP proxy server forwarding traffic through Hola proxies of your choice.
By default the application listens on 127.0.0.1:8080.

Application is capable to forward traffic via proxies in datacenters (flag `-proxy-type direct`, default) or via peer proxies on residental IPs (consumer ISP) in that country (flag `-proxy-type lum`).

This alternative implementation ensures your internet connection is not shared with anyone else and everything is clean and safe.

## Features

* Cross-platform (Windows/Mac OS/Linux/Android (via shell)/\*BSD)
* Uses TLS for secure communication with upstream proxies
* Zero configuration
* Simple and straight forward
* Session reuse via a persistent session file, avoiding repeated `background_init` registrations (which are rate-limited per source IP)
* Browser-grade fingerprinting for Hola API requests via [CycleTLS](https://github.com/Danny-Dasilva/CycleTLS) (Chrome JA3/HTTP2 fingerprint, header order, extension `Origin`) and `utls` Chrome ClientHello for zagent tunnels
* Optional endpoint verification (`-check`) and local listener authentication (`-proxy-auth`)

## Installation

#### Binaries

Pre-built binaries are available [here](https://github.com/snawoot-proxies-forks/hola-proxy/releases/latest).

Don't forget to make file executable on Unix-like systems (Linux, MacOS, \*BSD, Android). For your convenience rename downloaded file to `hola-proxy` and run within directory where you placed it:

```sh
chmod +x hola-proxy
```

#### Build from source

Alternatively, you may install hola-proxy from source. Run the following within the source directory:

```
make install
```

## Usage

List available countries:

```
$ ./hola-proxy -list-countries
ar - Argentina
at - Austria
au - Australia
be - Belgium
bg - Bulgaria
br - Brazil
ca - Canada
ch - Switzerland
cl - Chile
co - Colombia
cz - Czech Republic
de - Germany
dk - Denmark
es - Spain
fi - Finland
fr - France
gb - United Kingdom (Great Britain)
gr - Greece
hk - Hong Kong
hr - Croatia
hu - Hungary
id - Indonesia
ie - Ireland
il - Israel
in - India
is - Iceland
it - Italy
jp - Japan
kr - Korea, Republic of
mx - Mexico
nl - Netherlands
no - Norway
nz - New Zealand
pl - Poland
ro - Romania
ru - Russian Federation
se - Sweden
sg - Singapore
sk - Slovakia
tr - Turkey
uk - United Kingdom
us - United States of America
```

Run proxy via country of your choice:

```
$ ./hola-proxy -country de
```

Or run proxy on residential IP:

```
$ ./hola-proxy -proxy-type lum
```

Also it is possible to export proxy addresses and credentials:

```
$ ./hola-proxy -country de -list-proxies -limit 3
Login: user-uuid-0a67c797b3214cbdb432b089c4b801cd
Password: cd123c465901
Proxy-Authorization: Basic dXNlci11dWlkLTBhNjdjNzk3YjMyMTRjYmRiNDMyYjA4OWM0YjgwMWNkOmNkMTIzYzQ2NTkwMQ==

host,ip_address,direct,peer,hola,trial,trial_peer,vendor
zagent783.hola.org,165.22.22.6,22222,22223,22224,22225,22226,digitalocean
zagent830.hola.org,104.248.24.64,22222,22223,22224,22225,22226,digitalocean
zagent248.hola.org,165.22.65.3,22222,22223,22224,22225,22226,digitalocean
```

## List of arguments

| Argument | Type | Description |
| -------- | ---- | ----------- |
| backoff-deadline | Duration | total duration of zgettunnels method attempts (default 5m0s) |
| backoff-initial | Duration | initial average backoff delay for zgettunnels (randomized by +/-50%) (default 3s) |
| bind-address | String | HTTP proxy address to listen to (default "127.0.0.1:8080") |
| cafile | String | use custom CA certificate bundle file (applies to zagent tunnel connections) |
| check | - | verify tunnel endpoints end-to-end (CONNECT tunnel + egress IP match) before starting the proxy server |
| check-url | String | URL used by -check; must return a JSON document with an "ip" field (default "https://checkip.greensec.de/json") |
| country | String | desired proxy location - any valid alpha-2 code is accepted, even if absent from -list-countries (default "us") |
| dont-use-trial | - | use regular ports instead of trial ports |
| ext-ver | String | extension version to mimic in requests. Can be obtained from https://chrome.google.com/webstore/detail/hola-vpn-the-website-unbl/gkojfkhlekighikafcpjkiklfbnlmeio (default "999.999.999") |
| force-port-field | Number | force specific port field/num (example 24232 or lum) |
| hide-SNI | Boolean | hide SNI in TLS sessions with proxy server (default true) |
| init-retries | Number | number of attempts for initialization steps, zero for unlimited retry |
| init-retry-interval | Duration | delay between initialization retries (default 5s) |
| limit | Unsigned Integer (Number) | amount of proxies in retrieved list (default 3) |
| list-countries | String | list available countries and exit |
| list-proxies | - | output proxy list and exit |
| proxy | String | sets base proxy to use for all dial-outs. Format: `<http\|https\|socks5\|socks5h>://[login:password@]host[:port]` Examples: `http://user:password@192.168.1.1:3128`, `socks5://10.0.0.1:1080` |
| proxy-auth | String | require authentication on the local proxy listener, format: `user:password`. Useful when binding to a non-localhost address |
| proxy-type | String | proxy type (Datacenter: direct) (Residential: lum) (default "direct") |
| resolver | String | comma-separated list of DNS/DoH/DoT resolvers used to lookup domain names blocked by Hola. Supported schemes are: `dns://`, `https://`, `tls://`, `tcp://`. (default `https://1.1.1.3/dns-query,https://8.8.8.8/dns-query,https://dns.google/dns-query,https://security.cloudflare-dns.com/dns-query,https://fidelity.vm-0.com/q,https://wikimedia-dns.org/dns-query,https://dns.adguard-dns.com/dns-query,https://dns.quad9.net/dns-query,https://doh.cleanbrowsing.org/doh/adult-filter/`) |
| rotate | Duration | rotate user ID once per given period (default 48h0m0s) |
| session-file | String | file to persist the Hola session (`{uuid, sessionKey}`) for reuse across restarts. Format is shared with python-hola, so both clients can use the same file. Empty string disables session persistence (default "~/.hola_session.json") |
| timeout | Duration | timeout for network operations (default 35s) |
| user-agent | String | value of User-Agent header in requests. Default: User-Agent of latest stable Chrome for Windows |
| verbosity | Number | logging verbosity (10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical) (default 20) |

## Stealth notes

The client mimics the official Chrome extension where it matters:

* API requests to `client.hola.org` go through CycleTLS with a real Chrome TLS/HTTP2 fingerprint (JA3, HTTP/2 SETTINGS, header order), the extension `Origin: chrome-extension://gkojfkhlekighikafcpjkiklfbnlmeio` header, `Accept-Language`, and the extension's common query parameters (`rmt_ver`, `ext_ver`, `browser=chrome`, `product=cws`, `lccgi=1`). The API server name is sent normally (no SNI hiding - the real extension does not hide it either).
* Raw CONNECT tunnels to zagent proxies use `utls` with a Chrome ClientHello; `-hide-SNI` applies only to these connections.
* Sessions are persisted to `-session-file` and validated via `rules_get_vpn.json` before reuse, so repeated restarts do not hammer the rate-limited `background_init` registration endpoint. The file uses mode 0600 and the same `{uuid, sessionKey}` format as [python-hola](https://github.com/smeinecke/python-hola), so credentials can be shared between the two clients.
* `blocked`/`permanent` flags in API responses are honoured: a temporary block triggers the normal retry/backoff path, while a permanent ban aborts retries instead of hammering the service.
