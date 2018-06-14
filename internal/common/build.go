package common

// BuildConfig represents the build options/arguments.
type BuildConfig struct {
	Verbose bool
	LLVMIR  bool
	Exe     string
}

func NewBuildConfig() *BuildConfig {
	config := &BuildConfig{}
	config.Exe = ""
	return config
}
