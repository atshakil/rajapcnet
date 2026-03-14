package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/joho/godotenv"

	"nvr/internal/client"
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

	// Load cached token for authenticated commands
	if token := loadToken(); token != "" {
		c.SetToken(token)
	}

	if len(os.Args) < 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "health":
		err = cmdHealth(c)
	case "login":
		err = cmdLogin(c)
	case "logout":
		err = cmdLogout()
	case "bootstrap":
		err = cmdBootstrap(c)
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
	case "users":
		if len(os.Args) < 3 {
			usersUsage()
		}
		switch os.Args[2] {
		case "list", "ls":
			err = cmdUsersList(c)
		case "add":
			err = cmdUsersAdd(c)
		case "get":
			err = cmdUsersGet(c)
		case "update":
			err = cmdUsersUpdate(c)
		case "delete", "rm":
			err = cmdUsersDelete(c)
		default:
			usersUsage()
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
  login <u> <p>      Authenticate and cache token
  logout             Remove cached token
  bootstrap <u> <p>  Create first admin user (one-time)
  cameras <action>   Manage cameras
  users <action>     Manage users (admin)

Environment:
  NVR_URL   NVR server URL (default: http://localhost:8080)
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
