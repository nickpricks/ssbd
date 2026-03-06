package core

// NoOpChecker always returns false. Used for offline mode or testing.
type NoOpChecker struct{}

func (n *NoOpChecker) IsBreached(_ string) (bool, error) {
	return false, nil
}
