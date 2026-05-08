package collector

import (
	"io"
	"os"
)

// logWriter is where collector progress messages are written (stderr by default).
var logWriter io.Writer = os.Stderr
