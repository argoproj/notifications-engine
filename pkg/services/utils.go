package services

import (
	"regexp"
	"strings"
	"unicode/utf8"

	giturls "github.com/whilp/git-urls"

	"github.com/argoproj/notifications-engine/pkg/util/text"
)

var (
	gitSuffix = regexp.MustCompile(`\.git$`)
)

func trunc(message string, n int) string {
	if utf8.RuneCountInString(message) > n {
		return string([]rune(message)[0:n-3]) + "..."
	}
	return message
}

func fullNameByRepoURL(rawURL string) string {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	path := gitSuffix.ReplaceAllString(parsed.Path, "")
	pathParts := text.SplitRemoveEmpty(path, "/")
	return strings.Join(pathParts, "/")
}
