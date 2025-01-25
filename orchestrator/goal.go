package main

type GoalI interface {
	ID() GoalID
	GoalStatement() string
	// Returns a setup script that will be run on the branch
	SetupOnBranch(BranchName) CompilationTask
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
}
