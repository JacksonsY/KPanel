# KPanel source backport

This directory contains the complete `github.com/anacrolix/torrent@v1.61.0`
Go module source, with only the two upstream message-writer fixes listed below.
The source and modifications remain under [MPL-2.0](LICENSE).
KPanel selects this directory through a local `replace` directive while retaining
the `v1.61.0` requirement and its existing dependency versions.

## Source integrity

- Module archive: <https://proxy.golang.org/github.com/anacrolix/torrent/@v/v1.61.0.zip>
- Module checksum: `h1:vxo+B4SwnoP5AQWbhvnTYIaTgPSX+llYUVuQVsN4Jg8=`
- Archive SHA-256: `58393811685d90f3e4f821cfc41e1b34c60019e6380750c6c8e31976707de0ef`
- Original module files: 453. Preserve these files byte for byte except for the
  two files changed by the upstream patches. This note and the machine-readable
  [source manifest](KPANEL_BACKPORT.json) are additions.

## Applied upstream changes

Apply in this order to the original module:

1. [`aa71e8d2f5c439257c0313919d671fd087347233`](https://github.com/anacrolix/torrent/commit/aa71e8d2f5c439257c0313919d671fd087347233):
   register the writer wakeup before filling its buffer, so concurrent wakeups
   are observed. Includes `TestMsgWriterBroadcastDuringFillWakesWriter`.
2. [`70072f1a7460c0fffdc1f547067c79bbd3dd3476`](https://github.com/anacrolix/torrent/commit/70072f1a7460c0fffdc1f547067c79bbd3dd3476):
   rearm the keepalive timer before parking an empty writer. Includes
   `TestMsgWriterKeepAliveTimerRearmed`.

Only `peer-conn-msg-writer.go` and `peer-conn-msg-writer_test.go` differ from the
original module. Their contents match upstream commit `70072f1a7460c0fffdc1f547067c79bbd3dd3476`:

| File | SHA-256 |
| --- | --- |
| `peer-conn-msg-writer.go` | `7fabd3760a33941d8bcf9df3d998437d0fa90b3801f2a2117700dd5e824e84b5` |
| `peer-conn-msg-writer_test.go` | `6e6dc5ab0ea96f44186ef41bee185a482bb72ef0d67a972db6e452a19eaa504b` |

Run the two regressions from the KPanel module root, using KPanel's dependency
graph, through `scripts/verify-source-lane.sh torrent`. Do not independently
update this nested module or reformat unchanged upstream files.

This backport does not assert that `v1.61.0` is the latest release or free of
vulnerabilities. Dependency freshness, source and vulnerability checks remain
required. Local replacements omit the upstream version from database matching;
the actual-source scans are supplemented by conservative original-version
govulncheck and Trivy scans through `scripts/torrent-backport-baseline.mjs`.
Those scans preserve KPanel's dependency versions and do not replace the actual
patched-source or image scans. Release provenance must identify the source as
modified and bind this manifest to the released candidate and image digest.
Replace this backport with a reviewed upstream release containing both
fixes when its wider changes and compatibility have been verified. To roll back,
revert the KPanel backport commit as a unit.
