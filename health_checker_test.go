package main

import (
    "time"
    "errors"
    
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
    "github.com/bluestatedigital/riemann-consul-receiver/consul-mocks"
)

var _ = Describe("health checker", func() {
    var mockHealth    consulmocks.MockHealth
    var mockCatalog   consulmocks.MockCatalog
    var healthChecker *HealthChecker

    serviceName := "some-service"
    nodeName    := "some-node"

    updateInterval := time.Minute + (time.Second * 42)

    genericQueryOpts := mock.AnythingOfType("*consulapi.QueryOptions")
    
    BeforeEach(func() {
        healthChecker = NewHealthChecker(
            &mockHealth,
            &mockCatalog,
            updateInterval,
        )

        mockHealth = consulmocks.MockHealth{}
        mockCatalog = consulmocks.MockCatalog{}
    })

    It("polls and stops when told", func(done Done) {
        mockCatalog.On("Service", serviceName, "", genericQueryOpts).Return(
            []*consulapi.CatalogService{
                &consulapi.CatalogService{
                    Node:        nodeName,
                    Address:     "127.0.0.2",
                    ServiceID:   serviceName,
                    ServiceName: serviceName,
                    ServiceTags: []string{ "tag1", "tag2" },
                    ServicePort: 0,
                },
            },
            new(consulapi.QueryMeta),
            nil,
        )
        
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
        c := make(chan []HealthCheck)
        
        // channel for terminating processing
        d := make(chan interface{})
        
        // start polling
        go healthChecker.WatchHealthResults(c, d)
        
        // read first set of results.  sender blocks until written, we block
        // until read.
        results, more := <-c
        Expect(len(results)).To(Equal(1))
        Expect(more).To(Equal(true))
        
        // now tell it to stop
        d <- nil
        
        // read from the channel again; should be closed
        _, more = <-c
        Expect(more).To(Equal(false))
        
        mockHealth.AssertExpectations(GinkgoT())
        mockCatalog.AssertExpectations(GinkgoT())

        // test's done *bing!*
        close(done)
    })

    It("provides service tags", func(done Done) {
        // Catalog().Service() should only be done once per service, per
        // Health().State() result.
        mockCatalog.On("Service", serviceName, "", genericQueryOpts).Return(
            []*consulapi.CatalogService{
                &consulapi.CatalogService{
                    Node:        nodeName,
                    Address:     "127.0.0.2",
                    ServiceID:   serviceName + "0",
                    ServiceName: serviceName,
                    ServiceTags: []string{ "tag1", "tag2" },
                    ServicePort: 0,
                },
                &consulapi.CatalogService{
                    Node:        "other-node-name",
                    Address:     "127.0.0.3",
                    ServiceID:   serviceName + "99",
                    ServiceName: serviceName,
                    ServiceTags: []string{ "tag3", "tag4" },
                    ServicePort: 0,
                },
            },
            new(consulapi.QueryMeta),
            nil,
        ).Twice()
        
        mockHealth.On("State", "any", genericQueryOpts).Return(
            []*consulapi.HealthCheck{
                &consulapi.HealthCheck{
                    Node:        nodeName,
                    CheckID:     "service:" + serviceName,
                    Name:        "Health check for '" + serviceName + "' service",
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "some check result output",
                    ServiceID:   serviceName + "0",
                    ServiceName: serviceName,
                },
                &consulapi.HealthCheck{
                    Node:        "other-node-name",
                    CheckID:     "service:" + serviceName,
                    Name:        "Health check for '" + serviceName + "' service",
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "some check result output",
                    ServiceID:   serviceName + "99",
                    ServiceName: serviceName,
                },
            },
            &consulapi.QueryMeta{
                LastIndex: 10,
            },
            nil,
        ).Twice()

        // channel for receiving results
        c := make(chan []HealthCheck)
        
        // channel for terminating processing
        d := make(chan interface{})
        
        // start polling
        go healthChecker.WatchHealthResults(c, d)
        
        // read first set of results.  sender blocks until written, we block
        // until read.
        results, more := <-c
        Expect(more).To(Equal(true))
        Expect(len(results)).To(Equal(2))
        Expect(results[0].Node).To(Equal(nodeName))
        Expect(results[0].CheckID).To(Equal("service:" + serviceName))
        Expect(results[0].Status).To(Equal("passing"))
        Expect(results[0].Notes).To(Equal("'s good, yo"))
        Expect(results[0].Output).To(Equal("some check result output"))
        Expect(results[0].ServiceID).To(Equal(serviceName + "0"))
        Expect(results[0].ServiceName).To(Equal(serviceName))
        Expect(results[0].Tags).To(ContainElement("tag1"))
        Expect(results[0].Tags).To(ContainElement("tag2"))
        
        // now close the channel
        d <- nil
        
        // read from the channel again; should be closed
        _, more = <-c
        Expect(more).To(Equal(false))
        
        mockHealth.AssertExpectations(GinkgoT())
        mockCatalog.AssertExpectations(GinkgoT())

        // test's done *bing!*
        close(done)
    })

    It("does not bomb if no details are found for a node and service", func(done Done) {
        mockHealth.On("State", "any", genericQueryOpts).Return(
            []*consulapi.HealthCheck{
                &consulapi.HealthCheck{
                    Node:        nodeName,
                    CheckID:     "service:" + serviceName,
                    Name:        "Health check for '" + serviceName + "' service",
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "some check result output",
                    ServiceID:   serviceName + "0",
                    ServiceName: serviceName,
                },
                &consulapi.HealthCheck{
                    Node:        "yet-another-node-name",
                    CheckID:     "service:" + serviceName,
                    Name:        "Health check for '" + serviceName + "' service",
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "some check result output",
                    ServiceID:   serviceName + "99",
                    ServiceName: serviceName,
                },
            },
            &consulapi.QueryMeta{
                LastIndex: 10,
            },
            nil,
        ).Twice()

        // Catalog().Service() should only be done once per service, per
        // Health().State() result.
        mockCatalog.On("Service", serviceName, "", genericQueryOpts).Return(
            []*consulapi.CatalogService{
                &consulapi.CatalogService{
                    Node:        nodeName,
                    Address:     "127.0.0.2",
                    ServiceID:   serviceName + "0",
                    ServiceName: serviceName,
                    ServiceTags: []string{ "tag1", "tag2" },
                    ServicePort: 0,
                },
                &consulapi.CatalogService{
                    Node:        "other-node-name",
                    Address:     "127.0.0.3",
                    ServiceID:   serviceName + "99",
                    ServiceName: serviceName,
                    ServiceTags: []string{ "tag3", "tag4" },
                    ServicePort: 0,
                },
            },
            new(consulapi.QueryMeta),
            nil,
        ).Twice()
        
        // channel for receiving results
        c := make(chan []HealthCheck)
        
        // channel for terminating processing
        d := make(chan interface{})
        
        // start polling
        go healthChecker.WatchHealthResults(c, d)
        
        // read first set of results.  sender blocks until written, we block
        // until read.
        results, more := <-c
        Expect(more).To(Equal(true))
        Expect(len(results)).To(Equal(2))
        Expect(len(results[1].Tags)).To(Equal(0))
        
        // now close the channel
        d <- nil
        
        // read from the channel again; should be closed
        _, more = <-c
        Expect(more).To(Equal(false))
        
        mockHealth.AssertExpectations(GinkgoT())
        mockCatalog.AssertExpectations(GinkgoT())

        // test's done *bing!*
        close(done)
    })

    It("stops polling if an error happens retrieving services", func(done Done) {
        mockHealth.On("State", "any", genericQueryOpts).Return(
            []*consulapi.HealthCheck{
                &consulapi.HealthCheck{
                    Node:        nodeName,
                    CheckID:     "service:" + serviceName,
                    Name:        "Health check for '" + serviceName + "' service",
                    Status:      "passing",
                    Notes:       "'s good, yo",
                    Output:      "some check result output",
                    ServiceID:   serviceName + "0",
                    ServiceName: serviceName,
                },
            },
            &consulapi.QueryMeta{
                LastIndex: 10,
            },
            nil,
        ).Once()

        mockCatalog.On("Service", serviceName, "", genericQueryOpts).Return(
            nil,
            nil,
            errors.New("some error"),
        ).Once()

        // channel for receiving results
        c := make(chan []HealthCheck)
        
        // channel for terminating processing
        d := make(chan interface{})
        
        // start polling
        go healthChecker.WatchHealthResults(c, d)
        
        // read first set of results.  sender blocks until written, we block
        // until read.
        _, more := <-c
        Expect(more).To(Equal(false))
        
        mockHealth.AssertExpectations(GinkgoT())
        mockCatalog.AssertExpectations(GinkgoT())

        // test's done *bing!*
        close(done)

    })
})
