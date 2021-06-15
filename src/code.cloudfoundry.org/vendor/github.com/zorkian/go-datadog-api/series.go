/*
 * Datadog API for Go
 *
 * Please see the included LICENSE file for licensing information.
 *
 * Copyright 2013 by authors and contributors.
 */

package datadog

// DataPoint is a tuple of [UNIX timestamp, value]. This has to use floats
// because the value could be non-integer.
type DataPoint [2]float64

// Metric represents a collection of data points that we might send or receive
// on one single metric line.
type Metric struct {
	Metric string      `json:"metric,omitempty"`
	Points []DataPoint `json:"points,omitempty"`
	Type   string      `json:"type,omitempty"`
	Host   string      `json:"host,omitempty"`
	Tags   []string    `json:"tags,omitempty"`
}

// reqPostSeries from /api/v1/series
type reqPostSeries struct {
	Series []Metric `json:"series"`
}

// PostSeries takes as input a slice of metrics and then posts them up to the
// server for posting data.
func (self *Client) PostMetrics(series []Metric) error {
	return self.doJsonRequest("POST", "/v1/series",
		reqPostSeries{Series: series}, nil)
}
