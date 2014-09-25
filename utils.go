package main

import log "github.com/Sirupsen/logrus"

func checkError(msg string, err error) {
    if err != nil {
        log.Fatal(msg + ": ", err)
    }
}
