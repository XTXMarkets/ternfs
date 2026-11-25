// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// See <https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/>
// for docs.
package log

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	METRICS_INIT        uint8 = 0
	METRICS_MEASUREMENT uint8 = 1
	METRICS_TAGS        uint8 = 2
	METRICS_FIELDS      uint8 = 3
	METRICS_TIMESTAMP   uint8 = 4
)

type MetricsBuilder struct {
	state   uint8
	payload bytes.Buffer
}

func (b *MetricsBuilder) Reset() {
	b.state = METRICS_INIT
	b.payload.Reset()
}

func (b *MetricsBuilder) Payload() io.Reader {
	if !(b.state == METRICS_INIT || b.state == METRICS_TIMESTAMP) {
		panic(fmt.Errorf("bad state %v for payload", b.state))
	}
	return bytes.NewReader(b.payload.Bytes())
}

// We're being overly conservative here, see
// <https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters>,
// but better being safe than sorry.
func validMetricsName(name string) {
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == ':') {
			panic(fmt.Errorf("bad metrics name %q", name))
		}
	}
}

// Tag values additionally allow '+', so that a histogram's final bucket can
// carry the le="+Inf" label histogram_quantile requires. Line protocol
// only escapes commas, equals signs and spaces in tag values.
func validTagValue(value string) {
	for _, ch := range value {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == ':' || ch == '+') {
			panic(fmt.Errorf("bad metrics tag value %q", value))
		}
	}
}

func (b *MetricsBuilder) Measurement(name string) {
	if !(b.state == METRICS_INIT || b.state == METRICS_TIMESTAMP) {
		panic(fmt.Errorf("bad state %v for measurement", b.state))
	}
	validMetricsName(name)
	if b.state == METRICS_TIMESTAMP {
		b.payload.Write([]byte{'\n'})
	}
	b.payload.Write([]byte(name))
	b.state = METRICS_MEASUREMENT
}

func (b *MetricsBuilder) Tag(name string, value string) {
	if !(b.state == METRICS_MEASUREMENT || b.state == METRICS_TAGS) {
		panic(fmt.Errorf("bad state %v for tag", b.state))
	}
	validMetricsName(name)
	validTagValue(value)
	fmt.Fprintf(&b.payload, ",%s=%s", name, value)
	b.state = METRICS_TAGS
}

func (b *MetricsBuilder) fieldRaw(name string, value string) {
	if !(b.state == METRICS_MEASUREMENT || b.state == METRICS_TAGS || b.state == METRICS_FIELDS) {
		panic(fmt.Errorf("bad state %v for field", b.state))
	}
	validMetricsName(name)
	if b.state == METRICS_MEASUREMENT || b.state == METRICS_TAGS {
		b.payload.Write([]byte{' '})
	} else {
		b.payload.Write([]byte{','})
	}
	fmt.Fprintf(&b.payload, "%s=%s", name, value)
	b.state = METRICS_FIELDS

}

func (b *MetricsBuilder) FieldU64(name string, value uint64) {
	b.fieldRaw(name, fmt.Sprintf("%vi", value))
}

func (b *MetricsBuilder) FieldU32(name string, value uint32) {
	b.fieldRaw(name, fmt.Sprintf("%vi", value))
}

func (b *MetricsBuilder) Timestamp(t time.Time) {
	if !(b.state == METRICS_FIELDS) {
		panic(fmt.Errorf("bad state %v for timestamp", b.state))
	}
	fmt.Fprintf(&b.payload, " %v", t.UnixNano())
	b.state = METRICS_TIMESTAMP
}

type InfluxDB struct {
	Origin string
	Org    string
	Bucket string
}

func (idb *InfluxDB) SendMetrics(payload io.Reader) error {
	url := idb.Origin + "/api/v2/write?org=" + idb.Org + "&bucket=" + idb.Bucket + "&precision=ns"
	resp, err := http.Post(url, "text/plain", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The store explains a rejected payload in the body; keep the start
		// of it, which names the offending line.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("metrics store returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
