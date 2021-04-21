// +build gops

package diagnostics

import (
	log "github.com/apex/log"
	"github.com/google/gops/agent"
)

func init() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err.Error())
	}
}
