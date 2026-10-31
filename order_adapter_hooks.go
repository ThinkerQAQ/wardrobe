package main

import (
	"database/sql"
)

// Core defines an optional, build-time source adapter seam. Public builds do
// not require any retailer-specific implementation.
var retailerSyncEnabled = false
var privateIncomingOrderKey = func(OrderItem) string { return "" }
var privateMatchOrder = func(*store, *sql.Tx, OrderItem, bool) (string, string, error) {
	return "", "", nil
}
var privateReconcileOrder = func(*store, int) (LegacyReconcileResult, error) {
	return LegacyReconcileResult{}, nil
}

type LegacyReconcileResult struct {
	Merged    int `json:"merged"`
	Conflicts int `json:"conflicts"`
	Ambiguous int `json:"ambiguous"`
	Eligible  int `json:"eligible"`
}
