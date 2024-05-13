package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var splitter = string(os.PathSeparator)

func runCommand(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output: %s", out)
		return err
	} else {
		log.Debugf("output: %s", out)
	}
	return nil
}

func extractStream(job *Job, stream StreamInfo) {
	id := fmt.Sprintf("%d-%s", stream.Index, stream.Tags.Language)
	var cmd *exec.Cmd
	var idd string
	var err error
	s := Stream{
		CodecName: stream.CodecName,
		Index:     stream.Index,
	}
	if stream.CodecType == "subtitle" {
		idd = fmt.Sprintf("%s%s", id, TheConfig.SubtitleExt)
		outputFile := filepath.Join(job.OutputPath, idd)
		log.Infof("Handling subtitle stream #%d (%s)", stream.Index, stream.CodecName)
		pair := &Pair[Subtitle]{}
		job.Subtitles[stream.Index] = pair
		pair.Raw = &Subtitle{
			Language: stream.Tags.Language,
			Stream:   s,
		}
		cmd = exec.Command(TheConfig.Ffmpeg, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), outputFile)
		err = runCommand(cmd)
		if err == nil {
			pair.Enc = &Subtitle{
				Language: stream.Tags.Language,
				Stream: Stream{
					CodecName: TheConfig.SubtitleExt,
					Index:     stream.Index,
					Location:  idd,
				},
			}
		} else {
			log.Errorf("error extracting subtitle: %v", err)
		}
	} else if stream.CodecType == "audio" {
		idd = fmt.Sprintf("%s.%s", id, stream.CodecName)
		outputFile := filepath.Join(job.OutputPath, idd)
		log.Infof("Handling audio stream #%d (%s)", stream.Index, stream.CodecName)
		pair := &Pair[Audio]{}
		job.Audios[stream.Index] = pair
		pair.Raw = &Audio{
			Channels: stream.Channels,
			Stream:   s,
		}
		if !TheConfig.SkipAudioExtraction {
			cmd = exec.Command(TheConfig.Ffmpeg, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), "-c:a", "copy", outputFile)
			err := runCommand(cmd)
			if err == nil {
				pair.Enc = &Audio{
					Channels: stream.Channels,
					Stream: Stream{
						CodecName: stream.CodecName,
						Index:     stream.Index,
					},
				}
			} else {
				log.Errorf("error extracting audio: %v", err)
			}
		}
	}
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
		extractStream(job, stream)
	}
	return nil
}

func handbrakeTranscode(job *Job) error {
	encoders := strings.Split(TheConfig.Encoder, ",")
	wg := sync.WaitGroup{}
	for _, encoder := range encoders {
		switch encoder {
		case "av1":
			outputFile := filepath.Join(job.OutputPath, fmt.Sprintf("av1.%s", TheConfig.VideoExt))
			log.Infof("Converting video: %s -> %s", job.Input, outputFile)
			cmd := exec.Command(
				TheConfig.HandbrakeCli,
				"-i", job.Input,
				"-o", outputFile,
				"--encoder", TheConfig.Av1Encoder,
				"--vfr",
				"--quality", TheConfig.ConstantQuality,
				"--encoder-preset", TheConfig.Av1Preset,
				"--subtitle", "none",
				"--aencoder", "opus",
				"--audio-lang-list", "any",
				"--all-audio",
				"--mixdown", "stereo",
			)
			wg.Add(1)
			go func() {
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Errorf("output: %s", out)
				} else {
					log.Debugf("output: %s", out)
					job.EncodedCodecs = append(job.EncodedCodecs, "av1")
					job.EncodedExt = TheConfig.VideoExt
				}
				wg.Done()
			}()
		case "hevc":
			outputFile := filepath.Join(job.OutputPath, fmt.Sprintf("hevc.%s", TheConfig.VideoExt))
			log.Infof("Converting video: %s -> %s", job.Input, outputFile)
			cmd := exec.Command(
				TheConfig.HandbrakeCli,
				"-i", job.Input,
				"-o", outputFile,
				"--encoder", TheConfig.HevcEncoder,
				"--vfr",
				"--quality", TheConfig.ConstantQuality,
				"--encoder-preset", TheConfig.NvencPreset,
				"--subtitle", "none",
				"--aencoder", "opus",
				"--audio-lang-list", "any",
				"--all-audio",
				"--mixdown", "stereo",
			)
			wg.Add(1)
			go func() {
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Errorf("output: %s", out)
				} else {
					log.Debugf("output: %s", out)
					job.EncodedCodecs = append(job.EncodedCodecs, "hevc")
					job.EncodedExt = TheConfig.VideoExt
				}
				wg.Done()
			}()
		default:
			return fmt.Errorf("unsupported encoder: %s", encoder)
		}
	}
	wg.Wait()
	return nil
}

func convertVideoToSVTAV1FFMPEG(job Job) error {
	outputFile := filepath.Join(job.OutputPath, fmt.Sprintf("out%s", TheConfig.VideoExt))
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.Input, outputFile)
	// ffmpeg -i input_video.mp4 -map 0:v -map 0:a -c:v libsvtav1 -preset 6 -crf 22 -c:a libopus -vbr on -sn -vf "format=yuv420p10le" output_video.mkv
	cmd := exec.Command(
		TheConfig.Ffmpeg,
		"-i", job.Input,
		"-c:v", "libsvtav1",
		"-preset", TheConfig.Av1Preset,
		"-crf", TheConfig.ConstantQuality,
		"-c:a", "libopus",
		"-vbr", "on",
		"-sn",
		"-vf", "format=yuv420p10le",
		"-filter:a", "channelmap=FL-FL|FR-FR|FC-FC|LFE-LFE|SL-BL|SR-BR:5.1",
		outputFile,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output: %s", out)
		log.Errorf("command: %s", cmd.String())
	} else {
		log.Debugf("output: %s", out)
	}
	return err
}

func pipeline(inputFile string) (*Job, error) {
	job := Job{
		Id:          RandomString(32),
		FileRawPath: inputFile,
		Subtitles:   make(map[int]*Pair[Subtitle]),
		Videos:      make(map[int]*Pair[Video]),
		Audios:      make(map[int]*Pair[Audio]),
	}
	s := strings.Split(job.FileRawPath, splitter)
	file := s[len(s)-1]
	job.FileRawFolder = strings.Join(s[:len(s)-1], splitter)
	s = strings.Split(file, ".")
	job.FileRawExt = s[len(s)-1]
	job.FileRawName = strings.Join(s[:len(s)-1], ".")
	if !slices.Contains(ValidExtensions, job.FileRawExt) {
		return &job, fmt.Errorf("unsupported file extension: %s", job.FileRawExt)
	}
	job.Input = filepath.Join(job.FileRawFolder, fmt.Sprintf("%s.%s", job.Id, job.FileRawExt))
	err := os.Rename(job.FileRawPath, job.Input)
	if err != nil {
		return &job, err
	}
	job.SHA256, err = calculateFileSHA256(job.Input)
	if err != nil {
		return &job, err
	}

	job.OutputPath = filepath.Join(TheConfig.Output, job.Id)
	log.Infof("Processing Job: %+v", job)
	err = os.MkdirAll(job.OutputPath, 0755)
	if err != nil {
		return &job, err
	}
	err = os.WriteFile(filepath.Join(job.OutputPath, job.FileRawName), []byte{}, 0644)
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
	err = handbrakeTranscode(&job)
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
	jobStr, err := json.Marshal(job)
	if err != nil {
		log.Errorf("error persisting job: %v", err)
		return err
	}
	err = os.WriteFile(filepath.Join(job.OutputPath, JobFile), jobStr, 0644)
	if err != nil {
		log.Errorf("error persisting job: %v", err)
		return err
	}
	return nil
}

func encode() error {
	// get all files under INPUT
	files, err := os.ReadDir(TheConfig.Input)
	if err != nil {
		return err
	}
	for _, file := range files {
		startTime := time.Now()
		log.Infof("Processing file: %s", file.Name())
		job, err := pipeline(filepath.Join(TheConfig.Input, file.Name()))
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
			err = os.Rename(job.Input, filepath.Join(TheConfig.Input, file.Name()))
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
	log.Infof("Starting in %s mode", TheConfig.Mode)
	switch TheConfig.Mode {
	case EncodingMode:
		err := encode()
		if err != nil {
			log.Errorf("error: %v", err)
		}
	case RESTMode:
		REST()
	}
}
