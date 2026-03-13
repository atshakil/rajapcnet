package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"nvr/internal/client"
	"nvr/internal/model"
)

func usersUsage() {
	fmt.Fprintf(os.Stderr, `Usage: nvrctl users <action>

Actions:
  list, ls                              List all users
  add <username> <password> [role]      Add a user (role: admin|viewer, default: viewer)
  get <id>                              Show user details
  update <id> <json>                    Update user (JSON body)
  delete, rm <id>                       Delete a user
`)
	os.Exit(1)
}

func cmdUsersList(c *client.Client) error {
	users, err := c.ListUsers()
	if err != nil {
		return err
	}
	if len(users) == 0 {
		fmt.Println("No users configured.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUSERNAME\tROLE\tENABLED")
	for _, u := range users {
		fmt.Fprintf(w, "%d\t%s\t%s\t%v\n", u.ID, u.Username, u.Role, u.Enabled)
	}
	return w.Flush()
}

func cmdUsersAdd(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl users add <username> <password> [role]")
	}
	role := model.RoleViewer
	if len(os.Args) >= 6 {
		role = model.Role(os.Args[5])
	}
	u := &model.User{
		Username: os.Args[3],
		Password: os.Args[4],
		Role:     role,
	}
	result, err := c.AddUser(u)
	if err != nil {
		return err
	}
	fmt.Printf("Added user %d: %s (role: %s)\n", result.ID, result.Username, result.Role)
	return nil
}

func cmdUsersGet(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: nvrctl users get <id>")
	}
	u, err := c.GetUser(os.Args[3])
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(u)
}

func cmdUsersUpdate(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl users update <id> <json>")
	}
	var u model.User
	if err := json.Unmarshal([]byte(os.Args[4]), &u); err != nil {
		return fmt.Errorf("invalid json: %v", err)
	}
	if err := c.UpdateUser(os.Args[3], &u); err != nil {
		return err
	}
	fmt.Println("Updated.")
	return nil
}

func cmdUsersDelete(c *client.Client) error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: nvrctl users delete <id>")
	}
	if err := c.DeleteUser(os.Args[3]); err != nil {
		return err
	}
	fmt.Println("Deleted.")
	return nil
}
