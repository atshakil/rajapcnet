package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"

	"nvr/internal/client"
	"nvr/internal/model"
)

func main() {
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		_ = godotenv.Load(".env.host")
	} else {
		_ = godotenv.Load(".env.client")
	}

	base := os.Getenv("NVR_URL")
	if base == "" {
		base = "http://localhost:8080"
	}

	c := client.New(base)

	if len(os.Args) < 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "health":
		err = cmdHealth(c)
	case "cameras":
		if len(os.Args) < 3 {
			camerasUsage()
		}
		switch os.Args[2] {
		case "list", "ls":
			err = cmdCamerasList(c)
		case "add":
			err = cmdCamerasAdd(c)
		case "get":
			err = cmdCamerasGet(c)
		case "update":
			err = cmdCamerasUpdate(c)
		case "delete", "rm":
			err = cmdCamerasDelete(c)
		case "status":
			err = cmdCamerasStatus(c)
		case "diagnose":
			err = cmdCamerasDiagnose(c)
		default:
			camerasUsage()
		}
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: nvrctl <command>

Commands:
  health             Check NVR server health
  cameras <action>   Manage cameras

Environment:
  NVR_URL   NVR server URL (default: http://localhost:8080)
`)
	os.Exit(1)
}

func camerasUsage() {
	fmt.Fprintf(os.Stderr, `Usage: nvrctl cameras <action>

Actions:
  list, ls                       List all cameras
  add <name> <ip> [rtsp_port]    Add a camera
  get <id>                       Show camera details
  update <id> <json>             Update camera (JSON body)
  delete, rm <id>                Delete a camera
  status                         Connectivity status of all cameras
  diagnose                       Full diagnostics for all cameras
`)
	os.Exit(1)
}

func cmdHealth(c *client.Client) error {
	status, err := c.Health()
	if err != nil {
		return err
	}
	fmt.Println(status)
	return nil
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
	fmt.Fprintln(w, "ID\tNAME\tIP\tPORT\tRTSP\tENABLED")
	for _, cam := range cameras {
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%d\t%v\n",
			cam.ID, cam.Name, cam.IP, cam.Port, cam.RTSPPort, cam.Enabled)
	}
	return w.Flush()
}

func cmdCamerasAdd(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl cameras add <name> <ip> [rtsp_port]")
	}
	cam := &model.Camera{
		Name:     os.Args[3],
		IP:       os.Args[4],
		Port:     80,
		RTSPPort: 554,
	}
	if len(os.Args) >= 6 {
		p, err := strconv.Atoi(os.Args[5])
		if err != nil {
			return fmt.Errorf("invalid rtsp_port: %v", err)
		}
		cam.RTSPPort = p
	}
	result, err := c.AddCamera(cam)
	if err != nil {
		return err
	}
	fmt.Printf("Added camera %d: %s (%s)\n", result.ID, result.Name, result.IP)
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
	fmt.Fprintln(w, "ID\tNAME\tIP\tPING\tHTTP\tRTSP")
	for _, cam := range cameras {
		ping := probe(cam.IP, "icmp")
		httpOk := probeHTTP(cam.IP, cam.Port)
		rtsp := probe(cam.IP, strconv.Itoa(cam.RTSPPort))
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			cam.ID, cam.Name, cam.IP, ping, httpOk, rtsp)
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
		fmt.Printf("  IP:         %s\n", cam.IP)
		fmt.Printf("  HTTP Port:  %d\n", cam.Port)
		fmt.Printf("  RTSP Port:  %d\n", cam.RTSPPort)
		fmt.Printf("  ONVIF Path: %s\n", cam.ONVIFPath)
		fmt.Printf("  Stream:     %s\n", cam.StreamPath)
		fmt.Printf("  Enabled:    %v\n", cam.Enabled)
		fmt.Println()

		// TCP connectivity
		fmt.Printf("  [check] HTTP  (%d/tcp): %s\n", cam.Port, probe(cam.IP, strconv.Itoa(cam.Port)))
		fmt.Printf("  [check] RTSP  (%d/tcp): %s\n", cam.RTSPPort, probe(cam.IP, strconv.Itoa(cam.RTSPPort)))
		fmt.Printf("  [check] ONVIF (%d/tcp): %s\n", cam.Port, probeHTTP(cam.IP, cam.Port))

		// ONVIF endpoint
		onvifURL := fmt.Sprintf("http://%s:%d%s", cam.IP, cam.Port, cam.ONVIFPath)
		fmt.Printf("  [check] ONVIF endpoint: %s → %s\n", onvifURL, probeURL(onvifURL))

		// RTSP URL
		if cam.StreamPath != "" {
			rtspURL := fmt.Sprintf("rtsp://%s:%d%s", cam.IP, cam.RTSPPort, cam.StreamPath)
			fmt.Printf("  [info]  RTSP URL: %s\n", rtspURL)
		} else {
			fmt.Printf("  [warn]  No stream path configured\n")
		}
	}
	return nil
}

func probe(host, port string) string {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return "FAIL"
	}
	conn.Close()
	return "OK"
}

func probeHTTP(host string, port int) string {
	url := fmt.Sprintf("http://%s:%d/", host, port)
	return probeURL(url)
}

func probeURL(url string) string {
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return "FAIL"
	}
	resp.Body.Close()
	return fmt.Sprintf("%d", resp.StatusCode)
}
