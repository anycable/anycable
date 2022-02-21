//go:build gops
// +build gops

package diagnostics

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"

	log "github.com/apex/log"
	"github.com/google/gops/agent"
)

func init() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err.Error())
	}

	pprofRequired := false

	if vals := os.Getenv("BLOCK_PROFILE_RATE"); vals != "" {
		val, err := strconv.Atoi(vals)

		if err != nil {
			log.Fatalf("Invalid value for block profile rate: %s", vals)
		}

		runtime.SetBlockProfileRate(val)
		pprofRequired = true
		fmt.Println("[PPROF] Block profiling enabled")
	}

	if vals := os.Getenv("MUTEX_PROFILE_FRACTION"); vals != "" {
		val, err := strconv.Atoi(vals)

		if err != nil {
			log.Fatalf("Invalid value for mutex profile fraction: %s", vals)
		}

		runtime.SetMutexProfileFraction(val)
		pprofRequired = true
		fmt.Println("[PPROF] Mutex profiling enabled")
	}

	// Run pprof web server as well to be able to capture blocks and mutex profiles
	// (not supported by gops)
	if pprofRequired {
		go func() { http.ListenAndServe(":6060", nil) }()
	}
}
