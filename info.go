package main

import (
	"fmt"
	"path/filepath"
)

const (
	Complete         = "complete"
	Incomplete       = "incomplete"
	StreamsExtracted = "streams_extracted"
	JobFile          = "job.json"
	ThumbnailVtt     = "storyboard.vtt"
	SpritePrefix     = "sp"
	SpriteExtension  = ".jpg"
)

// StreamInfo holds information about a stream in a media file
type StreamInfo struct {
	Index     int    `json:"index"`
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Channels  int    `json:"channels,omitempty"` // Ensure this matches the JSON structure
	Tags      struct {
		Language string `json:"language"`
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
	FileRawPath       string
	FileRawFolder     string
	FileRawName       string
	FileRawExt        string
	EncodedExt        string
	Input             string
	OutputPath        string
	State             string
	SHA256            string
	EncodedCodecs     []string
	MappedAudio       map[string]map[int]*Pair[Audio]
	EncodedCodecsSize map[string]int64
	Subtitles         map[int]*Pair[Subtitle]
	Videos            map[int]*Pair[Video]
	Audios            map[int]*Pair[Audio]
	Duration          float64
	Width             int
	Height            int
}

func (j *Job) GetCodecVideo(codec string) string {
	return filepath.Join(j.OutputPath, fmt.Sprintf("%s.%s", codec, TheConfig.VideoExt))
}

type Pair[T any] struct {
	Raw *T
	Enc *T
}

type Stream struct {
	Bitrate   int
	CodecName string
	Index     int
	Location  string
}

type Subtitle struct {
	Language string
	Stream
}

type Video struct {
	Width     int
	Height    int
	Framerate string
	Stream
}

type Audio struct {
	Channels   int
	SampleRate int
	Language   string
	Stream
}

var ValidExtensions = []string{"mkv", "mp4", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "ts", "vob", "3gp", "3g2"}
