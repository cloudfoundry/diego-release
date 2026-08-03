package models

import (
	"fmt"
	"time"

	uuid "github.com/nu7hatch/gouuid"
)

type TcpRouteMapping struct {
	Model
	ExpiresAt time.Time `json:"-"`
	TcpMappingEntity
}

// IMPORTANT!! when adding a new field here that is part of the unique index for
//
//	a tcp route, make sure to update not only the logic for Matches(),
//	but also the SqlDb.FindExistingTcpRouteMapping() function's custom
//	WHERE filter to include the new field
type TcpMappingEntity struct {
	RouterGroupGuid    string  `gorm:"not null; unique_index:idx_tcp_route" json:"router_group_guid"`
	HostPort           uint16  `gorm:"not null; unique_index:idx_tcp_route; type:int; size:32" json:"backend_port"`
	HostTLSPort        int     `gorm:"default:null; unique_index:idx_tcp_route; type:int" json:"backend_tls_port"`
	HostIP             string  `gorm:"not null; unique_index:idx_tcp_route" json:"backend_ip"`
	SniHostname        *string `gorm:"default:null; unique_index:idx_tcp_route" json:"backend_sni_hostname,omitempty"`
	SniRewriteHostname *string `gorm:"default:null" json:"sni_rewrite_hostname,omitempty"`
	// We don't add uniqueness on InstanceId so that if a route is attempted to be created with the same detals but
	// different InstanceId, we fail uniqueness and prevent stale/duplicate routes. If this fails a route, the
	// TTL on the old record should expire + allow the new route to be created eventually.
	InstanceId           string `gorm:"null; default:null;" json:"instance_id"`
	ExternalPort         uint16 `gorm:"not null; unique_index:idx_tcp_route; type:int; size:32" json:"port"`
	ModificationTag      `json:"modification_tag"`
	TTL                  *int   `json:"ttl,omitempty"`
	IsolationSegment     string `json:"isolation_segment"`
	TerminateFrontendTLS bool   `gorm:"default:false" json:"terminate_frontend_tls,omitempty"`
	// alpns is a csv value
	ALPNs string `json:"alpns,omitempty"`
}

func (TcpRouteMapping) TableName() string {
	return "tcp_routes"
}

func NewTcpRouteMappingWithModel(tcpMapping TcpRouteMapping) (TcpRouteMapping, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return TcpRouteMapping{}, err
	}

	m := Model{Guid: guid.String()}
	return TcpRouteMapping{
		ExpiresAt:        time.Now().Add(time.Duration(*tcpMapping.TTL) * time.Second),
		Model:            m,
		TcpMappingEntity: tcpMapping.TcpMappingEntity,
	}, nil
}

func NewTcpRouteMapping(
	routerGroupGuid string,
	externalPort uint16,
	hostIP string,
	hostPort uint16,
	hostTlsPort int,
	instanceId string,
	sniHostname *string,
	sniRewriteHostname *string,
	ttl int,
	modTag ModificationTag,
	terminateFrontendTLS bool,
	alpns string,
) TcpRouteMapping {
	mapping := TcpRouteMapping{
		TcpMappingEntity: TcpMappingEntity{
			RouterGroupGuid:      routerGroupGuid,
			ExternalPort:         externalPort,
			SniHostname:          sniHostname,
			SniRewriteHostname:   sniRewriteHostname,
			InstanceId:           instanceId,
			HostPort:             hostPort,
			HostTLSPort:          hostTlsPort,
			HostIP:               hostIP,
			TTL:                  &ttl,
			ModificationTag:      modTag,
			TerminateFrontendTLS: terminateFrontendTLS,
			ALPNs:                alpns,
		},
	}
	return mapping
}

func (m TcpRouteMapping) String() string {
	return fmt.Sprintf("%s:%d<->%s:%d", m.RouterGroupGuid, m.ExternalPort, m.HostIP, m.HostPort)
}

func (m TcpRouteMapping) Matches(other TcpRouteMapping) bool {
	sameRouterGroupGuid := m.RouterGroupGuid == other.RouterGroupGuid
	sameExternalPort := m.ExternalPort == other.ExternalPort
	sameHostIP := m.HostIP == other.HostIP
	sameHostPort := m.HostPort == other.HostPort
	sameInstanceId := m.InstanceId == other.InstanceId
	sameHostTLSPort := m.HostTLSPort == other.HostTLSPort

	nilTTL := m.TTL == nil && other.TTL == nil
	sameTTLPointer := m.TTL == other.TTL
	sameTTLValue := m.TTL != nil && other.TTL != nil && *m.TTL == *other.TTL
	sameTTL := nilTTL || sameTTLPointer || sameTTLValue

	nilSniHostname := m.SniHostname == nil && other.SniHostname == nil
	sameSniHostnamePointer := m.SniHostname == other.SniHostname
	sameSniHostnameValue := m.SniHostname != nil && other.SniHostname != nil && *m.SniHostname == *other.SniHostname
	sameSniHostname := nilSniHostname || sameSniHostnamePointer || sameSniHostnameValue

	return sameRouterGroupGuid &&
		sameExternalPort &&
		sameHostIP &&
		sameHostPort &&
		sameInstanceId &&
		sameTTL &&
		sameHostTLSPort &&
		sameSniHostname
}

func (t *TcpRouteMapping) SetDefaults(maxTTL int) {
	// default ttl if not present
	// TTL is a pointer to a uint16 so that we can
	// detect if it's present or not (i.e. nil or 0)
	if t.TTL == nil {
		t.TTL = &maxTTL
	}
}
