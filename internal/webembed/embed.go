package webembed

import "embed"

//go:embed all:dist
var DistFS embed.FS
