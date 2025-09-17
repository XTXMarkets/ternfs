// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
)

const C_PADDING_SKIP = 6
const BINS_OFFSET = 258 + C_PADDING_SKIP
const MIN_HEADER_LEN = BINS_OFFSET + 1
const EGGSFS_STATS_NUM_COUNTERS = 5

const (
	METRIC_OFFSET_SUCCESS = iota
	REQUEST_STATUS_ATTEMPTS
	REQUEST_STATUS_TIMEOUTS
	REQUEST_STATUS_FAILURES
	REQUEST_STATUS_NET_FAILURES
)

type kernelMetricsHeader struct {
	Version          int
	BinsCount        int
	RequestMap       []uint8
	RequestKindMax   int
	UpperBoundValues []uint64
}

type kernelRequestCounters struct {
	Kind        uint8
	Success     uint64
	Attempts    uint64
	Timeouts    uint64
	Failures    uint64
	NetFailures uint64
}

type kernelLatencies struct {
	Kind        uint8
	LatencyBins []uint64
}

func reqKindCount(header []byte) int {
	ct := 0
	for _, v := range header {
		if v != 0xff {
			ct++
		}
	}
	return ct
}

func parseKernelMetricsHeader(serviceType string) (kernelMetricsHeader, error) {
	mf := fmt.Sprintf("/sys/kernel/debug/eggsfs/%s_stats_header", serviceType)
	d, err := ioutil.ReadFile(mf)
	if err != nil {
		return kernelMetricsHeader{}, fmt.Errorf("error reading file %s: %v", mf, err)
	}

	if len(d) < MIN_HEADER_LEN {
		return kernelMetricsHeader{}, fmt.Errorf("invalid content in file %s: expected at least %d bytes, got %d", mf, MIN_HEADER_LEN, len(d))
	}

	ret := kernelMetricsHeader{Version: int(d[0])}
	ret.BinsCount = int(d[1])

	md := d[2:258]
	ret.RequestKindMax = reqKindCount(md)
	var reqMap []uint8 = make([]uint8, ret.RequestKindMax)
	for k, v := range md {
		if v != 0xff {
			reqMap[int(v)] = uint8(k)
		}
	}
	ret.RequestMap = reqMap

	d = d[BINS_OFFSET:]
	var bins []uint64 = make([]uint64, ret.BinsCount)
	buf := bytes.NewReader(d)
	if err := binary.Read(buf, binary.LittleEndian, &bins); err != nil {
		return kernelMetricsHeader{}, fmt.Errorf("reading bin upper bounds from %s failed: %v", mf, err)
	}
	ret.UpperBoundValues = append(ret.UpperBoundValues, bins...)

	return ret, nil
}

func parseKernelMetricsCounters(serviceType string, nInstances int, metricsHeader kernelMetricsHeader) ([]kernelRequestCounters, error) {
	mf := fmt.Sprintf("/sys/kernel/debug/eggsfs/%s_counters", serviceType)
	d, err := ioutil.ReadFile(mf)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", mf, err)
	}

	var metrics []uint64 = make([]uint64, nInstances*metricsHeader.RequestKindMax*EGGSFS_STATS_NUM_COUNTERS)
	buf := bytes.NewReader(d)
	if err := binary.Read(buf, binary.LittleEndian, &metrics); err != nil {
		return nil, fmt.Errorf("binary.Read from %s failed: %v", mf, err)
	}

	ret := []kernelRequestCounters{}
	for j := 0; j < metricsHeader.RequestKindMax; j++ {
		rm := kernelRequestCounters{Kind: uint8(metricsHeader.RequestMap[j])}
		ret = append(ret, rm)
	}

	for i := 0; i < nInstances; i++ {
		for j := 0; j < metricsHeader.RequestKindMax; j++ {
			rIdx := i*metricsHeader.RequestKindMax*EGGSFS_STATS_NUM_COUNTERS + j*EGGSFS_STATS_NUM_COUNTERS
			ret[j].Success += metrics[rIdx+METRIC_OFFSET_SUCCESS]
			ret[j].Attempts += metrics[rIdx+REQUEST_STATUS_ATTEMPTS]
			ret[j].Timeouts += metrics[rIdx+REQUEST_STATUS_TIMEOUTS]
			ret[j].Failures += metrics[rIdx+REQUEST_STATUS_FAILURES]
			ret[j].NetFailures += metrics[rIdx+REQUEST_STATUS_NET_FAILURES]
		}
	}
	return ret, nil
}

func parseShardKernelCounters(metricsHeader kernelMetricsHeader) ([]kernelRequestCounters, error) {
	return parseKernelMetricsCounters("shard", 256, metricsHeader)
}

func parseCDCKernelCounters(metricsHeader kernelMetricsHeader) ([]kernelRequestCounters, error) {
	return parseKernelMetricsCounters("cdc", 1, metricsHeader)
}

func parseKernelLatencies(serviceType string, nInstances int, metricsHeader kernelMetricsHeader) ([]kernelLatencies, error) {
	mf := fmt.Sprintf("/sys/kernel/debug/eggsfs/%s_latencies", serviceType)
	d, err := ioutil.ReadFile(mf)
	if err != nil {
		return nil, fmt.Errorf("error reading latencies file %s: %v", mf, err)
	}

	var latencies []uint64 = make([]uint64, nInstances*metricsHeader.RequestKindMax*metricsHeader.BinsCount)
	buf := bytes.NewReader(d)
	if err := binary.Read(buf, binary.LittleEndian, &latencies); err != nil {
		return nil, fmt.Errorf("binary.Read failed from %s:%v", mf, err)
	}

	ret := []kernelLatencies{}
	// Initialise
	for j := 0; j < metricsHeader.RequestKindMax; j++ {
		st := kernelLatencies{Kind: metricsHeader.RequestMap[j]}
		st.LatencyBins = make([]uint64, metricsHeader.BinsCount)
		ret = append(ret, st)
	}
	// Fill
	for i := 0; i < nInstances; i++ {
		for j := 0; j < metricsHeader.RequestKindMax; j++ {
			rIdx := i*metricsHeader.RequestKindMax*metricsHeader.BinsCount + j*metricsHeader.BinsCount
			for k := 0; k < metricsHeader.BinsCount; k++ {
				ret[j].LatencyBins[k] += latencies[rIdx+k]
			}
		}
	}
	return ret, nil
}

func parseShardKernelLatencies(metricsHeader kernelMetricsHeader) ([]kernelLatencies, error) {
	return parseKernelLatencies("shard", 256, metricsHeader)
}

func parseCDCKernelLatencies(metricsHeader kernelMetricsHeader) ([]kernelLatencies, error) {
	return parseKernelLatencies("cdc", 1, metricsHeader)
}
