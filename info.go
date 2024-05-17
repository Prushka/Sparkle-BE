package main

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

type Job struct {
	Id            string
	FileRawPath   string
	FileRawFolder string
	FileRawName   string
	FileRawExt    string
	EncodedExt    string
	Input         string
	OutputPath    string
	State         string
	SHA256        string
	EncodedCodecs []string
	Subtitles     map[int]*Pair[Subtitle]
	Videos        map[int]*Pair[Video]
	Audios        map[int]*Pair[Audio]
	Duration      float64
	Width         int
	Height        int
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
	Stream
}

var ValidExtensions = []string{"mkv", "mp4", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "ts", "vob", "3gp", "3g2"}
