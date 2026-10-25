# Bundled IP country data

IP Geolocation by DB-IP (https://db-ip.com).

This directory contains the unmodified DB-IP IP to Country Lite database,
October 2026 release, published by DB-IP / Eris Networks S.A.S.
It is licensed separately from KPanel under the Creative Commons Attribution 4.0
International license: https://creativecommons.org/licenses/by/4.0/.
The data is supplied without warranty. Lite data has reduced coverage and accuracy.

- Source and attribution terms: https://db-ip.com/db/download/ip-to-country-lite
- Download: https://download.db-ip.com/free/dbip-country-lite-2026-10.mmdb.gz
- Retrieved: 2026-10-25
- Uncompressed size: 8,348,509 bytes
- Upstream published MD5 (uncompressed): `b8647f712bcabbe01f8afe3ff9e7569e`
- SHA-256 (uncompressed): `dbd70ccfa2a13627eaf4913d19920a1a229f1b3c5439ac01c509995b6364202a`

KPanel embeds this fixed snapshot for offline lookup; no visitor IP is sent to DB-IP.
To refresh the bundled data, obtain a new monthly MMDB from the official source,
verify its published digest, update this notice, the embedded filename and the
checksum regression, then run the site-traffic tests and the change verification gate.
The monitoring page retains a visible DB-IP attribution link. A local compatible
`/home/web/config/GeoLite2-Country.mmdb` takes priority over the bundled data.
