package main

import (
	"Sparkle/cleanup"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/rueidis"

	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"slices"
	"strings"
)

var rdb rueidis.Client

func extractStream(job *Job, stream StreamInfo) error {
	id := fmt.Sprintf("%d-%s", stream.Index, stream.Tags.Language)
	var cmd *exec.Cmd
	if stream.CodecType == "subtitle" {
		idd := fmt.Sprintf("%s%s", id, TheConfig.SubtitleExt)
		outputFile := fmt.Sprintf("%s/%s", job.OutputPath, idd)
		log.Infof("Extracting subtitle stream #%d (%s)", stream.Index, stream.CodecName)
		cmd = exec.Command(TheConfig.Ffmpeg, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), outputFile)
		job.Subtitles = append(job.Subtitles, idd)
	} else if stream.CodecType == "audio" {
		idd := fmt.Sprintf("%s.wav", id)
		outputFile := fmt.Sprintf("%s/%s", job.OutputPath, idd)
		log.Infof("Extracting audio stream #%d (%s)", stream.Index, stream.CodecName)
		cmd = exec.Command(TheConfig.Ffmpeg, "-i", job.Input, "-map", fmt.Sprintf("0:a:%d", stream.Index), "-c:a", "copy", outputFile)
		job.RawAudios = append(job.RawAudios, Audio{
			Channels: stream.Channels,
			Stream: Stream{
				CodecType: stream.CodecType,
				CodecName: stream.CodecName,
			},
		})
	} else {
		return nil
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output: %s", out)
	} else {
		log.Debugf("output: %s", out)
	}
	return err
}

func extractStreams(job *Job) error {
	cmd := exec.Command(TheConfig.Ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", job.Input)
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
		err = extractStream(job, stream)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertVideoToSVTAV1(job Job) error {
	outputFile := fmt.Sprintf("%s/out%s", job.OutputPath, TheConfig.VideoExt)
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.Input, outputFile)
	cmd := exec.Command(
		TheConfig.HandbrakeCli,
		"-i", job.Input,
		"-o", outputFile,
		"--encoder", "svt_av1_10bit",
		"--vfr",
		"--quality", TheConfig.Av1Quality,
		"--encoder-preset", TheConfig.Av1Preset,
		"--subtitle", "none",
		"--aencoder", "opus",
		"--audio-lang-list", "any",
		"--all-audio",
	)
	out, err := cmd.CombinedOutput()
	log.Debugf("output: %s", out)
	return err
}

func convertVideoToSVTAV1FFMPEG(job Job) error {
	outputFile := fmt.Sprintf("%s/out%s", job.OutputPath, TheConfig.VideoExt)
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.Input, outputFile)
	// ffmpeg -i input_video.mp4 -map 0:v -map 0:a -c:v libsvtav1 -preset 6 -crf 22 -c:a libopus -vbr on -sn -vf "format=yuv420p10le" output_video.mkv
	cmd := exec.Command(
		TheConfig.Ffmpeg,
		"-i", job.Input,
		"-map", "0:v",
		"-c:v", "libsvtav1",
		"-preset", TheConfig.Av1Preset,
		"-crf", TheConfig.Av1Quality,
		"-vbr", "on",
		"-sn",
		"-an",
		"-vf", "\"format=yuv420p10le\"",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output: %s", out)
	} else {
		log.Debugf("output: %s", out)
	}
	return err
}

func pipeline(inputFile string) (*Job, error) {
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
		return &job, fmt.Errorf("unsupported file extension: %s", job.FileRawExt)
	}
	job.Input = fmt.Sprintf("%s/%s.%s", job.FileRawFolder, job.Id, job.FileRawExt)
	err := os.Rename(job.FileRawPath, job.Input)
	if err != nil {
		return &job, err
	}
	job.SHA256, err = calculateFileSHA256(job.Input)
	if err != nil {
		return &job, err
	}

	job.OutputPath = fmt.Sprintf("%s/%s", TheConfig.Output, job.Id)
	log.Infof("Processing Job: %+v", job)
	err = os.MkdirAll(job.OutputPath, 0755)
	if err != nil {
		return &job, err
	}
	job.State = Incomplete
	err = persistJob(job)
	if err != nil {
		return &job, err
	}
	err = extractStreams(&job)
	if err != nil {
		return &job, err
	}
	job.State = StreamsExtracted
	err = persistJob(job)
	if err != nil {
		return &job, err
	}
	err = convertVideoToSVTAV1FFMPEG(job)
	if err != nil {
		return &job, err
	}
	job.State = Complete
	err = persistJob(job)
	if err != nil {
		return &job, err
	}
	return &job, nil
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

func test() error {
	// get all files under INPUT
	files, err := os.ReadDir(TheConfig.Input)
	if err != nil {
		return err
	}
	for _, file := range files {
		startTime := time.Now()
		log.Infof("Processing file: %s", file.Name())
		job, err := pipeline(fmt.Sprintf("%s/%s", TheConfig.Input, file.Name()))
		if err != nil {
			log.Errorf("error processing file: %v", err)
		}
		log.Infof("Processed %s, time cost: %s", file.Name(), time.Since(startTime))
		// remove file
		if job.State == Complete {
			err = os.Remove(job.Input)
			if err != nil {
				log.Errorf("error removing file: %v", err)
			}
		} else {
			// rename back
			err = os.Rename(job.Input, fmt.Sprintf("%s/%s", TheConfig.Input, file.Name()))
			if err != nil {
				log.Errorf("error renaming file: %v", err)
			}
		}
	}
	return nil
}

func main() {
	log.SetLevel(log.InfoLevel)
	configure()
	cleanup.InitSignalCallback()
	var err error
	rdb, err = rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{TheConfig.Redis},
		Password:    TheConfig.RedisPassword,
	})
	if err != nil {
		panic(err)
	}
	cleanup.AddOnStopFunc(cleanup.Redis, func(_ os.Signal) {
		rdb.Close()
	})
	log.Infof("Starting in %s mode", TheConfig.Mode)
	switch TheConfig.Mode {
	case EncodingMode:
		err := test()
		if err != nil {
			log.Errorf("error: %v", err)
		}
	case RESTMode:
		REST()
	}
}
