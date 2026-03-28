package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"nvr/internal/client"
	"nvr/internal/model"
)

func motionLogUsage() {
	fmt.Fprintf(os.Stderr, `Usage: nvrctl cameras motion-log <action> [args]

Actions:
  enable <camera-id>                       Enable motion logging
  disable <camera-id>                      Disable motion logging
  retention <camera-id> <days>             Set retention days
  status [camera-id]                       Show motion logging status
  ls <camera-id> [--from T] [--to T] [--status S] [--limit N]
                                           List motion events
  stream [camera-id]                       Stream live motion events
  export <camera-id> [--from T] [--to T] [--format csv|jsonl]
                                           Export motion events
`)
	os.Exit(1)
}

func cmdMotionLog(c *client.Client) error {
	if len(os.Args) < 4 {
		motionLogUsage()
	}
	switch os.Args[3] {
	case "enable":
		return cmdMotionEnable(c, true)
	case "disable":
		return cmdMotionEnable(c, false)
	case "retention":
		return cmdMotionRetention(c)
	case "status":
		return cmdMotionStatus(c)
	case "ls", "list":
		return cmdMotionList(c)
	case "stream":
		return cmdMotionStream(c)
	case "export":
		return cmdMotionExport(c)
	default:
		motionLogUsage()
	}
	return nil
}

func cmdMotionEnable(c *client.Client, enable bool) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl cameras motion-log %s <camera-id>", map[bool]string{true: "enable", false: "disable"}[enable])
	}
	id := os.Args[4]
	ms, err := c.UpdateMotionSettings(id, map[string]any{"enabled": enable})
	if err != nil {
		return err
	}
	state := "disabled"
	if ms.Enabled {
		state = "enabled"
	}
	fmt.Printf("Motion logging %s for camera %s (retention: %d days, runtime: %s)\n",
		state, id, ms.RetentionDays, ms.RuntimeState)
	return nil
}

func cmdMotionRetention(c *client.Client) error {
	if len(os.Args) < 6 {
		return fmt.Errorf("usage: nvrctl cameras motion-log retention <camera-id> <days>")
	}
	id := os.Args[4]
	days, err := strconv.Atoi(os.Args[5])
	if err != nil || days < 1 {
		return fmt.Errorf("days must be a positive integer")
	}
	ms, err := c.UpdateMotionSettings(id, map[string]any{"retention_days": days})
	if err != nil {
		return err
	}
	fmt.Printf("Retention set to %d days for camera %s\n", ms.RetentionDays, id)
	return nil
}

func cmdMotionStatus(c *client.Client) error {
	// Optional camera ID argument
	if len(os.Args) >= 5 {
		id := os.Args[4]
		ms, err := c.GetMotionSettings(id)
		if err != nil {
			return err
		}
		fmt.Printf("Camera %s:\n", id)
		fmt.Printf("  Enabled:        %v\n", ms.Enabled)
		fmt.Printf("  Retention:      %d days\n", ms.RetentionDays)
		fmt.Printf("  Runtime state:  %s\n", ms.RuntimeState)
		return nil
	}

	// All cameras
	statuses, err := c.MotionLogStatus()
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		fmt.Println("No cameras have motion logging configured.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CAMERA\tENABLED\tRETENTION\tRUNTIME")
	for _, s := range statuses {
		fmt.Fprintf(w, "%d\t%v\t%d days\t%s\n", s.CameraID, s.Enabled, s.RetentionDays, s.RuntimeState)
	}
	return w.Flush()
}

func cmdMotionList(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl cameras motion-log ls <camera-id> [flags]")
	}
	id := os.Args[4]
	query := buildMotionQuery(os.Args[5:])

	page, err := c.ListMotionEvents(id, query)
	if err != nil {
		return err
	}
	if len(page.Episodes) == 0 {
		fmt.Println("No motion events found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTARTED\tDURATION\tSTATUS\tEVENTS\tTOPIC")
	for _, ep := range page.Episodes {
		dur := "—"
		if ep.DurationMs != nil {
			dur = fmtDuration(*ep.DurationMs)
		}
		started := time.UnixMilli(ep.StartedAtMs).Format("2006-01-02 15:04:05")
		topic := ep.Topic
		if len(topic) > 40 {
			topic = "..." + topic[len(topic)-37:]
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n",
			ep.ID, started, dur, ep.Status, ep.EventCount, topic)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if page.NextCursor != "" {
		fmt.Printf("\n  Next page: --cursor %s\n", page.NextCursor)
	}
	return nil
}

func cmdMotionStream(c *client.Client) error {
	path := "/api/motion-log/stream"
	if len(os.Args) >= 5 {
		path = "/api/cameras/" + os.Args[4] + "/motion-log/stream"
	}
	resp, err := c.StreamMotionLog(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var evt model.MotionEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			ts := time.UnixMilli(evt.TimestampMs).Format("15:04:05")
			name := evt.CameraName
			if name == "" {
				name = fmt.Sprintf("cam%d", evt.CameraID)
			}
			switch eventType {
			case "motion.start":
				fmt.Printf("[%s] ● %s — motion started (episode %d)\n", ts, name, evt.EpisodeID)
			case "motion.end":
				dur := ""
				if evt.DurationMs != nil {
					dur = " (" + fmtDuration(*evt.DurationMs) + ")"
				}
				fmt.Printf("[%s] ○ %s — motion ended%s\n", ts, name, dur)
			case "motion.update":
				fmt.Printf("[%s] · %s — motion update (count: %d)\n", ts, name, evt.EventCount)
			case "motion.runtime":
				fmt.Printf("[%s] ◆ %s — runtime: %s\n", ts, name, evt.State)
			case "motion.error":
				fmt.Printf("[%s] ✗ %s — error: %s\n", ts, name, evt.Error)
			}
			eventType = ""
		}
	}
	return scanner.Err()
}

func cmdMotionExport(c *client.Client) error {
	if len(os.Args) < 5 {
		return fmt.Errorf("usage: nvrctl cameras motion-log export <camera-id> [--from T] [--to T] [--format csv|jsonl]")
	}
	id := os.Args[4]

	flags := os.Args[5:]
	format := "csv"
	for i, f := range flags {
		if f == "--format" && i+1 < len(flags) {
			format = flags[i+1]
		}
	}
	query := buildMotionQuery(flags)
	// Use large limit for export
	if !strings.Contains(query, "limit=") {
		if query != "" {
			query += "&"
		}
		query += "limit=200"
	}

	var csvW *csv.Writer
	if format == "csv" {
		csvW = csv.NewWriter(os.Stdout)
		csvW.Write([]string{"id", "camera_id", "source", "started_at", "ended_at", "duration_ms", "status", "close_reason", "event_count", "topic", "rule_name"})
	}

	cursor := ""
	for {
		q := query
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		page, err := c.ListMotionEvents(id, q)
		if err != nil {
			return err
		}
		for _, ep := range page.Episodes {
			switch format {
			case "jsonl":
				data, _ := json.Marshal(ep)
				os.Stdout.Write(data)
				os.Stdout.Write([]byte("\n"))
			default:
				ended := ""
				if ep.EndedAtMs != nil {
					ended = time.UnixMilli(*ep.EndedAtMs).UTC().Format(time.RFC3339)
				}
				dur := ""
				if ep.DurationMs != nil {
					dur = strconv.FormatInt(*ep.DurationMs, 10)
				}
				csvW.Write([]string{
					strconv.FormatInt(ep.ID, 10),
					strconv.FormatInt(ep.CameraID, 10),
					ep.Source,
					time.UnixMilli(ep.StartedAtMs).UTC().Format(time.RFC3339),
					ended,
					dur,
					ep.Status,
					ep.CloseReason,
					strconv.Itoa(ep.EventCount),
					ep.Topic,
					ep.RuleName,
				})
			}
		}
		if csvW != nil {
			csvW.Flush()
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers

func buildMotionQuery(flags []string) string {
	params := url.Values{}
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case "--from":
			if i+1 < len(flags) {
				i++
				params.Set("from", parseTimeFlag(flags[i]))
			}
		case "--to":
			if i+1 < len(flags) {
				i++
				params.Set("to", parseTimeFlag(flags[i]))
			}
		case "--status":
			if i+1 < len(flags) {
				i++
				params.Set("status", flags[i])
			}
		case "--limit":
			if i+1 < len(flags) {
				i++
				params.Set("limit", flags[i])
			}
		case "--cursor":
			if i+1 < len(flags) {
				i++
				params.Set("cursor", flags[i])
			}
		}
	}
	return params.Encode()
}

func parseTimeFlag(s string) string {
	// Accept epoch milliseconds directly
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return s
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return strconv.FormatInt(t.UnixMilli(), 10)
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return strconv.FormatInt(t.UnixMilli(), 10)
	}
	return s
}

func fmtDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	s := ms / 1000
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	return fmt.Sprintf("%dm%ds", m, s%60)
}
