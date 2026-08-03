package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/eventhub"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/models"

	"gorm.io/gorm"
)

//go:generate counterfeiter -o fakes/fake_db.go . DB
type DB interface {
	ReadRoutes() ([]models.Route, error)
	SaveRoute(route models.Route) error
	DeleteRoute(route models.Route) error

	ReadTcpRouteMappings() ([]models.TcpRouteMapping, error)
	ReadFilteredTcpRouteMappings(columnName string, values []string) ([]models.TcpRouteMapping, error)
	FindSimilarTcpRouteMappings(sniHostname string, externalPort uint16) ([]models.TcpRouteMapping, error)
	SaveTcpRouteMapping(tcpMapping models.TcpRouteMapping) error
	DeleteTcpRouteMapping(tcpMapping models.TcpRouteMapping) error

	ReadRouterGroups() (models.RouterGroups, error)
	ReadRouterGroup(guid string) (models.RouterGroup, error)
	DeleteRouterGroup(guid string) error
	ReadRouterGroupByName(name string) (models.RouterGroup, error)
	SaveRouterGroup(routerGroup models.RouterGroup) error

	CancelWatches()
	WatchChanges(watchType string) (<-chan Event, <-chan error, context.CancelFunc)

	LockRouterGroupReads()
	LockRouterGroupWrites()
	UnlockRouterGroupReads()
	UnlockRouterGroupWrites()
}

const (
	TCP_MAPPING_BASE_KEY  string = "/v1/tcp_routes/router_groups"
	HTTP_ROUTE_BASE_KEY   string = "/routes"
	ROUTER_GROUP_BASE_KEY string = "/v1/router_groups"
	defaultDialTimeout           = 30 * time.Second
	maxRetries                   = 3
	TCP_WATCH             string = "tcp-watch"
	HTTP_WATCH            string = "http-watch"
	ROUTER_GROUP_WATCH    string = "router-group-watch"
)

const backupError = "database unavailable due to backup or restore"

type rwLocker struct {
	readLock  uint32
	writeLock uint32
}

func (l *rwLocker) isReadLocked() bool {
	return atomic.LoadUint32(&l.readLock) != 0
}

func (l *rwLocker) isWriteLocked() bool {
	return atomic.LoadUint32(&l.writeLock) != 0
}

func (l *rwLocker) lockReads() {
	atomic.StoreUint32(&l.readLock, 1)
}

func (l *rwLocker) lockWrites() {
	atomic.StoreUint32(&l.writeLock, 1)
}

func (l *rwLocker) unlockReads() {
	atomic.StoreUint32(&l.readLock, 0)
}

func (l *rwLocker) unlockWrites() {
	atomic.StoreUint32(&l.writeLock, 0)
}

type SqlDB struct {
	Client       Client
	tcpEventHub  eventhub.Hub
	httpEventHub eventhub.Hub
	locker       *rwLocker
}

var DeleteRouteError = DBError{Type: KeyNotFound, Message: "Delete Fails: Route does not exist"}
var DeleteRouterGroupError = DBError{Type: KeyNotFound, Message: "Delete Fails: Router Group does not exist"}

func NewSqlDB(cfg *config.SqlDB) (*SqlDB, error) {
	if cfg == nil {
		return nil, errors.New("sql configuration cannot be nil")
	}

	connStr, err := ConnectionString(cfg)
	if err != nil {
		return nil, err
	}

	var dialect gorm.Dialector
	switch cfg.Type {
	case "postgres":
		dialect = postgres.Open(connStr)
	case "mysql":
		dialect = mysql.Open(connStr)
	default:
		return &SqlDB{}, fmt.Errorf("unknown type %s", cfg.Type)
	}

	db, err := gorm.Open(dialect, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	connMaxLifetime := time.Duration(cfg.ConnMaxLifetime) * time.Second
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	tcpEventHub := eventhub.NewNonBlocking(1024)
	httpEventHub := eventhub.NewNonBlocking(1024)

	return &SqlDB{
		Client:       NewGormClient(db),
		tcpEventHub:  tcpEventHub,
		httpEventHub: httpEventHub,
		locker:       &rwLocker{},
	}, nil
}

func (s *SqlDB) FindExpiredRoutes(routes interface{}, c clock.Clock) error {
	// mysql stores time at second level precision, but lets us query with sub-second precision.
	// postgres stores at microsecond precision. we subtract a second from expiry time to give
	// us an extra second of buffer to account for rounding issues:
	// if we tell the db to save an expiry of 5.3s, and we query at 5.2s, mysql will think it expired,
	// as the db will compare 5s against 5.2s. Oops.
	return s.Client.Find(routes, "expires_at < ?", c.Now().Add(-1*time.Second))
}

func (s *SqlDB) CleanupRoutes(logger lager.Logger, pruningInterval time.Duration, signals <-chan os.Signal) {
	var tcpInFlight, httpInFlight int32
	pruningTicker := time.NewTicker(pruningInterval)
	clock := clock.NewClock()
	for {
		select {
		case <-pruningTicker.C:
			if atomic.CompareAndSwapInt32(&tcpInFlight, 0, 1) {
				go func() {
					defer atomic.StoreInt32(&tcpInFlight, 0)
					var tcpRoutes []models.TcpRouteMapping
					err := s.FindExpiredRoutes(&tcpRoutes, clock)
					if err != nil {
						logger.Error("failed-to-prune-tcp-routes", err)
						return
					}
					guids := make([]string, len(tcpRoutes))
					for _, route := range tcpRoutes {
						guids = append(guids, route.Guid)
					}
					rowsAffected, err := s.Client.Delete(models.TcpRouteMapping{}, "guid in (?)", guids)
					if err != nil {
						logger.Error("failed-to-prune-tcp-routes", err)
						return
					}
					for _, route := range tcpRoutes {
						err = s.emitEvent(ExpireEvent, route)
						if err != nil {
							logger.Error("failed-to-emit-expire-tcp-event", err)
						}
					}

					logger.Info("successfully-finished-pruning-tcp-routes", lager.Data{"rowsAffected": rowsAffected})
				}()
			}

			if atomic.CompareAndSwapInt32(&httpInFlight, 0, 1) {
				go func() {
					defer atomic.StoreInt32(&httpInFlight, 0)
					var httpRoutes []models.Route
					err := s.FindExpiredRoutes(&httpRoutes, clock)
					if err != nil {
						logger.Error("failed-to-prune-http-routes", err)
						return
					}
					guids := make([]string, len(httpRoutes))
					for _, route := range httpRoutes {
						guids = append(guids, route.Guid)
					}
					rowsAffected, err := s.Client.Delete(models.Route{}, "guid in (?)", guids)
					if err != nil {
						logger.Error("failed-to-prune-http-routes", err)
						return
					}
					for _, route := range httpRoutes {
						err = s.emitEvent(ExpireEvent, route)
						if err != nil {
							logger.Error("failed-to-emit-expire-http-event", err)
						}
					}

					logger.Info("successfully-finished-pruning-http-routes", lager.Data{"rowsAffected": rowsAffected})
				}()
			}
		case <-signals:
			return
		}
	}
}

func (s *SqlDB) ReadRouterGroups() (models.RouterGroups, error) {
	if s.locker.isReadLocked() {
		return models.RouterGroups{}, errors.New(backupError)
	}
	routerGroupsDB := models.RouterGroupsDB{}
	routerGroups := models.RouterGroups{}
	err := s.Client.Find(&routerGroupsDB)
	if err == nil {
		routerGroups = routerGroupsDB.ToRouterGroups()
	}

	return routerGroups, err
}

func (s *SqlDB) ReadRouterGroup(guid string) (models.RouterGroup, error) {
	if s.locker.isReadLocked() {
		return models.RouterGroup{}, errors.New(backupError)
	}
	routerGroupDB := models.RouterGroupDB{}
	routerGroup := models.RouterGroup{}
	err := s.Client.Where("guid = ?", guid).First(&routerGroupDB)
	if err == nil {
		routerGroup = routerGroupDB.ToRouterGroup()
	}

	if recordNotFound(err) {
		err = nil
	}
	return routerGroup, err
}

func (s *SqlDB) ReadRouterGroupByName(name string) (models.RouterGroup, error) {
	if s.locker.isReadLocked() {
		return models.RouterGroup{}, errors.New(backupError)
	}
	routerGroupDB := models.RouterGroupDB{}
	routerGroup := models.RouterGroup{}
	err := s.Client.Where("name = ?", name).First(&routerGroupDB)
	if err == nil {
		routerGroup = routerGroupDB.ToRouterGroup()
	}

	if recordNotFound(err) {
		err = nil
	}
	return routerGroup, err
}

func (s *SqlDB) SaveRouterGroup(routerGroup models.RouterGroup) error {
	if s.locker.isWriteLocked() {
		return errors.New(backupError)
	}
	existingRouterGroup, err := s.ReadRouterGroup(routerGroup.Guid)
	if err != nil {
		return err
	}

	routerGroupDB := models.NewRouterGroupDB(routerGroup)
	if existingRouterGroup.Guid == routerGroup.Guid {
		updateRouterGroup(&existingRouterGroup, &routerGroup)
		routerGroupDB = models.NewRouterGroupDB(existingRouterGroup)
		_, err = s.Client.Save(&routerGroupDB)
	} else {
		_, err = s.Client.Create(&routerGroupDB)
	}

	return err
}

func (s *SqlDB) DeleteRouterGroup(guid string) error {
	if s.locker.isWriteLocked() {
		return errors.New(backupError)
	}
	routerGroup, err := s.ReadRouterGroup(guid)
	if err != nil {
		return err
	}
	if routerGroup == (models.RouterGroup{}) {
		return DeleteRouterGroupError
	}

	// Use WHERE clause to delete the specific router group by guid
	_, err = s.Client.Where("guid = ?", guid).Delete(&models.RouterGroupDB{})
	if err != nil {
		return err
	}
	return nil
}

func (s *SqlDB) LockRouterGroupReads() {
	s.locker.lockReads()
}

func (s *SqlDB) LockRouterGroupWrites() {
	s.locker.lockWrites()
}

func (s *SqlDB) UnlockRouterGroupReads() {
	s.locker.unlockReads()
}

func (s *SqlDB) UnlockRouterGroupWrites() {
	s.locker.unlockWrites()
}

func updateRouterGroup(existingRouterGroup, currentRouterGroup *models.RouterGroup) {
	if currentRouterGroup.Type != "" {
		existingRouterGroup.Type = currentRouterGroup.Type
	}
	if currentRouterGroup.Name != "" {
		existingRouterGroup.Name = currentRouterGroup.Name
	}
	existingRouterGroup.ReservablePorts = currentRouterGroup.ReservablePorts
}

func updateTcpRouteMapping(existingTcpRouteMapping models.TcpRouteMapping, currentTcpRouteMapping models.TcpRouteMapping) models.TcpRouteMapping {
	existingTcpRouteMapping.ModificationTag.Increment()
	if currentTcpRouteMapping.TTL != nil {
		existingTcpRouteMapping.TTL = currentTcpRouteMapping.TTL
	}
	existingTcpRouteMapping.IsolationSegment = currentTcpRouteMapping.IsolationSegment
	existingTcpRouteMapping.SniRewriteHostname = currentTcpRouteMapping.SniRewriteHostname

	existingTcpRouteMapping.ExpiresAt = time.Now().
		Add(time.Duration(*existingTcpRouteMapping.TTL) * time.Second)
	return existingTcpRouteMapping
}

func updateRoute(existingRoute, currentRoute models.Route) models.Route {
	existingRoute.ModificationTag.Increment()
	if currentRoute.TTL != nil {
		existingRoute.TTL = currentRoute.TTL
	}

	if currentRoute.LogGuid != "" {
		existingRoute.LogGuid = currentRoute.LogGuid
	}

	existingRoute.ExpiresAt = time.Now().
		Add(time.Duration(*existingRoute.TTL) * time.Second)

	return existingRoute
}

func notImplementedError() error {
	pc, _, _, _ := runtime.Caller(1)
	fnName := runtime.FuncForPC(pc).Name()
	return fmt.Errorf("function not implemented: %s", fnName)
}

func (s *SqlDB) ReadRoutes() ([]models.Route, error) {
	var routes []models.Route
	now := time.Now()
	err := s.Client.Where("expires_at > ?", now).Find(&routes)
	if err != nil {
		return nil, err
	}
	return routes, err
}

func (s *SqlDB) readRoute(route models.Route) (models.Route, error) {
	var routes []models.Route
	err := s.Client.Where("route = ? and ip = ? and port = ? and route_service_url = ?",
		route.Route, route.IP, route.Port, route.RouteServiceUrl).Find(&routes)

	if err != nil {
		return route, err
	}
	count := len(routes)
	if count > 1 || count < 0 {
		return route, errors.New("have duplicate routes")
	}
	if count == 1 {
		return routes[0], nil
	}
	return models.Route{}, nil
}

func (s *SqlDB) SaveRoute(route models.Route) error {
	existingRoute, err := s.readRoute(route)
	if err != nil {
		return err
	}

	if existingRoute != (models.Route{}) {
		newRoute := updateRoute(existingRoute, route)
		_, err = s.Client.Save(&newRoute)
		if err != nil {
			return err
		}
		return s.emitEvent(UpdateEvent, newRoute)
	}

	newRoute, err := models.NewRouteWithModel(route)
	if err != nil {
		return err
	}

	tag, err := models.NewModificationTag()
	if err != nil {
		return err
	}
	newRoute.ModificationTag = tag

	_, err = s.Client.Create(&newRoute)
	if err != nil {
		return err
	}
	return s.emitEvent(CreateEvent, newRoute)
}

func (s *SqlDB) DeleteRoute(route models.Route) error {
	route, err := s.readRoute(route)
	if err != nil {
		return err
	}
	if route == (models.Route{}) {
		return DeleteRouteError
	}

	_, err = s.Client.Delete(&route)
	if err != nil {
		return err
	}
	return s.emitEvent(DeleteEvent, route)
}

func (s *SqlDB) ReadTcpRouteMappings() ([]models.TcpRouteMapping, error) {
	var tcpRoutes []models.TcpRouteMapping
	now := time.Now()
	err := s.Client.Where("expires_at > ?", now).Find(&tcpRoutes)
	if err != nil {
		return nil, err
	}
	return tcpRoutes, nil
}

func (s *SqlDB) ReadFilteredTcpRouteMappings(columnName string, values []string) ([]models.TcpRouteMapping, error) {
	var tcpRoutes []models.TcpRouteMapping
	now := time.Now()
	err := s.Client.Where(columnName+" in (?)", values).Where("expires_at > ?", now).Find(&tcpRoutes)
	if err != nil {
		return nil, err
	}
	return tcpRoutes, nil
}

func (s *SqlDB) FindSimilarTcpRouteMappings(sniHostname string, externalPort uint16) ([]models.TcpRouteMapping, error) {
	var tcpRoutes []models.TcpRouteMapping
	now := time.Now()
	err := s.Client.
		Where("sni_hostname = ? and external_port = ?", sniHostname, externalPort).
		Where("expires_at > ?", now).
		Find(&tcpRoutes)
	if err != nil {
		return nil, err
	}
	return tcpRoutes, nil
}

func (s *SqlDB) FindExistingTcpRouteMapping(tcpMapping models.TcpRouteMapping) (models.TcpRouteMapping, error) {
	var routes []models.TcpRouteMapping
	var tcpRoute models.TcpRouteMapping
	var err error

	// this where clause should represent all fields marked with the unique index on the TcpRouteMapping model,
	// to ensure it returns the correct record from the database
	if tcpMapping.SniHostname == nil {
		err = s.Client.Where("router_group_guid = ? and host_ip = ? and host_port = ? and external_port = ? and host_tls_port = ? and sni_hostname IS NULL",
			tcpMapping.RouterGroupGuid, tcpMapping.HostIP, tcpMapping.HostPort, tcpMapping.ExternalPort, tcpMapping.HostTLSPort).Find(&routes)
	} else {
		err = s.Client.Where("router_group_guid = ? and host_ip = ? and host_port = ? and external_port = ? and host_tls_port = ? and sni_hostname = ?",
			tcpMapping.RouterGroupGuid, tcpMapping.HostIP, tcpMapping.HostPort, tcpMapping.ExternalPort, tcpMapping.HostTLSPort, tcpMapping.SniHostname).Find(&routes)
	}

	if err != nil {
		return tcpRoute, err
	}
	count := len(routes)
	if count > 1 || count < 0 {
		return tcpRoute, errors.New("have duplicate tcp route mappings")
	}
	if count == 1 {
		tcpRoute = routes[0]
	}

	return tcpRoute, err
}

func (s *SqlDB) emitEvent(eventType EventType, obj interface{}) error {
	event, err := NewEventFromInterface(eventType, obj)
	if err != nil {
		return err
	}

	switch obj.(type) {
	case models.Route:
		s.httpEventHub.Emit(event)
	case models.TcpRouteMapping:
		s.tcpEventHub.Emit(event)
	default:
		return errors.New("unknown event type")
	}
	return nil
}

func (s *SqlDB) SaveTcpRouteMapping(tcpRouteMapping models.TcpRouteMapping) error {
	existingTcpRouteMapping, err := s.FindExistingTcpRouteMapping(tcpRouteMapping)
	if err != nil {
		return err
	}

	if existingTcpRouteMapping != (models.TcpRouteMapping{}) {
		newTcpRouteMapping := updateTcpRouteMapping(existingTcpRouteMapping, tcpRouteMapping)
		_, err = s.Client.Save(&newTcpRouteMapping)
		if err != nil {
			return err
		}
		return s.emitEvent(UpdateEvent, newTcpRouteMapping)
	}

	tcpMapping, err := models.NewTcpRouteMappingWithModel(tcpRouteMapping)
	if err != nil {
		return err
	}

	tag, err := models.NewModificationTag()
	if err != nil {
		return err
	}
	tcpMapping.ModificationTag = tag

	_, err = s.Client.Create(&tcpMapping)
	if err != nil {
		return err
	}

	return s.emitEvent(CreateEvent, tcpMapping)
}

func (s *SqlDB) DeleteTcpRouteMapping(tcpMapping models.TcpRouteMapping) error {
	tcpMapping, err := s.FindExistingTcpRouteMapping(tcpMapping)
	if err != nil {
		return err
	}
	if tcpMapping == (models.TcpRouteMapping{}) {
		return DeleteRouteError
	}

	_, err = s.Client.Delete(&tcpMapping)
	if err != nil {
		return err
	}
	return s.emitEvent(DeleteEvent, tcpMapping)
}

func (s *SqlDB) Connect() error {
	return notImplementedError()
}

func (s *SqlDB) CancelWatches() {
	// This only errors if the eventhub was closed.
	_ = s.tcpEventHub.Close()
	_ = s.httpEventHub.Close()
}

func (s *SqlDB) WatchChanges(watchType string) (<-chan Event, <-chan error, context.CancelFunc) {
	var (
		sub eventhub.Source
		err error
	)
	events := make(chan Event)
	errors := make(chan error, 1)
	cancelFunc := func() {}

	switch watchType {
	case TCP_WATCH:
		sub, err = s.tcpEventHub.Subscribe()
		if err != nil {
			errors <- err
			close(events)
			close(errors)
			return events, errors, cancelFunc
		}
	case HTTP_WATCH:
		sub, err = s.httpEventHub.Subscribe()
		if err != nil {
			errors <- err
			close(events)
			close(errors)
			return events, errors, cancelFunc
		}
	default:
		err := fmt.Errorf("invalid watch type: %s", watchType)
		errors <- err
		close(events)
		close(errors)
		return events, errors, cancelFunc
	}

	cancelFunc = func() {
		_ = sub.Close()
	}

	go dispatchWatchEvents(sub, events, errors)

	return events, errors, cancelFunc
}

func dispatchWatchEvents(sub eventhub.Source, events chan<- Event, errors chan<- error) {
	defer close(events)
	defer close(errors)
	for {
		event, err := sub.Next()
		if err != nil {
			if err == eventhub.ErrReadFromClosedSource {
				return
			}
			errors <- err
			return
		}
		watchEvent, ok := event.(Event)
		if !ok {
			errors <- fmt.Errorf("incoming event is not a db.Event: %#v", event)
		}

		events <- watchEvent
	}
}

func recordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func ConnectionString(cfg *config.SqlDB) (string, error) {
	var connectionString string
	switch cfg.Type {
	case "mysql":
		connStringBuilder := &MySQLConnectionStringBuilder{MySQLAdapter: &MySQLAdapter{}}
		return connStringBuilder.Build(cfg)

	case "postgres":
		var queryString string
		if cfg.CACert == "" {
			queryString = "?sslmode=disable"
		} else {
			if cfg.SkipSSLValidation {
				queryString = "?sslmode=require"
			} else {
				tempDir, err := os.MkdirTemp("", "")
				if err != nil {
					return "", err
				}
				certPath := filepath.Join(tempDir, "postgres_cert.pem")
				err = os.WriteFile(certPath, []byte(cfg.CACert), 0400)
				if err != nil {
					return "", err
				}
				queryString = fmt.Sprintf("?sslmode=verify-full&sslrootcert=%s", certPath)
			}
		}
		connectionString = fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s%s",
			cfg.Username,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Schema,
			queryString,
		)
	}

	return connectionString, nil
}
