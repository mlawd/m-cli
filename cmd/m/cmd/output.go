package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
)

func outSuccess(w io.Writer, format string, args ...any) {
	outStyled(w, ansiGreen, "✅", format, args...)
}

func outInfo(w io.Writer, format string, args ...any) {
	outStyled(w, ansiCyan, "ℹ️", format, args...)
}

func outAction(w io.Writer, format string, args ...any) {
	outStyled(w, ansiBlue, "🚀", format, args...)
}

func outWarn(w io.Writer, format string, args ...any) {
	outStyled(w, ansiYellow, "⚠️", format, args...)
}

func outCurrent(w io.Writer, format string, args ...any) {
	outStyled(w, ansiCyan, "➜", format, args...)
}

func outReuse(w io.Writer, format string, args ...any) {
	outStyled(w, ansiBlue, "♻️", format, args...)
}

func outLink(w io.Writer, format string, args ...any) {
	outStyled(w, ansiCyan, "🔗", format, args...)
}

func outStyled(w io.Writer, color, icon, format string, args ...any) {
	line := fmt.Sprintf("%s %s", icon, fmt.Sprintf(format, args...))
	if supportsColor(w) {
		line = color + line + ansiReset
	}
	fmt.Fprintln(w, line)
}

func supportsColor(w io.Writer) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

func FormatCLIError(w io.Writer, err error) string {
	if err == nil {
		return ""
	}

	line := fmt.Sprintf("❌ %s", strings.TrimSpace(err.Error()))
	if supportsColor(w) {
		line = ansiRed + line + ansiReset
	}

	return line
}
