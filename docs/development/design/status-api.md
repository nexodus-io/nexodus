# Design Document: Status API

## Introduction

The goal of this design document is to outline the purpose and implementation of the status API.

## The Problem

There is a visual disconnect for a user's Nexodus network of devices.

## The Solution

With the addition of an API endpoint, device statuses can be created and stored. The status of devices in a user's Nexodus network can then be visualized in a web interface. The network will be displayed as a graph.

## Implementation

A general view of a device's connectivity includes:

  1. Devices a source device is attempting to directly connect to
  2. Is the device connected to a relay and which devices is it using the relay to connect to
  3. Which devies can it reach successfully
  4. The latecy of the source to each device in the network

The proposed API additions are as follows:

```go
    // Create new statuses
    private.POST("/status", api.CreateStatus)
    // List all satuses for a user's devices
    private.GET("/status", api.ListStatuses)
    // Updates a device status if a status already exists
    private.PUT("/status", api.UpdateStatus)
```

## New Tables

    A new table will be defined for device statuses.

### Status Model

- New models

```go
    // Status represents the status of single user device 
    type Status struct {
        Base
        UserId      uuid.UUID `json:"user_id"`
        WgIP        string    `json:"wg_ip"`
        IsReachable bool      `json:"is_reachable"`
        Hostname    string    `json:"hostname"`
        Latency     string    `json:"latency"`
        Method      string    `json:"method"`
    }

    // AddStatus is the information necessary to add a device status
    type AddStatus struct {
        WgIP        string `json:"wg_ip"`
        IsReachable bool   `json:"is_reachable"`
        Hostname    string `json:"hostname"`
        Latency     string `json:"latency"`
        Method      string `json:"method"`
    }

    // UpdateStatus is the information needed to update a device status that already exists
    type UpdateStatus struct{
        WgIP        string  `json:"wg_ip"`
        IsReachable bool   `json:"is_reachable"`
        Latency     string `json:""`
        Method      string `json:"method"`
    }
```

### The Graph Display

React Flow library was chosen for its flexibility and support for customization. The custom nodes were created with a set of criteria in mind including the shape of the node, the border color based on latency, icons for the connection status, and what information to include in the table when the node is clicked on. The device nodes are rectangular in shape, while the relay nodes are oval shaped. The border color will display green if the latency is in the range of 0 to 40 milliseconds, gold/orange if the latency is between 41 to 80 milliseconds, and red if the latency is 81 milliseconds or higher. If a device or relay is reachable, then a green check mark icon will appear on the right side of the node. If a device or relay becomes unreachable, then a red X icon will appear instead. The graph organizes itself show devices connected directly to the host device and those connected to a relay. The nodes are draggable for better organization based on user preferences. The data is fetched every 3 minutes and the information in the nodes is updated.

