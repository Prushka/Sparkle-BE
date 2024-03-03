package main

import (
	"Sparkle/cleanup"
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/rueidis"

	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	HANDBRAKE   = "./HandBrakeCLI"
	FFPROBE     = "ffprobe"
	FFMPEG      = "ffmpeg"
	OUTPUT      = "./output"
	Av1Preset   = "6"
	SubtitleExt = ".vtt"
	VideoExt    = ".mp4"
	Audios      = ""
)

var rdb rueidis.Client

func extractStream(job *Job, stream StreamInfo, streamType string) error {
	outputFile := fmt.Sprintf("%s/%d-%s", job.OutputPath, stream.Index, streamType)
	var cmd *exec.Cmd
	if streamType == "subtitle" {
		cmd = exec.Command(FFMPEG, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), outputFile+"_"+stream.Tags.Language+SubtitleExt)
		job.Subtitles = append(job.Subtitles, outputFile)
	}
	out, err := cmd.CombinedOutput()
	log.Debugf("output: %s", out)
	return err
}

func extractStreams(job *Job) error {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", job.Input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	var probeOutput FFProbeOutput
	err = json.Unmarshal(out, &probeOutput)
	if err != nil {
		return err
	}
	log.Debugf("%+v", string(out))
	for _, stream := range probeOutput.Streams {
		log.Debugf("Stream: %+v", stream)
		switch stream.CodecType {
		case "subtitle":
			log.Infof("Extracting subtitle stream #%d (%s)", stream.Index, stream.CodecName)
			err = extractStream(job, stream, "subtitle")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func convertVideoToSVTAV1(job Job) error {
	outputFile := fmt.Sprintf("%s/out%s", job.OutputPath, VideoExt)
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.Input, outputFile)
	cmd := exec.Command(
		HANDBRAKE,
		"-i", job.Input, // Input file
		"-o", outputFile, // Output file
		"--encoder", "svt_av1_10bit", // Use AV1 encoder
		"--vfr",           // Variable frame rate
		"--quality", "24", // Constant quality RF 22
		"--encoder-preset", Av1Preset, // Encoder preset
		"--subtitle", "none", // No subtitles
		"--aencoder", "opus",
		"--audio-lang-list", "any",
		"--all-audio",
	)
	out, err := cmd.CombinedOutput()
	log.Debugf("output: %s", out)
	return err
}

func pipeline(inputFile string) error {
	job := Job{
		Id:          RandomString(32),
		FileRawPath: inputFile,
	}
	s := strings.Split(job.FileRawPath, "/")
	file := s[len(s)-1]
	job.FileRawFolder = strings.Join(s[:len(s)-1], "/")
	s = strings.Split(file, ".")
	job.FileRawName = s[0]
	job.FileRawExt = s[1]
	if !slices.Contains(ValidExtensions, job.FileRawExt) {
		return fmt.Errorf("unsupported file extension: %s", job.FileRawExt)
	}
	job.Input = fmt.Sprintf("%s/%s.%s", job.FileRawFolder, job.Id, job.FileRawExt)
	err := os.Rename(job.FileRawPath, job.Input)
	if err != nil {
		return err
	}
	job.SHA256, err = calculateFileSHA256(job.Input)
	if err != nil {
		return err
	}

	job.OutputPath = fmt.Sprintf("%s/%s", OUTPUT, job.Id)
	log.Infof("Processing Job: %+v", job)
	err = os.MkdirAll(job.OutputPath, 0755)
	if err != nil {
		return err
	}
	job.State = Incomplete
	err = persistJob(job)
	if err != nil {
		return err
	}
	err = extractStreams(&job)
	if err != nil {
		return err
	}
	job.State = StreamsExtracted
	err = persistJob(job)
	if err != nil {
		return err
	}
	err = convertVideoToSVTAV1(job)
	if err != nil {
		return err
	}
	job.State = Complete
	err = persistJob(job)
	if err != nil {
		return err
	}
	return nil
}

func persistJob(job Job) error {
	key := fmt.Sprintf("job:%s", job.Id)
	ctx := context.Background()
	log.Info(rdb)
	err := rdb.Do(ctx, rdb.B().JsonSet().Key(key).Path(".").Value(rueidis.JSON(job)).Build()).Error()
	if err != nil {
		log.Errorf("error persisting job: %v", err)
		return err
	}
	return nil
}

func test() {
	err := pipeline("./A Sign of Affection - S01E01 - Yuki's World WEBDL-1080p.mkv")
	if err != nil {
		log.Fatalf("error scanning input file: %v", err)
	}
}

func main() {
	log.SetLevel(log.InfoLevel)
	cleanup.InitSignalCallback()
	var err error
	rdb, err = rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{"192.168.50.200:6379"},
	})
	if err != nil {
		panic(err)
	}
	cleanup.AddOnStopFunc(cleanup.Redis, func(_ os.Signal) {
		rdb.Close()
	})
	test()
}
