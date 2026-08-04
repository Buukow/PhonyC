package proxy

import (
	"net/http"

	"github.com/phonyg/phonyg/internal/requestheaders"
	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
)

func isHop(name string) bool { return requestheaders.IsHop(name) }

// BuildUpstreamHeaders builds final upstream headers per spec §5.4.
func BuildUpstreamHeaders(client http.Header, ch store.Channel, key store.UserKey, snap *snapshot.Snapshot, contentLength int64) http.Header {
	return requestheaders.BuildUpstreamHeaders(client, ch, key, snap, contentLength)
}
