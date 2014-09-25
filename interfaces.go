package main

import (
    "github.com/amir/raidman"
    "github.com/armon/consul-api"
)

type RiemannClient interface {
    Send(*raidman.Event) error
}

type ConsulClient interface {
    Agent()   *consulapi.Agent
    Session() *consulapi.Session
    KV()      *consulapi.KV
    Health()  *consulapi.Health
}
