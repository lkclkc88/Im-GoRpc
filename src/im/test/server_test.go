package test

import (
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	StartLog("/lkclkc88/git/Im-GoRpc/logConfig.json")
	startServer()

	for {
		time.Sleep(24 * time.Hour)
	}
}
