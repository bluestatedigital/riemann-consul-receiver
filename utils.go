package main

import (
    log "github.com/Sirupsen/logrus"
    "runtime"
)

func checkError(msg string, err error) {
    if err != nil {
        log.Fatal(msg + ": ", err)
    }
}

// (attempt to) capture stack trace on a panic
func recoverAndLog(ident string) {
    if err := recover(); err != nil {
        log.Errorf("%s panic: %v", ident, err)
        
        trace := make([]byte, 10240)
        stackSize := runtime.Stack(trace, true)
        
        log.Error(string(trace[:stackSize]))
        
        if stackSize == cap(trace) {
            log.Errorf("stack trace likely truncated; length: %d", stackSize)
        }
    }
}
