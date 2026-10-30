//go:build darwin && cgo

// Package processclock supplies an OS-observed process creation time. Starting
// the capture clock in main would omit Go runtime and loader startup.
package processclock

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <unistd.h>

static int ganesh_process_start(long long *seconds, long long *micros) {
    struct proc_bsdinfo info;
    int count = proc_pidinfo(getpid(), PROC_PIDTBSDINFO, 0, &info, sizeof(info));
    if (count != sizeof(info)) return -1;
    *seconds = info.pbi_start_tvsec;
    *micros = info.pbi_start_tvusec;
    return 0;
}
*/
import "C"

import (
	"errors"
	"time"
)

func Start() (time.Time, error) {
	var seconds, micros C.longlong
	if C.ganesh_process_start(&seconds, &micros) != 0 {
		return time.Time{}, errors.New("OS process start time unavailable")
	}
	return time.Unix(int64(seconds), int64(micros)*1000), nil
}
