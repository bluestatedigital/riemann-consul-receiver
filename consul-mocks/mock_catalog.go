package consulmocks

import (
    "github.com/stretchr/testify/mock"
    "github.com/armon/consul-api"
)

type MockCatalog struct {
    mock.Mock
}

func (m *MockCatalog) Service(service, tag string, q *consulapi.QueryOptions) ([]*consulapi.CatalogService, *consulapi.QueryMeta, error) {
    ret := m.Called(service, tag, q)

    var svcs []*consulapi.CatalogService = nil
    var qm *consulapi.QueryMeta = nil
    
    if ret.Get(0) != nil {
        svcs = ret.Get(0).([]*consulapi.CatalogService)
    }
    
    if ret.Get(1) != nil {
        qm = ret.Get(1).(*consulapi.QueryMeta)
    }

    r2 := ret.Error(2)

    return svcs, qm, r2
}
