// Command mf is the Memory Flow command-line client. It resolves which Memory
// Flow instance to talk to (remote home server, or the local standalone app)
// and exposes the API as stable verbs:
//
//	mf issue show ORT-100
//	mf issue attach-git ORT-100 5253083
//	mf issue done ORT-100
//
// Run `mf help` for the full command list.
package main

import (
	"os"

	"github.com/warriorguo/memory_flow/backend/internal/mfcli"
)

func main() {
	os.Exit(mfcli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
