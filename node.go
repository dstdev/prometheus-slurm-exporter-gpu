/* Copyright 2021 Chris Read

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>. */

package main

import (
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
)

// NodeMetrics stores metrics for each node
type NodeMetrics struct {
	memAlloc uint64
	memTotal uint64
	cpuAlloc uint64
	cpuIdle  uint64
	cpuOther uint64
	cpuTotal uint64
	gpuAlloc uint64
	gpuIdle  uint64
	gpuOther uint64
	gpuTotal uint64
	nodeStatus string
}

func NodeGetMetrics() map[string]*NodeMetrics {
	return ParseNodeMetrics(NodeData())
}

// ParseNodeMetrics takes the output of sinfo with node data
// It returns a map of metrics per node
func ParseNodeMetrics(input []byte) map[string]*NodeMetrics {
	nodes := make(map[string]*NodeMetrics)
	lines := strings.Split(string(input), "\n")

	// Sort and remove all the duplicates from the 'sinfo' output
	sort.Strings(lines)
	linesUniq := RemoveDuplicates(lines)

	for _, line := range linesUniq {
		node := strings.Fields(line)
		nodeName := node[0]
		nodeStatus := node[4] // mixed, allocated, etc.

		nodes[nodeName] = &NodeMetrics{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, ""}

		memAlloc, _ := strconv.ParseUint(node[1], 10, 64)
		memTotal, _ := strconv.ParseUint(node[2], 10, 64)


		cpuInfo := strings.Split(node[3], "/")
		cpuAlloc, _ := strconv.ParseUint(cpuInfo[0], 10, 64)
		cpuIdle, _ := strconv.ParseUint(cpuInfo[1], 10, 64)
		cpuOther, _ := strconv.ParseUint(cpuInfo[2], 10, 64)
		cpuTotal, _ := strconv.ParseUint(cpuInfo[3], 10, 64)

		nodes[nodeName].memAlloc = memAlloc
		nodes[nodeName].memTotal = memTotal
		nodes[nodeName].cpuAlloc = cpuAlloc
		nodes[nodeName].cpuIdle = cpuIdle
		nodes[nodeName].cpuOther = cpuOther
		nodes[nodeName].cpuTotal = cpuTotal
		// nodes[nodeName].gpuAlloc = gpuAlloc
		// nodes[nodeName].gpuIdle = gpuIdle
		// nodes[nodeName].gpuOther = gpuOther
		// nodes[nodeName].gpuTotal = gpuTotal
		nodes[nodeName].nodeStatus = nodeStatus
		
        gpuGres := strings.Split(node[5], ":")
		gpuGresUsed := strings.Split(node[6], ":")
		re := regexp.MustCompile(`\d[\d,]*[\.]?[\d{2}]*`)

		if len(gpuGres) >= 3 {
			gpuAlloc, _ := strconv.ParseUint((re.FindAllString(gpuGresUsed[2], 1))[0], 10, 64)
			gpuTotal, _ := strconv.ParseUint((re.FindAllString(gpuGres[2], 1))[0], 10, 64)
			gpuIdle := gpuTotal - gpuAlloc
			gpuOther, _ := strconv.ParseUint("0", 10, 64)
			
			nodes[nodeName].gpuAlloc = gpuAlloc
			nodes[nodeName].gpuIdle = gpuIdle
			nodes[nodeName].gpuOther = gpuOther
			nodes[nodeName].gpuTotal = gpuTotal
		} else {
			nodes[nodeName].gpuAlloc = 0
			nodes[nodeName].gpuIdle = 0
			nodes[nodeName].gpuOther = 0
			nodes[nodeName].gpuTotal = 0
		}
		
	}

	return nodes
}

// NodeData executes the sinfo command to get data for each node
// It returns the output of the sinfo command
func NodeData() []byte {
	cmd := exec.Command("sinfo", "-h", "-N", "-O", "NodeList,AllocMem,Memory,CPUsState,StateLong,Gres:30,GresUsed:30")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return out
}

type NodeCollector struct {
	cpuAlloc *prometheus.Desc
	cpuIdle  *prometheus.Desc
	cpuOther *prometheus.Desc
	cpuTotal *prometheus.Desc
	gpuAlloc *prometheus.Desc
	gpuIdle  *prometheus.Desc
	gpuOther *prometheus.Desc
	gpuTotal *prometheus.Desc
	memAlloc *prometheus.Desc
	memTotal *prometheus.Desc
}

// NewNodeCollector creates a Prometheus collector to keep all our stats in
// It returns a set of collections for consumption
func NewNodeCollector() *NodeCollector {
	labels := []string{"node","status"}

	return &NodeCollector{
		cpuAlloc: prometheus.NewDesc("slurm_node_cpu_alloc", "Allocated CPUs per node", labels, nil),
		cpuIdle:  prometheus.NewDesc("slurm_node_cpu_idle", "Idle CPUs per node", labels, nil),
		cpuOther: prometheus.NewDesc("slurm_node_cpu_other", "Other CPUs per node", labels, nil),
		cpuTotal: prometheus.NewDesc("slurm_node_cpu_total", "Total CPUs per node", labels, nil),
		gpuAlloc: prometheus.NewDesc("slurm_node_gpu_alloc", "Allocated GPUs per node", labels, nil),
		gpuIdle:  prometheus.NewDesc("slurm_node_gpu_idle", "Idle GPUs per node", labels, nil),
		gpuOther: prometheus.NewDesc("slurm_node_gpu_other", "Other GPUs per node", labels, nil),
		gpuTotal: prometheus.NewDesc("slurm_node_gpu_total", "Total GPUs per node", labels, nil),
		memAlloc: prometheus.NewDesc("slurm_node_mem_alloc", "Allocated memory per node", labels, nil),
		memTotal: prometheus.NewDesc("slurm_node_mem_total", "Total memory per node", labels, nil),
	}
}

// Send all metric descriptions
func (nc *NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.cpuAlloc
	ch <- nc.cpuIdle
	ch <- nc.cpuOther
	ch <- nc.cpuTotal
	ch <- nc.gpuAlloc
	ch <- nc.gpuIdle
	ch <- nc.gpuOther
	ch <- nc.gpuTotal
	ch <- nc.memAlloc
	ch <- nc.memTotal
}

func (nc *NodeCollector) Collect(ch chan<- prometheus.Metric) {
	nodes := NodeGetMetrics()
	for node := range nodes {
		ch <- prometheus.MustNewConstMetric(nc.cpuAlloc, prometheus.GaugeValue, float64(nodes[node].cpuAlloc), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.cpuIdle,  prometheus.GaugeValue, float64(nodes[node].cpuIdle),  node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.cpuOther, prometheus.GaugeValue, float64(nodes[node].cpuOther), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.cpuTotal, prometheus.GaugeValue, float64(nodes[node].cpuTotal), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.gpuAlloc, prometheus.GaugeValue, float64(nodes[node].gpuAlloc), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.gpuIdle,  prometheus.GaugeValue, float64(nodes[node].gpuIdle),  node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.gpuOther, prometheus.GaugeValue, float64(nodes[node].gpuOther), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.gpuTotal, prometheus.GaugeValue, float64(nodes[node].gpuTotal), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.memAlloc, prometheus.GaugeValue, float64(nodes[node].memAlloc), node, nodes[node].nodeStatus)
		ch <- prometheus.MustNewConstMetric(nc.memTotal, prometheus.GaugeValue, float64(nodes[node].memTotal), node, nodes[node].nodeStatus)
	}
}
