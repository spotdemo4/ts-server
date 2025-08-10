//go:build !dev

package main

import "embed"

//go:embed all:client
var clientfs embed.FS

//go:embed db/migrations/*.sql
var dbfs embed.FS

func init() {
	clientFS = clientfs
	dbFS = dbfs
}
