package pttbbs

import (
	"regexp"
)

var (
	ArticleFirstLineRegexp = regexp.MustCompile(`^(.+?): (.+) (.+?): (.+?)\n$`)
	ArticleMetaLineRegexp  = regexp.MustCompile(`^(.+?): (.+)\n$`)
)

const (
	ArticleAuthor = "作者"
	ArticleTitle  = "標題"
)

var validBrdNameRegexp *regexp.Regexp

func IsValidBrdName(brdname string) bool {
	if validBrdNameRegexp == nil {
		validBrdNameRegexp = regexp.MustCompile("^[a-zA-Z0-9_\\-]+$")
	}
	return validBrdNameRegexp.MatchString(brdname)
}

func ParseArticleFirstLine(line []byte) (tag1, val1, tag2, val2 []byte, ok bool) {
	m := ArticleFirstLineRegexp.FindSubmatch(line)
	if m == nil {
		ok = false
	} else {
		tag1, val1, tag2, val2 = m[1], m[2], m[3], m[4]
		ok = true
	}
	return
}

func ParseArticleMetaLine(line []byte) (tag, val []byte, ok bool) {
	m := ArticleMetaLineRegexp.FindSubmatch(line)
	if m == nil {
		ok = false
	} else {
		tag, val = m[1], m[2]
		ok = true
	}
	return
}
