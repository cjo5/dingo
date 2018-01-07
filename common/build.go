package common

// BuildEnvironment represents the build options/arguments.
type BuildEnvironment struct {
	Verbose bool
	LLVMIR  bool
	Exe     string
}
