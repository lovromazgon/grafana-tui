package grafanatui

import (
	"context"
)

// contextBackground returns a background context. This is used in
// tea.Cmd functions where no parent context is available.
func contextBackground() context.Context {
	return context.Background()
}
