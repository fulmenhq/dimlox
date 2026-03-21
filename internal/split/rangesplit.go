package split

import "fmt"

func RangeModeNotImplemented() error {
	return fmt.Errorf("split mode %q is not implemented yet in this Phase 4 slice", ModeRange)
}
