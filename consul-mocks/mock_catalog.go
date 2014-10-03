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

    r0 := ret.Get(0).([]*consulapi.CatalogService)
    r1 := ret.Get(1).(*consulapi.QueryMeta)
    r2 := ret.Error(2)

    return r0, r1, r2
}
