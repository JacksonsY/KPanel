// Package browserassets contains the same optional component configuration used
// by manual Compose deployments and the Agent's fixed lifecycle adapter.
package browserassets

import _ "embed"

//go:embed managed-compose.yml
var Compose []byte

//go:embed browser-seccomp.json
var Seccomp []byte

const Image = "docker.io/kjlion/kpanel-browser@sha256:fbc00e5b7e5bb39c117507b1995cc0e4f85976eb6818db23322d46968f7d5697"
