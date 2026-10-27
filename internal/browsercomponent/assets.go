package browsercomponent

import _ "embed"

// These build inputs are byte-checked against the manual deployment assets.
// Keep their provenance with packaging/browser rather than a second generator.
//
//go:embed assets/managed-compose.yml
var composeTemplate []byte

//go:embed assets/browser-seccomp.json
var seccompProfile []byte

const componentImage = "docker.io/kjlion/kpanel-browser@sha256:fbc00e5b7e5bb39c117507b1995cc0e4f85976eb6818db23322d46968f7d5697"
