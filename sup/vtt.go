package sup

import (
	"fmt"
	"io"
	"regexp"
	"time"

	"golang.org/x/text/encoding/unicode"
)

type VTTSubtitle struct {
	Start VTTTimestamp
	End   VTTTimestamp
	Text  string
}

type VTTSubtitles []VTTSubtitle

func (subtitles VTTSubtitles) Marshal(output io.Writer) (err error) {
	encoder := unicode.UTF8BOM.NewEncoder().Writer(output)
	// Write WebVTT header
	if _, err = encoder.Write([]byte("WEBVTT\n\n")); err != nil {
		return
	}
	for _, sub := range subtitles {
		// Num
		//if _, err = encoder.Write(fmt.Appendf(nil, "%d\n", i+1)); err != nil {
		//	return
		//}
		// Timestamp
		if _, err = encoder.Write(fmt.Appendf(nil, "%s --> %s\n", sub.Start, sub.End)); err != nil {
			return
		}
		// Text
		if _, err = encoder.Write(fmt.Appendf(nil, "%s\n", sanitizeNewLines(sub.Text))); err != nil {
			return
		}
		// Blank line
		if _, err = encoder.Write([]byte("\n")); err != nil {
			return
		}
	}
	return nil
}

// This regular expression matches two or more consecutive newline characters.
// It handles both \n (Unix-style) and \r\n (Windows-style) newlines.
var newLineRegex = regexp.MustCompile(`(\r?\n){2,}`)

func sanitizeNewLines(s string) string {
	return newLineRegex.ReplaceAllString(s, "\n")
}

type VTTTimestamp time.Duration

func (t VTTTimestamp) String() string {
	if time.Duration(t) >= time.Hour {
		return fmt.Sprintf("%02d:%02d:%02d.%03d",
			time.Duration(t)/time.Hour,
			(time.Duration(t)/time.Minute)%60,
			(time.Duration(t)/time.Second)%60,
			time.Duration(t)%time.Second/time.Millisecond,
		)
	} else {
		return fmt.Sprintf("%02d:%02d.%03d",
			(time.Duration(t)/time.Minute)%60,
			(time.Duration(t)/time.Second)%60,
			time.Duration(t)%time.Second/time.Millisecond,
		)
	}
}
