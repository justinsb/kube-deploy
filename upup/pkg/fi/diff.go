package fi

import "github.com/golang/glog"

// StringDiff allows for a user-friendly difference between strings
type StringDiff struct {
	CommonPrefix string
	Left         string
	Right        string
}

func FindFirstDiff(l, r string) StringDiff {
	min := len(l)
	if len(r) < min {
		min = len(r)
	}
	// Find common prefix
	i := 0
	for ; i < min; i++ {
		if l[i] != r[i] {
			break
		}
	}

	var diff StringDiff
	diff.CommonPrefix = l[:i]

	glog.Infof("DIFF L %q", l[i:])
	glog.Infof("DIFF R %q", r[i:])
	j := i + 20
	for ; j < min; j++ {
		if l[j] == r[j] {
			diff.Left = l[i:j]
			diff.Right = r[i:j]
			return diff
		}
	}

	diff.Left = l[i:]
	diff.Right = r[i:]
	return diff
}

func LimitedSuffix(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[len(s)-n:] + "..."
}

func LimitedPrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
