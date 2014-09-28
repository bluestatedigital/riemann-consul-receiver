package main

import (
	"time"

    "github.com/stretchr/testify/mock"
    
    "github.com/armon/consul-api"
)

type MockAgent struct {
    mock.Mock
}

func (m *MockAgent) Self() (map[string]map[string]interface{}, error) {
    ret := m.Called()

    r0 := ret.Get(0).(map[string]map[string]interface{})
    r1 := ret.Error(1)

    return r0, r1
}

func (m *MockAgent) ServiceRegister(service *consulapi.AgentServiceRegistration) error {
    ret := m.Called(service)

    r0 := ret.Error(0)

    return r0
}

func (m *MockAgent) PassTTL(checkID, note string) error {
    ret := m.Called(checkID, note)

    r0 := ret.Error(0)

    return r0
}

type MockSession struct {
    mock.Mock
}

func (m *MockSession) List(q *consulapi.QueryOptions) ([]*consulapi.SessionEntry, *consulapi.QueryMeta, error) {
    ret := m.Called(q)

    r0 := ret.Get(0).([]*consulapi.SessionEntry)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}
func (m *MockSession) Create(se *consulapi.SessionEntry, q *consulapi.WriteOptions) (string, *consulapi.WriteMeta, error) {
    ret := m.Called(se, q)

    r0 := ret.Get(0).(string)
    r1 := ret.Get(1).(*consulapi.WriteMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}
func (m *MockSession) Info(id string, q *consulapi.QueryOptions) (*consulapi.SessionEntry, *consulapi.QueryMeta, error) {
    ret := m.Called(id, q)
    
    var retSess *consulapi.SessionEntry = nil
    
    if ret.Get(0) != nil {
        retSess = ret.Get(0).(*consulapi.SessionEntry)
    }
    
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return retSess, r1, r2
}
func (m *MockSession) Destroy(id string, q *consulapi.WriteOptions) (*consulapi.WriteMeta, error) {
    ret := m.Called(id, q)

    r0 := ret.Get(0).(*consulapi.WriteMeta)
    r1 := ret.Error(1)

    return r0, r1
}

type MockKV struct {
    mock.Mock
}

func (m *MockKV) Acquire(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error) {
    ret := m.Called(p, q)

    r0 := ret.Get(0).(bool)
    r1 := ret.Get(1).(*consulapi.WriteMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

func (m *MockKV) Get(key string, q *consulapi.QueryOptions) (*consulapi.KVPair, *consulapi.QueryMeta, error) {
    ret := m.Called(key, q)

    r0 := ret.Get(0).(*consulapi.KVPair)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

func (m *MockKV) Release(p *consulapi.KVPair, q *consulapi.WriteOptions) (bool, *consulapi.WriteMeta, error) {
    ret := m.Called(p, q)

    r0 := ret.Get(0).(bool)
    r1 := ret.Get(1).(*consulapi.WriteMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

type MockHealth struct {
    mock.Mock
}

func (m *MockHealth) State(state string, q *consulapi.QueryOptions) ([]*consulapi.HealthCheck, *consulapi.QueryMeta, error) {
    ret := m.Called(state, q)

    r0 := ret.Get(0).([]*consulapi.HealthCheck)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}

var _ = Describe("ConsulReceiver", func() {
    var receiver *ConsulReceiver
    var err error
    
    var mockAgent   MockAgent
    var mockSession MockSession
    var mockKV      MockKV
    var mockHealth  MockHealth
    
    serviceName := "some-service"
    keyName     := "some/key"
    nodeName    := "some-node"
    sessionID   := "42"

    updateInterval := time.Minute + (time.Second * 42)
    lockDelay := time.Second * 15
    
    BeforeEach(func() {
        mockAgent = MockAgent{}
        
        mockAgent.On("Self").Return(
            map[string]map[string]interface{}{
                "Config": map[string]interface{}{
                    "NodeName": nodeName,
                },
            },
            nil,
        )
        
        receiver, err = NewConsulReceiver(
            &mockAgent,
            &mockSession,
            &mockKV,
            &mockHealth,

            updateInterval,
            lockDelay,

            serviceName,
            keyName,
        )
        
        mockAgent.AssertExpectations(GinkgoT())

        Expect(receiver).NotTo(BeNil())
        Expect(err).To(BeNil())
        
        // replace with a fresh instance
        mockAgent = MockAgent{}
        
        // not used in setup
        mockSession = MockSession{}
        mockKV      = MockKV{}
        mockHealth  = MockHealth{}
    })
    
    registersService := func() {
        // expect the call to ServiceRegister
        mockAgent.On(
            "ServiceRegister",
            mock.AnythingOfType("*consulapi.AgentServiceRegistration"),
        ).Return(nil)
        
        // invoke the object under test
        receiver.RegisterService()
        
        mockAgent.AssertExpectations(GinkgoT())
        
        // retrieve the call and its arguments
        svcRegCall := mockAgent.Calls[0]
        Expect(svcRegCall.Method).To(Equal("ServiceRegister"))
        
        svcReg := svcRegCall.Arguments.Get(0).(*consulapi.AgentServiceRegistration)
        
        // verify the service registration
        Expect(svcReg.ID).To(Equal(serviceName))
        Expect(svcReg.Name).To(Equal(serviceName))
        Expect(svcReg.Check.TTL).To(Equal("306s")) // 3 times the update interval
    }
    
    It("registers the service", registersService)
    
    passesHealthCheck := func() {
        mockAgent.On("PassTTL", "service:" + serviceName, "").Return(nil)
        
        receiver.UpdateHealthCheck()
        
        mockAgent.AssertExpectations(GinkgoT())
    }
    
    It("passes a health check", passesHealthCheck)
    
    initsNewSession := func() {
        var nilQueryMeta *consulapi.QueryMeta = nil
        
        // searching for an existing session
        mockSession.On(
            "List",
            mock.AnythingOfType("*consulapi.QueryOptions"),
        ).Return([]*consulapi.SessionEntry{}, nilQueryMeta, nil)
        
        // health check must be passing before creating a session tied to that
        // health check
        mockAgent.On("PassTTL", "service:" + serviceName, "").Return(nil)
        
        // create the session
        mockSession.On(
            "Create",
            mock.AnythingOfType("*consulapi.SessionEntry"),
            mock.AnythingOfType("*consulapi.WriteOptions"),
        ).Return(sessionID, &consulapi.WriteMeta{}, nil)
        
        // set it off
        newSessionId, err := receiver.InitSession()
        
        mockAgent.AssertExpectations(GinkgoT())
        
        Expect(newSessionId).To(Equal(sessionID))
        Expect(err).To(BeNil())
        
        // verify call to Session.Create()
        sessCreateCall := mockSession.Calls[1]
        Expect(sessCreateCall.Method).To(Equal("Create"))
        
        sess := sessCreateCall.Arguments.Get(0).(*consulapi.SessionEntry)
        
        // verify the session create request
        Expect(sess.Name).To(Equal(serviceName))
        Expect(sess.LockDelay).To(Equal(lockDelay))
        Expect(len(sess.Checks)).To(Equal(2))
        Expect(sess.Checks).To(ContainElement("serfHealth"))
        Expect(sess.Checks).To(ContainElement("service:" + serviceName))
    }
    
    It("initializes a new session", initsNewSession)
    
    It("finds an existing session", func() {
        sessionID := "42"
        
        // searching for an existing session
        mockSession.On(
            "List",
            mock.AnythingOfType("*consulapi.QueryOptions"),
        ).Return(
            []*consulapi.SessionEntry{
                &consulapi.SessionEntry{
                    Node: "some-other-node",
                    Name: "some-other-name",
                },
                &consulapi.SessionEntry{
                    Node: nodeName,
                    Name: "some-other-name",
                },
                &consulapi.SessionEntry{
                    Node: "some-other-node",
                    Name: serviceName,
                },
                &consulapi.SessionEntry{ // this is the one!
                    Node: nodeName,
                    Name: serviceName,
                    ID:   sessionID,
                },
            },
            new(consulapi.QueryMeta),
            nil,
        )
        
        // set it off
        existingSessionId, err := receiver.InitSession()
        
        mockAgent.AssertExpectations(GinkgoT())
        
        Expect(existingSessionId).To(Equal(sessionID))
        Expect(err).To(BeNil())
    })

    Describe("lock acquisition", func() {
        validSession := &consulapi.SessionEntry{}
        genericQueryOpts := mock.AnythingOfType("*consulapi.QueryOptions")

        BeforeEach(func() {
            initsNewSession()
        })
        
        // if the session is invalid, return error
        // if the key's already locked by this session, return true.
        // if the key's locked by someone else, return false.
        // if the key's not locked, try to acquire it and return result
        It("aborts if the session is invalid", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                nil,
                new(consulapi.QueryMeta),
                nil,
            )
            
            _, err := receiver.AcquireLock()
            
            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            Expect(err).NotTo(BeNil())
        })
        
        It("is already locked by us", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                validSession,
                new(consulapi.QueryMeta),
                nil,
            )

            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: sessionID,
                },
                new(consulapi.QueryMeta),
                nil,
            )
            
            success, err := receiver.AcquireLock()
            
            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            
            Expect(success).To(Equal(true))
            Expect(err).To(BeNil())
        })
        
        It("is locked by someone else", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                validSession,
                new(consulapi.QueryMeta),
                nil,
            )

            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "some-other-session",
                },
                new(consulapi.QueryMeta),
                nil,
            )
            
            success, err := receiver.AcquireLock()
            
            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            
            Expect(success).To(Equal(false))
            Expect(err).To(BeNil())
        })

        It("is able to be successfully locked", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                validSession,
                new(consulapi.QueryMeta),
                nil,
            )

            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "",
                },
                new(consulapi.QueryMeta),
                nil,
            )
            
            mockKV.On(
                "Acquire",
                mock.AnythingOfType("*consulapi.KVPair"),
                mock.AnythingOfType("*consulapi.WriteOptions"),
            ).Return(true, new(consulapi.WriteMeta), nil)
            
            success, err := receiver.AcquireLock()
            
            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            
            // verify call to KV.Acquire()
            kvAcquire := mockKV.Calls[1]
            Expect(kvAcquire.Method).To(Equal("Acquire"))
            
            kvp := kvAcquire.Arguments.Get(0).(*consulapi.KVPair)
            Expect(kvp.Key).To(Equal(keyName))
            Expect(kvp.Session).To(Equal(sessionID))
            
            Expect(success).To(Equal(true))
            Expect(err).To(BeNil())
        })

        It("is not able to be successfully locked", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                validSession,
                new(consulapi.QueryMeta),
                nil,
            )

            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "",
                },
                new(consulapi.QueryMeta),
                nil,
            )
            
            mockKV.On(
                "Acquire",
                mock.AnythingOfType("*consulapi.KVPair"),
                mock.AnythingOfType("*consulapi.WriteOptions"),
            ).Return(false, new(consulapi.WriteMeta), nil)
            
            success, err := receiver.AcquireLock()
            
            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            
            // verify call to KV.Acquire()
            kvAcquire := mockKV.Calls[1]
            Expect(kvAcquire.Method).To(Equal("Acquire"))
            
            kvp := kvAcquire.Arguments.Get(0).(*consulapi.KVPair)
            Expect(kvp.Key).To(Equal(keyName))
            Expect(kvp.Session).To(Equal(sessionID))
            
            Expect(success).To(Equal(false))
            Expect(err).To(BeNil())
        })
        
        It("subsequent acquires are blocking queries", func() {
            mockSession.On("Info", sessionID, genericQueryOpts).Return(
                validSession,
                new(consulapi.QueryMeta),
                nil,
            )

            // initial Acquire(): locked by someone else
            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "some-other-session",
                },
                &consulapi.QueryMeta{
                    LastIndex: 10,
                },
                nil,
            ).Once()
            
            // next Acquire(): still locked, but uses previous LastIndex to it blocks
            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "some-other-session",
                },
                new(consulapi.QueryMeta),
                nil,
            ).Once()

            success, err := receiver.AcquireLock()
            success, err = receiver.AcquireLock()
            Expect(success).To(Equal(false))
            Expect(err).To(BeNil())

            mockSession.AssertExpectations(GinkgoT())
            mockKV.AssertExpectations(GinkgoT())
            
            // verify calls to KV.Get()
            var kvGet mock.Call
            var queryOpts *consulapi.QueryOptions
            
            // first call
            kvGet = mockKV.Calls[0]
            Expect(kvGet.Method).To(Equal("Get"))
            
            queryOpts = kvGet.Arguments.Get(1).(*consulapi.QueryOptions)
            Expect(queryOpts).NotTo(BeNil())
            Expect(queryOpts.WaitIndex).To(Equal(uint64(0)))
            Expect(queryOpts.WaitTime).To(Equal(updateInterval))

            // second call
            kvGet = mockKV.Calls[1]
            Expect(kvGet.Method).To(Equal("Get"))
            
            queryOpts = kvGet.Arguments.Get(1).(*consulapi.QueryOptions)
            Expect(queryOpts).NotTo(BeNil())
            Expect(queryOpts.WaitIndex).To(Equal(uint64(10)))
            Expect(queryOpts.WaitTime).To(Equal(updateInterval))
        })
    })

    Describe("lock watching", func() {
        BeforeEach(func() {
            initsNewSession()
        })
        
        It("returns immediately if we don't have the lock", func(done Done) {
            resultKvp := consulapi.KVPair{
                Key: keyName,
                Session: "",
            }
            
            mockKV.On(
                "Get",
                keyName,
                mock.AnythingOfType("*consulapi.QueryOptions"),
            ).Return(
                &resultKvp,
                new(consulapi.QueryMeta),
                nil,
            )

            // channel used to notify when lock has been lost; it'll just get
            // closed
            c := make(chan interface{}, 0)
            
            go receiver.WatchLock(c)
            
            // wait for the lock to be lost
            select {
                case _, more := <-c:
                    Expect(more).To(Equal(false))
            }
            
            mockKV.AssertExpectations(GinkgoT())

            // test's done *bing!*
            close(done)
        })
        
        It("polls the key until the lock is gone", func(done Done) {
            genericQueryOpts := mock.AnythingOfType("*consulapi.QueryOptions")
            
            // first call; we have the lock
            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: sessionID,
                },
                &consulapi.QueryMeta{
                    LastIndex: 10,
                },
                nil,
            ).Once()
            
            // 2nd call; lost lock
            mockKV.On("Get", keyName, genericQueryOpts).Return(
                &consulapi.KVPair{
                    Key: keyName,
                    Session: "",
                },
                new(consulapi.QueryMeta),
                nil,
            ).Once()

            // channel used to notify when lock has been lost; it'll just get
            // closed
            c := make(chan interface{}, 0)
            
            go receiver.WatchLock(c)
            
            // wait for the lock to be lost
            select {
                case _, more := <-c:
                    Expect(more).To(Equal(false))
            }
            
            mockKV.AssertExpectations(GinkgoT())

            // verify calls to KV.Get()
            var kvGet mock.Call
            var queryOpts *consulapi.QueryOptions
            
            // first call
            kvGet = mockKV.Calls[0]
            Expect(kvGet.Method).To(Equal("Get"))
            
            queryOpts = kvGet.Arguments.Get(1).(*consulapi.QueryOptions)
            Expect(queryOpts).NotTo(BeNil())
            Expect(queryOpts.WaitIndex).To(Equal(uint64(0)))
            Expect(queryOpts.WaitTime).To(BeNumerically(">", 0))

            // second call
            kvGet = mockKV.Calls[1]
            Expect(kvGet.Method).To(Equal("Get"))
            
            queryOpts = kvGet.Arguments.Get(1).(*consulapi.QueryOptions)
            Expect(queryOpts).NotTo(BeNil())
            Expect(queryOpts.WaitIndex).To(Equal(uint64(10)))
            Expect(queryOpts.WaitTime).To(BeNumerically(">", 0))

            // test's done *bing!*
            close(done)
        })
    })

    Describe("health watching", func() {
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
            go receiver.WatchHealthResults(c)
            
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
})
