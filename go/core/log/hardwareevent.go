// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Component string

const (
	// These components align with the enum of allowed values in HDB.
	ControllerComponent    Component = "controller"
	CoolingDeviceComponent Component = "coolingdevice"
	CpuComponent           Component = "cpu"
	DimmComponent          Component = "dimm"
	DiskComponent          Component = "disk"
	FPGAComponent          Component = "fpga"
	FansComponent          Component = "fans"
	GPUComponent           Component = "gpu"
	ManagementComponent    Component = "management"
	MotherboardComponent   Component = "motherboard"
	NICComponent           Component = "nic"
	PCIComponent           Component = "pci"
	PowerComponent         Component = "power"
	UnknownComponent       Component = "unknown"
)

type HardwareEvent struct {
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
	Component Component `json:"component"`
	Location  string    `json:"location"`
	Message   string    `json:"message"`
}

// Simple client to send hardware events to a server that echoes them into HDB.
// For now, the only server is the buildserver so the API is exactly that of the buildserver.
type HardwareEventClient struct {
	serverURL string
	client    http.Client
}

func NewHardwareEventClient(serverURL string) HardwareEventClient {
	return HardwareEventClient{serverURL: serverURL, client: http.Client{
		Timeout: 10 * time.Second,
	}}
}

func (hec *HardwareEventClient) SendHardwareEvent(he HardwareEvent) error {
	// The API expects a list of hardware events, so we have to wrap the single event.
	b, err := json.Marshal([]HardwareEvent{he})
	if err != nil {
		return fmt.Errorf("failed to convert the HardwareEvent to json: %w", err)
	}
	resp, err := hec.client.Post(fmt.Sprintf("%s/hardware_event", hec.serverURL), "text/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to send hardware event to server (%s): %w", hec.serverURL, err)
	} else if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("failed to send hardware event to server: bad status code: %d", resp.StatusCode)
	}
	return nil
}
