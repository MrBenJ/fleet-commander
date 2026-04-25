package monitor_test

import (
	"os"
	"time"
)

func writeRawState(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
