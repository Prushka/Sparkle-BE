package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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
	err = spriteVtt(&job)
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

func spriteVtt(job *Job) (err error) {
	spriteFile := filepath.Join(job.OutputPath, ThumbnailPicture)
	vttFile := filepath.Join(job.OutputPath, ThumbnailVtt)
	videoFile := job.Input
	thumbnailHeight := TheConfig.ThumbnailHeight
	thumbnailInterval := TheConfig.ThumbnailInterval

	// Get video duration and aspect ratio using FFprobe
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoFile).Output()
	if err != nil {
		log.Errorf("Error getting video duration: %v\n", err)
		return
	}
	duration, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)

	out, err = exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", videoFile).Output()
	if err != nil {
		log.Errorf("Error getting video aspect ratio: %v\n", err)
		return
	}
	aspectRatioStr := strings.TrimSpace(string(out))
	aspectRatioParts := strings.Split(aspectRatioStr, "x")
	width, _ := strconv.Atoi(aspectRatioParts[0])
	height, _ := strconv.Atoi(aspectRatioParts[1])
	aspectRatio := float64(width) / float64(height)
	log.Infof("Width: %d, Height: %d, Duration: %f, Aspect Ratio: %f", width, height, duration, aspectRatio)

	// Calculate the number of thumbnails based on video duration and interval
	numThumbnails := int(math.Ceil(duration / float64(thumbnailInterval)))

	// Calculate thumbnail dimensions based on aspect ratio
	thumbnailWidth := int(math.Round(float64(thumbnailHeight) * aspectRatio))

	// Calculate the number of columns and rows for the sprite sheet
	numCols := int(math.Ceil(math.Sqrt(float64(numThumbnails))))
	numRows := int(math.Ceil(float64(numThumbnails) / float64(numCols)))

	// Generate sprite sheet using FFmpeg
	cmd := exec.Command("ffmpeg", "-i", videoFile, "-vf", fmt.Sprintf("fps=1/%d,scale=%d:%d,tile=%dx%d", thumbnailInterval, thumbnailWidth, thumbnailHeight, numCols, numRows), spriteFile)
	err = runCommand(cmd)
	if err != nil {
		log.Errorf("Error generating sprite sheet: %v\n", err)
		return
	}

	vttContent := "WEBVTT\n\n"
	for i := 0; i < numThumbnails; i++ {
		startTime := fmt.Sprintf("00:%02d:%02d.000", i*thumbnailInterval/60, i*thumbnailInterval%60)
		endTime := fmt.Sprintf("00:%02d:%02d.000", (i+1)*thumbnailInterval/60, (i+1)*thumbnailInterval%60)
		thumbnailCol := i % numCols
		thumbnailRow := i / numCols
		thumbnailCoords := fmt.Sprintf("%d,%d,%d,%d", thumbnailCol*thumbnailWidth, thumbnailRow*thumbnailHeight, thumbnailWidth, thumbnailHeight)
		vttContent += fmt.Sprintf("%s --> %s\n%s#xywh=%s\n\n", startTime, endTime, ThumbnailPicture, thumbnailCoords)
	}

	err = os.WriteFile(vttFile, []byte(vttContent), 0644)
	if err != nil {
		log.Errorf("Error writing WebVTT file: %v\n", err)
		return
	}

	log.Infof("Sprite sheet and WebVTT file generated successfully!")
	return
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
