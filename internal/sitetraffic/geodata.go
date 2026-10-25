package sitetraffic

import (
	_ "embed"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

// The pinned offline database is redistributed under CC BY 4.0; see geodata/NOTICE.md.
//
//go:embed geodata/dbip-country-lite-2026-10.mmdb
var builtinCountryDatabase []byte

func (r *Reader) loadBuiltinGeo() {
	if r.db != nil && r.dbInfo == nil {
		r.geoStatus = "ready"
		return
	}
	r.dropGeo()
	db, err := maxminddb.OpenBytes(builtinCountryDatabase)
	if err != nil {
		r.geoStatus = "invalid"
		return
	}
	r.db = db
	r.geoStatus = "ready"
}
