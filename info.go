package main

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
type Result struct {
	Version      Version      `json:"Version"`
	ProgressList []Progress   `json:"Progress"`
	JSONTitleSet JSONTitleSet `json:"JSON Title Set"`
}

type Version struct {
	Arch          string        `json:"Arch"`
	Name          string        `json:"Name"`
	Official      bool          `json:"Official"`
	RepoDate      string        `json:"RepoDate"`
	RepoHash      string        `json:"RepoHash"`
	System        string        `json:"System"`
	Type          string        `json:"Type"`
	Version       VersionDetail `json:"Version"`
	VersionString string        `json:"VersionString"`
}

type VersionDetail struct {
	Major int `json:"Major"`
	Minor int `json:"Minor"`
	Point int `json:"Point"`
}

type Progress struct {
	Scanning Scanning `json:"Scanning"`
	State    string   `json:"State"`
}

type Scanning struct {
	Preview      int     `json:"Preview"`
	PreviewCount int     `json:"PreviewCount"`
	Progress     float64 `json:"Progress"`
	SequenceID   int     `json:"SequenceID"`
	Title        int     `json:"Title"`
	TitleCount   int     `json:"TitleCount"`
}

type JSONTitleSet struct {
	MainFeature int         `json:"MainFeature"`
	TitleList   []TitleList `json:"TitleList"`
}

type TitleList struct {
	AngleCount        int                    `json:"AngleCount"`
	AudioList         []AudioList            `json:"AudioList"`
	ChapterList       []ChapterList          `json:"ChapterList"`
	Color             Color                  `json:"Color"`
	Container         string                 `json:"Container"`
	Crop              []int                  `json:"Crop"`
	Duration          Duration               `json:"Duration"`
	FrameRate         FrameRate              `json:"FrameRate"`
	Geometry          Geometry               `json:"Geometry"`
	Index             int                    `json:"Index"`
	InterlaceDetected bool                   `json:"InterlaceDetected"`
	LooseCrop         []int                  `json:"LooseCrop"`
	Metadata          map[string]interface{} `json:"Metadata"`
	Name              string                 `json:"Name"`
	Path              string                 `json:"Path"`
	Playlist          int                    `json:"Playlist"`
	SubtitleList      []SubtitleList         `json:"SubtitleList"`
	Type              int                    `json:"Type"`
	VideoCodec        string                 `json:"VideoCodec"`
}

type AudioList struct {
	Attributes        AudioAttributes `json:"Attributes"`
	BitRate           int             `json:"BitRate"`
	ChannelCount      int             `json:"ChannelCount"`
	ChannelLayout     int             `json:"ChannelLayout"`
	ChannelLayoutName string          `json:"ChannelLayoutName"`
	Codec             int             `json:"Codec"`
	CodecName         string          `json:"CodecName"`
	CodecParam        int             `json:"CodecParam"`
	Description       string          `json:"Description"`
	LFECount          int             `json:"LFECount"`
	Language          string          `json:"Language"`
	LanguageCode      string          `json:"LanguageCode"`
	Name              string          `json:"Name"`
	SampleRate        int             `json:"SampleRate"`
	TrackNumber       int             `json:"TrackNumber"`
}

type AudioAttributes struct {
	AltCommentary    bool `json:"AltCommentary"`
	Commentary       bool `json:"Commentary"`
	Default          bool `json:"Default"`
	Normal           bool `json:"Normal"`
	Secondary        bool `json:"Secondary"`
	VisuallyImpaired bool `json:"VisuallyImpaired"`
}

type ChapterList struct {
	Duration Duration `json:"Duration"`
	Name     string   `json:"Name"`
}

type Duration struct {
	Hours   int `json:"Hours"`
	Minutes int `json:"Minutes"`
	Seconds int `json:"Seconds"`
	Ticks   int `json:"Ticks"`
}

type Color struct {
	BitDepth          int    `json:"BitDepth"`
	ChromaLocation    int    `json:"ChromaLocation"`
	ChromaSubsampling string `json:"ChromaSubsampling"`
	Format            int    `json:"Format"`
	Matrix            int    `json:"Matrix"`
	Primary           int    `json:"Primary"`
	Range             int    `json:"Range"`
	Transfer          int    `json:"Transfer"`
}

type FrameRate struct {
	Den int `json:"Den"`
	Num int `json:"Num"`
}

type Geometry struct {
	Height int `json:"Height"`
	PAR    PAR `json:"PAR"`
	Width  int `json:"Width"`
}

type PAR struct {
	Den int `json:"Den"`
	Num int `json:"Num"`
}

type SubtitleList struct {
	Attributes   SubtitleAttributes `json:"Attributes"`
	Format       string             `json:"Format"`
	Language     string             `json:"Language"`
	LanguageCode string             `json:"LanguageCode"`
	Name         string             `json:"Name"`
	Source       int                `json:"Source"`
	SourceName   string             `json:"SourceName"`
	TrackNumber  int                `json:"TrackNumber"`
}

type SubtitleAttributes struct {
	By3           bool `json:"4By3"`
	Children      bool `json:"Children"`
	ClosedCaption bool `json:"ClosedCaption"`
	Commentary    bool `json:"Commentary"`
	Default       bool `json:"Default"`
	Forced        bool `json:"Forced"`
	Large         bool `json:"Large"`
	Letterbox     bool `json:"Letterbox"`
	Normal        bool `json:"Normal"`
	PanScan       bool `json:"PanScan"`
	Wide          bool `json:"Wide"`
}
