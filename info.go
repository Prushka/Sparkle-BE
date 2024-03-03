package main

const (
	Complete         = "complete"
	Incomplete       = "incomplete"
	StreamsExtracted = "streams_extracted"
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

type Job struct {
	Id            string
	FileRawPath   string
	FileRawFolder string
	FileRawName   string
	FileRawExt    string
	Input         string
	OutputPath    string
	State         string
	SHA256        string
	AudioStreams  []string
	Subtitles     []string
}

var ValidExtensions = []string{"mkv", "mp4", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "ts", "vob", "3gp", "3g2"}
