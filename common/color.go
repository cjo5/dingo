package common

import (
	"fmt"
	"os"
)

// ANSI color escape sequences.
var (
	BoldText   = ""
	RedText    = ""
	GreenText  = ""
	YellowText = ""
	ResetText  = ""
)

func init() {
	// http://no-color.org
	if _, isset := os.LookupEnv("NO_COLOR"); isset {
		return
	}

	BoldText = "\x1B[01m"
	RedText = "\x1B[31m"
	GreenText = "\x1B[32m"
	YellowText = "\x1B[33m"
	ResetText = "\x1B[0m"
}

func BoldRed(s string) string {
	return fmt.Sprintf("%s%s%s%s", BoldText, RedText, s, ResetText)
}

func Red(s string) string {
	return fmt.Sprintf("%s%s%s", RedText, s, ResetText)
}

func BoldGreen(s string) string {
	return fmt.Sprintf("%s%s%s%s", BoldText, GreenText, s, ResetText)
}

func Green(s string) string {
	return fmt.Sprintf("%s%s%s", GreenText, s, ResetText)
}

func BoldYellow(s string) string {
	return fmt.Sprintf("%s%s%s%s", BoldText, YellowText, s, ResetText)
}

func Yellow(s string) string {
	return fmt.Sprintf("%s%s%s", YellowText, s, ResetText)
}
