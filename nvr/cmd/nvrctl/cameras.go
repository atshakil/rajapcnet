package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"nvr/internal/client"
	"nvr/internal/model"
)

func camerasUsage() {
	fmt.Fprintf(os.Stderr, `Usage: nvrctl cameras <action>

Actions:
  list, ls           List all cameras
  add                Add a camera (interactive wizard)
  get <id>           Show camera details
  update <id> <json> Update camera (JSON body)
  delete, rm <id>    Delete a camera
  status             Connectivity status of all cameras
  diagnose           Full diagnostics for all cameras
`)
	os.Exit(1)
}

func cmdCamerasList(c *client.Client) error {
	cameras, err := c.ListCameras()
	if err != nil {
		return err
	}
	if len(cameras) == 0 {
		fmt.Println("No cameras configured.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tIP\tMANUFACTURER\tMODEL\tONVIF\tPTZ\tMOTION")
	for _, cam := range cameras {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cam.ID, cam.Name, cam.IP,
			dash(cam.Manufacturer), dash(cam.Model),
			yn(cam.HasONVIF), yn(cam.HasPTZ), yn(cam.HasMotion))
	}
	return w.Flush()
}

func cmdCamerasAdd(c *client.Client) error {
	fmt.Println("Add Camera Wizard")
	fmt.Println("─────────────────")

	name := prompt("Name", "")
	if name == "" {
		return fmt.Errorf("name is required")
	}

	ip := prompt("IP address", "")
	if ip == "" {
		return fmt.Errorf("IP is required")
	}

	portStr := prompt("HTTP port", "80")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port: %v", err)
	}

	rtspStr := prompt("RTSP port", "554")
	rtspPort, err := strconv.Atoi(rtspStr)
	if err != nil {
		return fmt.Errorf("invalid RTSP port: %v", err)
	}

	username := prompt("Username (ONVIF/camera)", "admin")
	password := promptSecret("Password")

	onvifPath := prompt("ONVIF path", "/onvif/device_service")
	streamPath := prompt("Stream path (blank to auto-detect)", "")

	fmt.Println()
	fmt.Printf("  Testing connectivity to %s...\n", ip)

	httpOk := probe(ip, strconv.Itoa(port))
	rtspOk := probe(ip, strconv.Itoa(rtspPort))
	fmt.Printf("  HTTP (%d/tcp): %s\n", port, httpOk)
	fmt.Printf("  RTSP (%d/tcp): %s\n", rtspPort, rtspOk)

	if httpOk == "FAIL" && rtspOk == "FAIL" {
		fmt.Println("  ⚠ Camera unreachable from this machine (may be on a different VLAN)")
		fmt.Println("  The NVR daemon will attempt to reach it server-side.")
	}

	fmt.Println()
	fmt.Println("  Sending to NVR for ONVIF probe and storage...")

	cam := &model.Camera{
		Name:       name,
		IP:         ip,
		Port:       port,
		RTSPPort:   rtspPort,
		Username:   username,
		Password:   password,
		ONVIFPath:  onvifPath,
		StreamPath: streamPath,
	}

	result, err := c.AddCamera(cam)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  ✓ Camera added (ID: %d)\n", result.ID)
	if result.Manufacturer != "" {
		fmt.Printf("  Manufacturer: %s\n", result.Manufacturer)
	}
	if result.Model != "" {
		fmt.Printf("  Model:        %s\n", result.Model)
	}
	if result.HasONVIF {
		fmt.Print("  Capabilities: ONVIF")
		if result.HasPTZ {
			fmt.Print(", PTZ")
		}
		if result.HasMotion {
			fmt.Print(", Motion")
		}
		if result.HasInfrared {
			fmt.Print(", IR")
		}
		fmt.Println()
	}
	if result.StreamURIs != "" && result.StreamURIs != "[]" {
		fmt.Printf("  Streams:      %s\n", result.StreamURIs)
	}
	if result.Resolutions != "" && result.Resolutions != "[]" {
		fmt.Printf("  Resolutions:  %s\n", result.Resolutions)
	}

	return nil
}

func cmdCamerasGet(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: nvrctl cameras get <id>")
	}
	cam, err := c.GetCamera(os.Args[3])
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(cam)
}

func cmdCamerasUpdate(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl cameras update <id> <json>")
	}
	var cam model.Camera
	if err := json.Unmarshal([]byte(os.Args[4]), &cam); err != nil {
		return fmt.Errorf("invalid json: %v", err)
	}
	if err := c.UpdateCamera(os.Args[3], &cam); err != nil {
		return err
	}
	fmt.Println("Updated.")
	return nil
}

func cmdCamerasDelete(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: nvrctl cameras delete <id>")
	}
	if err := c.DeleteCamera(os.Args[3]); err != nil {
		return err
	}
	fmt.Println("Deleted.")
	return nil
}

func cmdCamerasStatus(c *client.Client) error {
	cameras, err := c.ListCameras()
	if err != nil {
		return err
	}
	if len(cameras) == 0 {
		fmt.Println("No cameras configured.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tIP\tHTTP\tRTSP\tONVIF")
	for _, cam := range cameras {
		httpOk := probeHTTP(cam.IP, cam.Port)
		rtsp := probe(cam.IP, strconv.Itoa(cam.RTSPPort))
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			cam.ID, cam.Name, cam.IP, httpOk, rtsp, yn(cam.HasONVIF))
	}
	return w.Flush()
}

func cmdCamerasDiagnose(c *client.Client) error {
	cameras, err := c.ListCameras()
	if err != nil {
		return err
	}
	if len(cameras) == 0 {
		fmt.Println("No cameras configured.")
		return nil
	}

	for i, cam := range cameras {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s (ID: %d) ===\n", cam.Name, cam.ID)
		fmt.Printf("  IP:           %s\n", cam.IP)
		fmt.Printf("  HTTP Port:    %d\n", cam.Port)
		fmt.Printf("  RTSP Port:    %d\n", cam.RTSPPort)
		fmt.Printf("  ONVIF Path:   %s\n", cam.ONVIFPath)
		fmt.Printf("  Stream:       %s\n", cam.StreamPath)
		fmt.Printf("  Manufacturer: %s\n", dash(cam.Manufacturer))
		fmt.Printf("  Model:        %s\n", dash(cam.Model))
		fmt.Printf("  Firmware:     %s\n", dash(cam.Firmware))
		fmt.Printf("  Enabled:      %v\n", cam.Enabled)
		fmt.Println()

		fmt.Printf("  Capabilities:\n")
		fmt.Printf("    ONVIF:      %s\n", yn(cam.HasONVIF))
		fmt.Printf("    PTZ:        %s\n", yn(cam.HasPTZ))
		fmt.Printf("    Motion:     %s\n", yn(cam.HasMotion))
		fmt.Printf("    Infrared:   %s\n", yn(cam.HasInfrared))
		fmt.Printf("    Floodlight: %s\n", yn(cam.HasFloodlight))
		fmt.Printf("    Indicator:  %s\n", yn(cam.HasIndicator))
		fmt.Println()

		if cam.Resolutions != "" && cam.Resolutions != "[]" {
			fmt.Printf("  Resolutions:  %s\n", cam.Resolutions)
		}
		if cam.StreamURIs != "" && cam.StreamURIs != "[]" {
			fmt.Printf("  Stream URIs:  %s\n", cam.StreamURIs)
		}

		// Live connectivity
		fmt.Println()
		fmt.Printf("  [check] HTTP  (%d/tcp): %s\n", cam.Port, probe(cam.IP, strconv.Itoa(cam.Port)))
		fmt.Printf("  [check] RTSP  (%d/tcp): %s\n", cam.RTSPPort, probe(cam.IP, strconv.Itoa(cam.RTSPPort)))
	}
	return nil
}
