# Model capacity

A small authenticated service for fresh account allowance snapshots. Clients
such as model gateways and agents consume `GET /v1/allowances` using a reader
token. The collector has a different token and returns the versioned contract in
`capacity/types.go`.

```sh
go test -race ./...
go vet ./...
```

Configure `COLLECTOR_URL`, `COLLECTOR_TOKEN_FILE` and `READER_TOKEN_FILE`. Tokens
must be distinct and at least 32 characters. Mount their files at runtime; keep
them out of images and source. `LISTEN_ADDR` defaults to `:8388`.

The service refreshes every five minutes and keeps snapshots in memory. An
upstream failure preserves the previous observation and expiry. Expired data
returns HTTP 503. Account-wide and model-specific allowances have explicit scopes.
Unknown allowance is never unlimited capacity. The initial collector integration
is CLIProxy's restricted allowance endpoint.

Deploy behind TLS on a private network. All account responses require a bearer
token and use `Cache-Control: no-store`. `/healthz` returns liveness only. The
service forwards no client-controlled URLs, follows no upstream redirects, and
limits source responses to 4 MiB. Provider credentials stay with the collector.
Logs contain no tokens, account responses or upstream error bodies.

This program has no public plan catalog endpoint. Public documentation facts are
published by the independent `provider-plan-catalog` project, which has separate
code, image, deployment and storage. Its runtime has no private account connector.
