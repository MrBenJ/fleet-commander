package terminal

import "time"

// sleepShort is the short bounded wait used by timeout helpers in tests.
// Extracted so the helper file in shutdown_test.go can stay focused on the
// shutdown assertions.
func sleepShort() { time.Sleep(200 * time.Millisecond) }
