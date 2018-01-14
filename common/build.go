package common

// BuildConfig represents the build options/arguments.
type BuildConfig struct {
	Verbose bool
	LLVMIR  bool
	Exe     string
}
