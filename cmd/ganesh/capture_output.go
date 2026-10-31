package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

const captureOutputTimeoutExitCode = 124

// emitCaptureReceipt is for the short-lived CLI only. Inherited stdout is
// not necessarily pollable, so SetWriteDeadline cannot reliably bound it.
// Do not change shared descriptor flags or rely on cancelling a blocked Write:
// stop the CLI if receipt output stalls beyond its original process budget.
// The record is already durable; missing/partial output remains an ambiguous
// outcome and exact retry recovers the same event.
func emitCaptureReceipt(ctx context.Context, output io.Writer, value any) error {
	deadline, ok := ctx.Deadline()
	if !ok || ctx.Err() != nil || !time.Now().Before(deadline) {
		return errors.New("capture receipt output deadline exceeded")
	}
	// Retain the original context's monotonic deadline rather than deriving a
	// new wall-clock budget at output time. Earlier cancellation also applies.
	stop := context.AfterFunc(ctx, func() {
		// Avoid a potentially blocking stderr write on this timeout path.
		os.Exit(captureOutputTimeoutExitCode)
	})
	defer stop()
	if err := emit(output, value); err != nil {
		return errors.New("capture receipt output failed")
	}
	if ctx.Err() != nil || !time.Now().Before(deadline) {
		return errors.New("capture receipt output deadline exceeded")
	}
	return nil
}
