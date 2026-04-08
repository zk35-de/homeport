package api

// AppVersion is set at build time via -ldflags "-X github.com/zk35-de/homeport/internal/api.AppVersion=<tag>".
// Falls back to "dev" when built without ldflags.
var AppVersion = "dev"
