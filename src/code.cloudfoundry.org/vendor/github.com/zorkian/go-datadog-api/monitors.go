/*
 * Datadog API for Go
 *
 * Please see the included LICENSE file for licensing information.
 *
 * Copyright 2013 by authors and contributors.
 */

package datadog

import (
	"fmt"
)

type ThresholdCount struct {
	Ok       int `json:"ok"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
}

type Options struct {
	NoDataTimeframe   int               `json:"no_data_timeframe"`
	NotifyAudit       bool              `json:"notify_audit"`
	NotifyNoData      bool              `json:"notify_no_data"`
	Period            int               `json:"period"`
	RenotifyInterval  int               `json:"renotify_interval"`
	Silenced          map[string]string `json:"silenced"`
	TimeoutH          int               `json:"timeout_h"`
	EscalationMessage string            `json:"escalation_message"`
	Thresholds        ThresholdCount    `json:"thresholds"`
}

//Monitors allow you to watch a metric or check that you care about,
//notifying your team when some defined threshold is exceeded.
type Monitor struct {
	Id      int     `json:"id"`
	Type    string  `json:"type"`
	Query   string  `json:"query"`
	Name    string  `json:"name"`
	Message string  `json:"message"`
	Options Options `json:"options"`
}

// reqMonitors receives a slice of all monitors
type reqMonitors struct {
	Monitors []Monitor `json:"monitors"`
}

// Createmonitor adds a new monitor to the system. This returns a pointer to an
// monitor so you can pass that to Updatemonitor later if needed.
func (self *Client) CreateMonitor(monitor *Monitor) (*Monitor, error) {
	var out Monitor
	err := self.doJsonRequest("POST", "/v1/monitor", monitor, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Updatemonitor takes an monitor that was previously retrieved through some method
// and sends it back to the server.
func (self *Client) UpdateMonitor(monitor *Monitor) error {
	return self.doJsonRequest("PUT", fmt.Sprintf("/v1/monitor/%d", monitor.Id),
		monitor, nil)
}

// Getmonitor retrieves an monitor by identifier.
func (self *Client) GetMonitor(id int) (*Monitor, error) {
	var out Monitor
	err := self.doJsonRequest("GET", fmt.Sprintf("/v1/monitor/%d", id), nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Deletemonitor removes an monitor from the system.
func (self *Client) DeleteMonitor(id int) error {
	return self.doJsonRequest("DELETE", fmt.Sprintf("/v1/monitor/%d", id),
		nil, nil)
}

// GetMonitors returns a slice of all monitors.
func (self *Client) GetMonitors() ([]Monitor, error) {
	var out reqMonitors
	err := self.doJsonRequest("GET", "/v1/monitor", nil, &out.Monitors)
	if err != nil {
		return nil, err
	}
	return out.Monitors, nil
}

// MuteMonitors turns off monitoring notifications.
func (self *Client) MuteMonitors() error {
	return self.doJsonRequest("POST", "/v1/monitor/mute_all", nil, nil)
}

// UnmuteMonitors turns on monitoring notifications.
func (self *Client) UnmuteMonitors() error {
	return self.doJsonRequest("POST", "/v1/monitor/unmute_all", nil, nil)
}

// MuteMonitor turns off monitoring notifications for a monitor.
func (self *Client) MuteMonitor(id int) error {
	return self.doJsonRequest("POST", fmt.Sprintf("/v1/monitor/%d/mute", id), nil, nil)
}

// UnmuteMonitor turns on monitoring notifications for a monitor.
func (self *Client) UnmuteMonitor(id int) error {
	return self.doJsonRequest("POST", fmt.Sprintf("/v1/monitor/%d/unmute", id), nil, nil)
}
