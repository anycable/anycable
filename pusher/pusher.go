package pusher

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"time"
)

func GenerateSocketID(uid string) string {
	// Validate socket_id format (Pusher requrest <number>.<number>) or generate new one
	if matched, _ := regexp.MatchString(`\A\d+\.\d+\z`, uid); !matched {
		import_time := time.Now().Unix()
		randomBytes := make([]byte, 4)
		rand.Read(randomBytes) // nolint:errcheck
		random_part := float64(uint32(randomBytes[0])<<24 + uint32(randomBytes[1])<<16 + uint32(randomBytes[2])<<8 + uint32(randomBytes[3]))
		random_part = random_part / (1 << 32) * 1000000
		uid = fmt.Sprintf("%d.%.0f", import_time, random_part)
	}

	return uid
}
