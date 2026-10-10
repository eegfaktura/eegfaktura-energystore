package testsupport

import (
	_ "unsafe" // go:linkname

	"at.ourproject/energystore/store/ebow"
)

// connectionPool is ebow's process-wide pool. Since upstream #66 a closed pool stays closed
// (ebow.ClosePool is one-way: later opens fail with "failed to connect to database"), but the
// tests need a fresh pool per temporary persistence path, because the pool caches stores by
// tenant/ecId, not by path. The test environment must not change production code (open point
// ES-24), so the helper swaps the variable through go:linkname — test code only; upstream could
// offer a test hook instead (open point in the workspace).
//
//go:linkname connectionPool at.ourproject/energystore/store/ebow.connectionPool
var connectionPool *ebow.Pool

// resetPool closes every store of the current pool and replaces it with a new, open one of the
// production size.
func resetPool() {
	ebow.ClosePool()
	connectionPool = ebow.NewPool(20)
}
