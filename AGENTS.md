# Project rules

This service handles private account allowance snapshots. It exposes no public
plan catalog. Deploy it separately from provider-plan-catalog, with independent
credentials, storage and network policy. Keep credentials out of source, images,
logs and examples. Reader tokens and collector tokens are distinct.

Use ordinary Go packages and explicit structs. Validate snapshot versions,
timestamps, scopes and numeric bounds. Unknown or expired allowance is never
unlimited capacity. Run `go test -race ./...` and `go vet ./...` before release.
Build images through CI; the homelab host only pulls them.
