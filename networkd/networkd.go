// Copyright 2023 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networkd

import (
	"context"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "networkd"

type Collector struct {
	ctx                    context.Context
	logger                 log.Logger
	leases                 *prometheus.Desc
	links                  *prometheus.Desc
	link_carrier_state     *prometheus.Desc
	link_online_state      *prometheus.Desc
	link_operational_state *prometheus.Desc
}

// NewCollector returns a new Collector exporing networkd statistics
func NewCollector(logger log.Logger) (*Collector, error) {
	leases := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "dhcpserver_leases_total"),
		"networkd DHCP server leases",
		[]string{"iface"}, nil,
	)
	links := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "links_total"),
		"networkd links",
		nil, nil,
	)

	ctx := context.TODO()
	return &Collector{
		ctx:                    ctx,
		logger:                 logger,
		leases:                 leases,
		links:                  links,
	}, nil
}

// Collect gathers metrics from networkd
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	err := c.collect(ch)
	if err != nil {
		level.Error(c.logger).Log("msg", "error collecting metrics",
			"err", err)
	}
}

// Describe gathers descriptions of metrics
func (c *Collector) Describe(desc chan<- *prometheus.Desc) {
	desc <- c.leases
	desc <- c.links
}

func (c *Collector) collect(ch chan<- prometheus.Metric) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return errors.Wrapf(err, "could not get DBus connection")
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.network1", "/org/freedesktop/network1")

	var links [][]interface{}

	err = obj.Call("org.freedesktop.network1.Manager.ListLinks", 0).Store(&links)
	if err != nil {
		return level.Warn(c.logger).Log("msg", "Unable to list networkd links", "err", err)
	}

	// Record number of links
	ch <- prometheus.MustNewConstMetric(c.links, prometheus.GaugeValue, float64(len(links)))

	//var leases []string
	for _, v := range links {
		link_obj_iface := v[1].(string)
		link_obj_path := v[2].(dbus.ObjectPath)

		link_obj := conn.Object("org.freedesktop.network1", link_obj_path)

		leases_property, err := link_obj.GetProperty("org.freedesktop.network1.DHCPServer.Leases")
		if err != nil {
			// No leases found
			level.Debug(c.logger).Log("msg", "No leases found for interface", "err", err)
			continue
		}

		leases_count := len(leases_property.Value().([][]interface{}))
		ch <- prometheus.MustNewConstMetric(c.leases, prometheus.GaugeValue, float64(leases_count), link_obj_iface)
	}

	return nil
}
