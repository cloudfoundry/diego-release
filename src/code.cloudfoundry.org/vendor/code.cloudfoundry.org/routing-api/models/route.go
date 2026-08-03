package models

import (
	"time"

	uuid "github.com/nu7hatch/gouuid"
)

type Route struct {
	Model
	ExpiresAt time.Time `json:"-"`
	RouteEntity
}

type RouteEntity struct {
	Route           string `gorm:"not null; unique_index:idx_route" json:"route"`
	Port            uint16 `gorm:"not null; unique_index:idx_route; size:32" json:"port"`
	IP              string `gorm:"not null; unique_index:idx_route" json:"ip"`
	TTL             *int   `json:"ttl"`
	LogGuid         string `json:"log_guid"`
	RouteServiceUrl string `gorm:"not null; unique_index:idx_route" json:"route_service_url,omitempty"`
	ModificationTag `json:"modification_tag"`
}

func NewRouteWithModel(route Route) (Route, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return Route{}, err
	}

	return Route{
		ExpiresAt:   time.Now().Add(time.Duration(*route.TTL) * time.Second),
		Model:       Model{Guid: guid.String()},
		RouteEntity: route.RouteEntity,
	}, nil
}
func NewRoute(url string, port uint16, ip, logGuid, routeServiceUrl string, ttl int) Route {
	route := RouteEntity{
		Route:           url,
		Port:            port,
		IP:              ip,
		TTL:             &ttl,
		LogGuid:         logGuid,
		RouteServiceUrl: routeServiceUrl,
	}
	return Route{
		RouteEntity: route,
	}
}

func NewModificationTag() (ModificationTag, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return ModificationTag{}, err
	}

	return ModificationTag{
		Guid:  uuid.String(),
		Index: 0,
	}, nil
}

func (t *ModificationTag) Increment() {
	t.Index++
}

func (m *ModificationTag) SucceededBy(other *ModificationTag) bool {
	if m == nil || m.Guid == "" || other.Guid == "" {
		return true
	}

	return m.Guid != other.Guid || m.Index < other.Index
}

func (r Route) GetTTL() int {
	if r.TTL == nil {
		return 0
	}
	return *r.TTL
}

func (r *Route) SetDefaults(defaultTTL int) {
	if r.TTL == nil {
		r.TTL = &defaultTTL
	}
}

type ModificationTag struct {
	Guid  string `gorm:"column:modification_guid" json:"guid"`
	Index uint32 `gorm:"column:modification_index" json:"index"`
}
