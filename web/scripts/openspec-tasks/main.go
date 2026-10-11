// Command openspec-tasks authors task state from explicit task definitions and
// a successful repository-generated integration evidence summary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type values []string

func (v *values) String() string         { return strings.Join(*v, ",") }
func (v *values) Set(value string) error { *v = append(*v, value); return nil }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	change := flag.String("change", "", "Existing OpenSpec change")
	evidence := flag.String("evidence", "", "Passed integration summary.json")
	var tasks, done values
	flag.Var(&tasks, "task", "Task ID and description")
	flag.Var(&done, "done", "Completed task ID")
	flag.Parse()
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]+$`).MatchString(*change) {
		return fmt.Errorf("valid --change required")
	}
	dir := filepath.Join("openspec", "changes", *change)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("create the change with openspec new change first")
	}
	path := filepath.Join(dir, "tasks.md")
	states := map[string]bool{}
	titles := map[string]string{}
	pattern := regexp.MustCompile(`^- \[([ x])\] ([0-9]+\.[0-9]+) (.+)$`)
	if existing, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			if match := pattern.FindStringSubmatch(line); match != nil {
				states[match[2]] = match[1] == "x"
				titles[match[2]] = match[3]
			}
		}
	}
	for _, task := range tasks {
		id, title, ok := strings.Cut(task, " ")
		if !ok || !regexp.MustCompile(`^[0-9]+\.[0-9]+$`).MatchString(id) || strings.ContainsAny(title, "\r\n") {
			return fmt.Errorf("task must contain ID and one-line description")
		}
		titles[id] = title
	}
	if len(titles) == 0 {
		return fmt.Errorf("at least one --task required")
	}
	if len(done) > 0 {
		clean := filepath.Clean(*evidence)
		if !strings.HasPrefix(clean, filepath.Join("temp", "integration-test-runs")+string(os.PathSeparator)) || filepath.Base(clean) != "summary.json" {
			return fmt.Errorf("completion requires project integration evidence")
		}
		body, err := os.ReadFile(clean)
		if err != nil {
			return err
		}
		var summary struct {
			Status   string `json:"status"`
			ExitCode int    `json:"exit_code"`
		}
		if json.Unmarshal(body, &summary) != nil || summary.Status != "passed" || summary.ExitCode != 0 {
			return fmt.Errorf("evidence must report passed with exit code zero")
		}
	}
	for _, id := range done {
		if _, ok := titles[id]; !ok {
			return fmt.Errorf("unknown task %s", id)
		}
		states[id] = true
	}
	ids := make([]string, 0, len(titles))
	for id := range titles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var body strings.Builder
	group := ""
	for _, id := range ids {
		number, _, _ := strings.Cut(id, ".")
		if number != group {
			fmt.Fprintf(&body, "\n## %s. 实施与验证\n\n", number)
			group = number
		}
		marker := " "
		if states[id] {
			marker = "x"
		}
		fmt.Fprintf(&body, "- [%s] %s %s\n", marker, id, titles[id])
	}
	if *evidence != "" {
		fmt.Fprintf(&body, "\n验证证据：`%s`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。\n", filepath.ToSlash(*evidence))
	}
	file, err := os.CreateTemp(dir, ".tasks-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.WriteString(strings.TrimSpace(body.String()) + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "success", "change": *change, "task_count": len(ids)})
}
