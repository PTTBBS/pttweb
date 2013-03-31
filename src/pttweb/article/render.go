package article

import (
	"ansi"
	"bytes"
	"pttbbs"
)

const (
	kPreviewContentLines = 5
)

type Renderer struct {
	buf    bytes.Buffer
	lineNo int

	mapper    *IndexMapper
	lineBuf   bytes.Buffer
	lineSegs  []Segment
	segIndex  int
	segOffset int
	segClosed bool

	acceptMetaLines bool

	title string

	previewContent   string
	previewLineCount int
}

func NewRenderer() *Renderer {
	ar := &Renderer{
		mapper:   NewIndexMapper(2),
		lineSegs: make([]Segment, 0, 8),
	}
	ar.Reset()
	return ar
}

func (r *Renderer) Reset() {
	r.buf.Reset()
	r.lineNo = 1

	r.mapper.Reset()
	r.lineBuf.Reset()
	r.lineSegs = r.lineSegs[0:0]
	r.segIndex = 0
	r.segOffset = 0
	r.segClosed = true

	r.acceptMetaLines = true

	r.title = ""

	r.previewContent = ""
	r.previewLineCount = 0
}

func (r *Renderer) ParsedTitle() string {
	return r.title
}

func (r *Renderer) PreviewContent() string {
	return r.previewContent
}

func (r *Renderer) Render(content []byte) (*bytes.Buffer, error) {
	converter := &ansi.Ansi2Html{
		StartColor: func(fg, bg, flags int) {
			r.startColor(fg, bg, flags)
		},
		EndColor: func() {
			r.endColor()
		},
		Rune: func(ru rune) {
			r.oneRune(ru)
		},
		EndOfLine: func() {
			r.endOfLine()
		},
	}
	if err := converter.ConvertFromUTF8(content); err != nil {
		return nil, err
	}
	return &r.buf, nil
}

func (r *Renderer) currSeg() *Segment {
	if len(r.lineSegs) == 0 || r.segClosed {
		r.startColor(ansi.DefaultFg, ansi.DefaultBg, ansi.NoFlags)
	}
	return &r.lineSegs[len(r.lineSegs)-1]
}

func (r *Renderer) startColor(fg, bg, flags int) {
	r.lineSegs = append(r.lineSegs, Segment{
		fg:     fg,
		bg:     bg,
		flags:  flags,
		Tag:    "span",
		Buffer: &bytes.Buffer{},
	})
	r.segClosed = false
}

func (r *Renderer) endColor() {
	r.segClosed = true
}

func (r *Renderer) oneRune(ru rune) {
	seg := r.currSeg()
	r.mapper.Record(r.lineBuf.Len(), len(r.lineSegs)-1, seg.Len())
	fastWriteHtmlEscapedRune(seg.Buffer, ru)
	r.lineBuf.WriteRune(ru)
}

func (r *Renderer) outputToSegment(i, off int) {
	for ; r.segIndex < i; r.segIndex++ {
		s := &r.lineSegs[r.segIndex]
		if r.segOffset > 0 {
			// half output
			if r.segClosed {
				s.WriteOpen(&r.buf)
			}
			r.buf.Write(s.InnerBytes()[r.segOffset:])
			s.WriteClose(&r.buf)
			r.segOffset = 0
			r.segClosed = true
		} else {
			if !r.segClosed {
				panic("Segment not closed at beginning of a segment is not allowed")
			}
			s.WriteTo(&r.buf)
			r.segClosed = true
		}
	}
	if off > 0 {
		s := &r.lineSegs[r.segIndex]
		if r.segClosed {
			s.WriteOpen(&r.buf)
		}
		r.buf.Write(s.InnerBytes()[r.segOffset:off])
		r.segOffset = off
		r.segClosed = false
	}
}

func (r *Renderer) skipToSegment(i, off int) {
	r.prematureCloseSegment()
	r.segIndex = i
	r.segOffset = off
}

func (r *Renderer) prematureCloseSegment() {
	if !r.segClosed {
		r.lineSegs[r.segIndex].WriteClose(&r.buf)
		r.segClosed = true
	}
}

func (r *Renderer) matchFirstLineAndOutput(line []byte) bool {
	tag1, val1, tag2, val2, ok := pttbbs.ParseArticleFirstLine(r.lineBuf.Bytes())
	if !ok {
		return false
	}

	r.writeMetaLine(tag1, val1, "articleMetaLine")
	r.writeMetaLine(tag2, val2, "articleMetaLineRight")
	return true
}

func (r *Renderer) writeMetaLine(tag, val []byte, divClass string) {
	r.buf.WriteString(`<div class="` + divClass + `"><span class="articleMetaTag">`)
	r.buf.Write(tag)
	r.buf.WriteString(`</span>`)
	r.buf.WriteString(`<span class="articleMetaVal">`)
	r.buf.Write(val)
	r.buf.WriteString(`</span></div>`)
}

func (r *Renderer) endOfLine() {
	r.segClosed = true

	line := r.lineBuf.Bytes()
	parsed := false

	if r.acceptMetaLines && r.lineNo < 5 {
		if r.lineNo == 1 && r.matchFirstLineAndOutput(line) {
			parsed = true
		} else if tag, val, ok := pttbbs.ParseArticleMetaLine(line); ok {
			if bytes.Equal(tag, []byte(pttbbs.ArticleTitle)) {
				r.title = string(val)
			}
			r.writeMetaLine(tag, val, "articleMetaLine")
			parsed = true
		} else {
			r.acceptMetaLines = false
		}
	}

	if !parsed {
		if r.previewLineCount < kPreviewContentLines {
			r.previewContent += string(line)
			r.previewLineCount++
		}
		r.processNormalContentLine(line)
	}

	// Reset and update variables
	r.mapper.Reset()
	r.lineBuf.Reset()
	r.lineSegs = r.lineSegs[0:0]
	r.segIndex = 0
	r.segOffset = 0
	r.segClosed = true
	r.lineNo++
}

func (r *Renderer) processNormalContentLine(line []byte) {
	// Detect push line
	isPush := false
	if matchPushLine(r.lineSegs) {
		r.lineSegs[0].ExtraFlags |= PushTag
		r.lineSegs[1].ExtraFlags |= PushUserId
		r.lineSegs[2].ExtraFlags |= PushContent
		r.lineSegs[3].ExtraFlags |= PushIpDateTime
		r.buf.WriteString(`<div class="push">`)
		isPush = true
	}

	if urls := urlPattern.FindAllIndex(line, -1); urls != nil {
		for _, u := range urls {
			for i, n := 0, len(u); i < n; i += 2 {
				urlString := string(line[u[i]:u[i+1]])
				linkBegin, linkEnd := makeExternalUrlLink(urlString)

				begin := r.mapper.Get(u[i])
				end := r.mapper.Get(u[i+1])
				r.outputToSegment(begin[0], begin[1])
				if begin[0] == end[0] {
					// same segment: embed
					r.buf.WriteString(linkBegin)
					r.outputToSegment(end[0], end[1])
					r.buf.WriteString(linkEnd)
				} else {
					// different segments: split, wrap-around
					r.prematureCloseSegment()
					r.buf.WriteString(linkBegin)
					r.outputToSegment(end[0], end[1])
					r.prematureCloseSegment()
					r.buf.WriteString(linkEnd)
				}
			}
		}
	}
	r.outputToSegment(len(r.lineSegs), 0)

	if isPush {
		r.buf.WriteString(`</div>`)
	}

	if rcs, err := FindRichContents(r.lineBuf.Bytes()); err == nil {
		for _, rc := range rcs {
			r.buf.WriteString(`<div class="richcontent">` + rc.ContentHtml + `</div>`)
		}
	}
}