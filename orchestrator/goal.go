package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

type GoalI interface {
	ID() GoalID
	// not guaranteed to be unique (may be empty)
	Name() string
	GoalStatement() string
	// Returns a setup script that will be run on the branch
	SetupOnBranch(src BranchName, target BranchName) CompilationTask
	// Returns true if the setup script was successful
	// & if the branch is ready to be explored
	//
	// If this returns false, the branch should not be scheduled.
	ValidateSetup(CompilationTaskResponse) bool
	// Total number of attempts within the repo
	MaxAttempts() int
}

type GoalProvider interface {
	GetGoal(GoalID) GoalI
	GetAll() []GoalI
	GetRandom() GoalI
	GetNext() GoalI
}

type StaticGoalProvider struct {
	index     int
	goals     map[GoalID]GoalI
	goalOrder []GoalID
}

func (g *StaticGoalProvider) GetGoal(id GoalID) GoalI {
	return g.goals[id]
}

func (g *StaticGoalProvider) GetAll() []GoalI {
	all := []GoalI{}
	for _, goal := range g.goals {
		all = append(all, goal)
	}
	return all
}

func (g *StaticGoalProvider) GetRandom() GoalI {
	// If this causes perf issues, consider switching to a different datastructure.
	keys := make([]GoalID, 0, len(g.goals))
	for gi := range g.goals {
		keys = append(keys, gi)
	}

	return g.goals[keys[rand.IntN(len(keys))]]
}

func (g *StaticGoalProvider) GetNext() GoalI {
	goal := g.goals[g.goalOrder[g.index]]
	g.index = (g.index + 1) % len(g.goals)
	return goal
}

type GoalAddExample struct {
	Name_   string `json:"name"`
	ID_     GoalID `json:"id"`
	Example string `json:"example"`
}

func (g *GoalAddExample) ID() GoalID {
	return g.ID_
}

func (g *GoalAddExample) Name() string {
	return g.Name_
}

func (g *GoalAddExample) GoalStatement() string {
	return "Fix the compilation errors that arose from adding an example to the repo."
}

func (g *GoalAddExample) SetupOnBranch(src BranchName, target BranchName) CompilationTask {
	return CompilationTask{
		BranchName:    src,
		NewBranchName: target,
		PreCommands: []CompilationPreCommand{
			{
				Name:   "add-example",
				Script: fmt.Sprintf("cat << 'EOF' >> Test.lean\n%s\nEOF", g.Example),
			},
			{
				// Shouldn't be needed... but just in case
				Name:   "mk_all-hidden",
				Script: "lake exec mk_all --lib Corelib",
			},
			{
				Name:   "prebuild-hidden",
				Script: "lake build",
			},
		},
		CompilationScript: "lake build",
	}
}

func (g *GoalAddExample) ValidateSetup(response CompilationTaskResponse) bool {
	// already-working branch should not get more examples
	if response.CompilationResult.ExitCode == 0 {
		return false
	}
	lines := strings.Split(response.CompilationResult.Out, "\n")
	// TODO: parse lean output correctly
	for _, line := range lines {
		if strings.Contains(line, "term has type") {
			return false
		}
	}
	return true
}

func (g *GoalAddExample) MaxAttempts() int {
	return 5
}

type GoalFile struct {
	AddExampleGoals []GoalAddExample `json:"add_example_goals"`
}

func (gf *GoalFile) SaveToFile(path string) {
	js, err := json.Marshal(gf)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(path, js, 0644)
	if err != nil {
		panic(err)
	}
}

func GoalFileFromPath(path string) *GoalFile {
	bytes, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	gf := &GoalFile{}
	err = json.Unmarshal(bytes, gf)
	if err != nil {
		panic(err)
	}
	return gf
}

/*
* GoalFileFromLeanSrcPath reads a Lean source file, and splits every goal by
* --N: {name}
* some lean code
* --N: {name}
* some lean code
* ...
 */
func GoalFileFromLeanSrcPath(leanSrcPath string) *GoalFile {
	leanSrc, err := os.ReadFile(leanSrcPath)
	if err != nil {
		panic(err)
	}
	examples := []GoalAddExample{}
	lines := strings.Split(string(leanSrc), "\n")
	var currentExample *GoalAddExample
	for _, line := range lines {
		if strings.HasPrefix(line, "--N:") {
			if currentExample != nil {
				examples = append(examples, *currentExample)
			}
			currentExample = &GoalAddExample{
				ID_:   NewGoalID(),
				Name_: strings.TrimSpace(line[len("--N:"):]),
			}
		} else if currentExample != nil {
			currentExample.Example += line + "\n"
		}
	}
	if currentExample != nil {
		examples = append(examples, *currentExample)
	}
	for i := range examples {
		examples[i].Example = strings.TrimSpace(examples[i].Example)
	}
	return &GoalFile{
		AddExampleGoals: examples,
	}
}

func StaticGoalProviderFromFile(path string) GoalProvider {
	gf := GoalFileFromPath(path)
	goals := map[GoalID]GoalI{}
	goalOrder := []GoalID{}
	for _, goal := range gf.AddExampleGoals {
		goals[goal.ID()] = &goal
		goalOrder = append(goalOrder, goal.ID())
	}
	return &StaticGoalProvider{
		goals:     goals,
		goalOrder: goalOrder,
		index:     0,
	}
}

func createGoalFileCli() *cli.Command {
	var leanSrcPath string
	var path string
	action := func(ctx context.Context, _ *cli.Command) error {
		gf := GoalFileFromLeanSrcPath(leanSrcPath)
		gf.SaveToFile(path)
		return nil
	}
	return &cli.Command{
		Name:   "goal-file",
		Usage:  "create a goal file",
		Action: action,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "examples-src",
				Usage:       "path to the examples lean source file",
				Destination: &leanSrcPath,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "out",
				Usage:       "path to save the goal file",
				Destination: &path,
				Required:    true,
			},
		},
	}
}
