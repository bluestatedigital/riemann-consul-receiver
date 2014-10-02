package main

import (
    "time"
    
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
    "github.com/bluestatedigital/riemann-consul-receiver/consul-mocks"
)

var _ = Describe("health checker", func() {
    var mockHealth    consulmocks.MockHealth
    var healthChecker *HealthChecker

    serviceName := "some-service"
    nodeName    := "some-node"

    updateInterval := time.Minute + (time.Second * 42)

    BeforeEach(func() {
        healthChecker = NewHealthChecker(
            &mockHealth,
            updateInterval,
        )

        mockHealth  = consulmocks.MockHealth{}
    })

    It("polls and stops when told", func(done Done) {
        genericQueryOpts := mock.AnythingOfType("*consulapi.QueryOptions")
        
        mockHealth.On("State", "any", genericQueryOpts).Return(
            []*consulapi.HealthCheck{
                &consulapi.HealthCheck{
                    Node:        nodeName,
                    CheckID:     "service:" + serviceName,
                    Name:        serviceName,
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "",
                    ServiceID:   serviceName,
                    ServiceName: serviceName,
                },
            },
            &consulapi.QueryMeta{
                LastIndex: 10,
            },
            nil,
        ).Twice()

        // channel for receiving results
        c := make(chan []consulapi.HealthCheck)
        
        // start polling
        go healthChecker.WatchHealthResults(c)
        
        // read first set of results.  sender blocks until written, we block
        // until read.
        results, more := <-c
        Expect(len(results)).To(Equal(1))
        Expect(more).To(Equal(true))
        
        // now close the channel
        c <- nil
        
        // read from the channel again; should be closed
        _, more = <-c
        Expect(more).To(Equal(false))
        
        mockHealth.AssertExpectations(GinkgoT())

        // test's done *bing!*
        close(done)
    })
})
