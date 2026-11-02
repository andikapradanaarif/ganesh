package capturecli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/adapter/crush"
	"github.com/andikapradanaarif/ganesh/internal/capture"
)

func TestInputWorkerPanicIsContentFree(t *testing.T) {
	ready := make(chan metadataResult, 1)
	go func() { ready <- reduceCaptureInput(context.Background(), panicReader{}, crush.ReduceStream) }()
	select {
	case result := <-ready:
		if result.err == nil || strings.Contains(result.err.Error(), "SYNTHETIC_SECRET") || result.metadata != (capture.Metadata{}) {
			t.Fatal("worker panic escaped the static rejection boundary")
		}
	case <-time.After(time.Second):
		t.Fatal("panicking input worker did not return")
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("ghp_SYNTHETIC_SECRET") }
