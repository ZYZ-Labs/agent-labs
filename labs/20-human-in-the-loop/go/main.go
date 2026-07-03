package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const stateFile = "workflow_state.json"

type destructiveAction struct {
	Target string
	Action string
}

func (a *destructiveAction) describe() string {
	return fmt.Sprintf("%s %s", a.Action, a.Target)
}

type humanInTheLoopWorkflow struct {
	stateFile string
	state     map[string]interface{}
}

func newHumanInTheLoopWorkflow(stateFile string) *humanInTheLoopWorkflow {
	w := &humanInTheLoopWorkflow{stateFile: stateFile}
	w.state = w.load()
	return w
}

func (w *humanInTheLoopWorkflow) load() map[string]interface{} {
	b, err := os.ReadFile(w.stateFile)
	if err != nil {
		return map[string]interface{}{}
	}
	var state map[string]interface{}
	_ = json.Unmarshal(b, &state)
	return state
}

func (w *humanInTheLoopWorkflow) save() error {
	b, err := json.MarshalIndent(w.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := w.stateFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, w.stateFile)
}

func (w *humanInTheLoopWorkflow) reset() {
	_ = os.Remove(w.stateFile)
	w.state = map[string]interface{}{}
}

func (w *humanInTheLoopWorkflow) requestApproval(action *destructiveAction) {
	stage, _ := w.state["stage"].(string)
	if stage == "approved" || stage == "executed" || stage == "rejected" {
		return
	}
	fmt.Printf("\nDestructive action requested: %s\n", action.describe())
	fmt.Println("This action cannot be undone. Please review carefully.")
	w.state = map[string]interface{}{
		"stage":    "awaiting_approval",
		"target":   action.Target,
		"action":   action.Action,
		"approved": nil,
	}
	_ = w.save()
}

func (w *humanInTheLoopWorkflow) collectDecision() bool {
	stage, _ := w.state["stage"].(string)
	if stage != "awaiting_approval" {
		approved, _ := w.state["approved"].(bool)
		return approved
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Approve? [y/n]: ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			w.state["approved"] = true
			w.state["stage"] = "approved"
			_ = w.save()
			return true
		}
		if answer == "n" || answer == "no" {
			w.state["approved"] = false
			w.state["stage"] = "rejected"
			_ = w.save()
			return false
		}
		fmt.Println("Please answer 'y' or 'n'.")
	}
}

func (w *humanInTheLoopWorkflow) execute(action *destructiveAction) {
	stage, _ := w.state["stage"].(string)
	if stage == "executed" {
		fmt.Println("Action already executed.")
		return
	}
	approved, _ := w.state["approved"].(bool)
	if !approved {
		fmt.Println("Action not approved; will not execute.")
		return
	}
	fmt.Printf("\nExecuting: %s\n", action.describe())
	fmt.Printf("  -> %s has been %sd.\n", action.Target, action.Action)
	w.state["stage"] = "executed"
	w.state["executed_at"] = time.Now().Format(time.RFC3339)
	_ = w.save()
}

func (w *humanInTheLoopWorkflow) run(action *destructiveAction) {
	w.requestApproval(action)
	approved := w.collectDecision()
	if approved {
		w.execute(action)
	} else {
		fmt.Println("\nAction rejected by human operator.")
	}
}

func main() {
	reset := flag.Bool("reset", false, "Clear saved workflow state")
	flag.Parse()

	workflow := newHumanInTheLoopWorkflow(stateFile)
	if *reset {
		workflow.reset()
		fmt.Println("Workflow state reset.")
		return
	}

	action := &destructiveAction{Target: "user_account_42", Action: "delete"}
	workflow.run(action)
}
