package main

import (
	"fmt"
	"os"
	"strings"
)

var splitter = string(os.PathSeparator)

const (
	Complete         = "complete"
	Incomplete       = "incomplete"
	StreamsExtracted = "streams_extracted"
	JobFile          = "job.json"
	ThumbnailVtt     = "storyboard.vtt"
	SpritePrefix     = "sp"
	SpriteExtension  = ".jpg"
	SubtitlesType    = "subtitle"
	AudioType        = "audio"
	AttachmentType   = "attachment"
)

// StreamInfo holds information about a stream in a media file
type StreamInfo struct {
	Index     int    `json:"index"`
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Channels  int    `json:"channels,omitempty"` // Ensure this matches the JSON structure
	Tags      struct {
		Language string `json:"language"`
		Title    string `json:"title"`
		Filename string `json:"filename"`
		MimeType string `json:"mimetype"`
	}
}

// FFProbeOutput holds the structure for ffprobe output
type FFProbeOutput struct {
	Streams []StreamInfo `json:"streams"`
}

var codecMap = map[string]string{
	"hdmv_pgs_subtitle": "sup",
	"subrip":            "srt",
	"webvtt":            "vtt",
}

type Job struct {
	Id                string
	InputParent       string
	Input             string
	State             string
	SHA256            string
	EncodedCodecs     []string
	MappedAudio       map[string][]Stream
	EncodedCodecsSize map[string]int64
	Streams           []Stream
	Duration          float64
	Width             int
	Height            int
	EncodedExt        string
}

type Stream struct {
	Bitrate    int
	CodecName  string
	CodecType  string
	Index      int
	Location   string
	Language   string
	Title      string
	Filename   string
	MimeType   string
	Channels   int
	SampleRate int
}

func (job *Job) InputExt() string {
	sp := strings.Split(job.Input, ".")
	return sp[len(sp)-1]
}

func (job *Job) InputName() string {
	sp := strings.Split(job.Input, ".")
	return strings.Join(sp[:len(sp)-1], ".")
}

func (job *Job) InputAfterRename() string {
	return fmt.Sprintf("%s.%s", job.Id, job.InputExt())
}

func (job *Job) OutputJoin(args ...string) string {
	return OutputJoin(append([]string{job.Id}, args...)...)
}

func (job *Job) InputJoin(args ...string) string {
	return InputJoin(append([]string{job.InputParent}, args...)...)
}

func (job *Job) GetCodecVideo(codec string) string {
	return job.OutputJoin(fmt.Sprintf("%s.%s", codec, TheConfig.VideoExt))
}

var ValidExtensions = []string{"mkv", "mp4", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "ts", "vob", "3gp", "3g2"}
