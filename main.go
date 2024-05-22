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
		fmt.Println(string(out))
		return err
	} else {
		log.Debugf("output: %s", out)
	}
	return nil
}

func extractStreams(job *Job, path, t string) error {
	cmd := exec.Command(TheConfig.Ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return err
	}

	var probeOutput FFProbeOutput
	err = json.Unmarshal(out, &probeOutput)
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	log.Debugf("%+v", string(out))
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == t {
			log.Debugf("Stream: %+v", stream)
			id := fmt.Sprintf("%d-%s", stream.Index, stream.Tags.Language)
			var cmd *exec.Cmd
			var idd string
			var err error
			s := Stream{
				CodecName: stream.CodecName,
				Index:     stream.Index,
			}
			if stream.CodecType == "subtitle" {
				idd = fmt.Sprintf("%s.%s", id, TheConfig.SubtitleExt)
				log.Infof("Handling subtitle stream #%d (%s)", stream.Index, stream.CodecName)
				pair := &Pair[Subtitle]{}
				job.Subtitles[stream.Index] = pair
				pair.Raw = &Subtitle{
					Language: stream.Tags.Language,
					Stream:   s,
				}
				cmd = exec.Command(TheConfig.Ffmpeg, "-i", path, "-c:s", TheConfig.SubtitleCodec, "-map", fmt.Sprintf("0:%d", stream.Index), filepath.Join(job.OutputPath, idd))
				err = runCommand(cmd)
				subtitle := &Subtitle{
					Language: stream.Tags.Language,
					Stream: Stream{
						CodecName: TheConfig.SubtitleCodec,
						Index:     stream.Index,
						Location:  idd,
					},
				}
				if err == nil {
					pair.Enc = subtitle
				} else {
					toCodec, ok := codecMap[stream.CodecName]
					if !ok {
						toCodec = stream.CodecName
					}
					log.Errorf("error converting subtitle: %v, extract: %s", err, toCodec)
					idd = fmt.Sprintf("%s.%s", id, toCodec)
					cmd = exec.Command(TheConfig.Ffmpeg, "-i", path, "-c:s", "copy", "-map", fmt.Sprintf("0:%d", stream.Index), filepath.Join(job.OutputPath, idd))
					err = runCommand(cmd)
					if err == nil {
						subtitle.Stream.Location = idd
						subtitle.Stream.CodecName = toCodec
						pair.Enc = subtitle
					} else {
						log.Errorf("error extracting raw subtitle: %v", err)
					}
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
				if TheConfig.EnableAudioExtraction {
					cmd = exec.Command(TheConfig.Ffmpeg, "-i", path, "-map", fmt.Sprintf("0:%d", stream.Index), "-c:a", "copy", outputFile)
					err := runCommand(cmd)
					if err == nil {
						pair.Enc = &Audio{
							Channels: stream.Channels,
							Language: stream.Tags.Language,
							Stream: Stream{
								CodecName: stream.CodecName,
								Index:     stream.Index,
								Location:  idd,
							},
						}
					} else {
						log.Errorf("error extracting audio: %v", err)
					}
				}
			}
		}
	}
	return nil
}

func handbrakeTranscode(job *Job) error {
	encoders := strings.Split(TheConfig.Encoder, ",")
	wg := sync.WaitGroup{}
	job.EncodedExt = TheConfig.VideoExt
	runEncoder := func(encoder string, encoderCmd string, encoderPreset string) {
		outputFile := filepath.Join(job.OutputPath, fmt.Sprintf("%s.%s", encoder, TheConfig.VideoExt))
		log.Infof("Converting video: %s -> %s", job.Input, outputFile)
		cmd := exec.Command(
			TheConfig.HandbrakeCli,
			"-i", job.Input,
			"-o", outputFile,
			"--encoder", encoderCmd,
			"--vfr",
			"--quality", TheConfig.ConstantQuality,
			"--encoder-preset", encoderPreset,
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
				job.EncodedCodecs = append(job.EncodedCodecs, encoder)
			}
			wg.Done()
		}()
	}
	for _, encoder := range encoders {
		switch encoder {
		case "av1":
			runEncoder(encoder, TheConfig.Av1Encoder, TheConfig.Av1Preset)
		case "hevc":
			runEncoder(encoder, TheConfig.HevcEncoder, TheConfig.HevcPreset)
		case "h264":
			runEncoder(encoder, TheConfig.H264Encoder, TheConfig.H264Preset)
		default:
			return fmt.Errorf("unsupported encoder: %s", encoder)
		}
	}
	wg.Wait()
	return nil
}

func pipeline(inputFile string) (*Job, error) {
	job := Job{
		Id:          RandomString(8),
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
	err = thumbnailsNfo(&job)
	if err != nil {
		return &job, err
	}
	err = extractStreams(&job, job.Input, "subtitle")
	if err != nil {
		return &job, err
	}
	if TheConfig.EnableSprite {
		err = spriteVtt(&job)
		if err != nil {
			return &job, err
		}
	}
	job.State = StreamsExtracted
	err = persistJob(job)
	if err != nil {
		return &job, err
	}
	if TheConfig.EnableEncode {
		err = handbrakeTranscode(&job)
		if err != nil {
			return &job, err
		}
		if len(job.EncodedCodecs) > 0 {
			err = extractStreams(&job, job.GetCodecVideo(job.EncodedCodecs[0]), "audio")
			if err != nil {
				return &job, err
			}
			mapAudioTracks(&job)
		}
	}
	job.State = Complete
	err = persistJob(job)
	if err != nil {
		return &job, err
	}
	return &job, nil
}

func mapAudioTracks(job *Job) {
	for _, pair := range job.Audios {
		if pair.Enc != nil {
			for _, codec := range job.EncodedCodecs {
				id := fmt.Sprintf("%s-%d-%s", codec, pair.Enc.Index, pair.Enc.Language)
				cmd := exec.Command(TheConfig.Ffmpeg, "-i", job.GetCodecVideo(codec), "-i", filepath.Join(job.OutputPath, pair.Enc.Location),
					"-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "copy", "-shortest", filepath.Join(job.OutputPath, fmt.Sprintf("%s.%s", id, TheConfig.VideoExt)))
				log.Infof("Command: %s", cmd.String())
				err := runCommand(cmd)
				if err != nil {
					log.Errorf("error mapping audio tracks: %v", err)
				} else {
					if _, ok := job.MappedAudio[codec]; !ok {
						job.MappedAudio[codec] = make(map[int]*Pair[Audio])
					}
					job.MappedAudio[codec][pair.Enc.Index] = pair
				}
			}
		}
	}
	return
}

func renameAndMove(source string, dest string) {
	_, err := os.Stat(source)
	if err == nil {
		err = os.Rename(source, dest)
		if err != nil {
			log.Errorf("error moving file: %s->%s %v", source, dest, err)
		}
	}
}

func thumbnailsNfo(job *Job) (err error) {
	renameAndMove(filepath.Join(job.FileRawFolder, "movie.nfo"), filepath.Join(job.OutputPath, "info.nfo"))
	renameAndMove(filepath.Join(job.FileRawFolder, job.FileRawName+".nfo"), filepath.Join(job.OutputPath, "info.nfo"))
	renameAndMove(filepath.Join(job.FileRawFolder, job.FileRawName+"-thumb.jpg"), filepath.Join(job.OutputPath, "poster.jpg"))
	renameAndMove(filepath.Join(job.FileRawFolder, "poster.jpg"), filepath.Join(job.OutputPath, "poster.jpg"))
	renameAndMove(filepath.Join(job.FileRawFolder, "fanart.jpg"), filepath.Join(job.OutputPath, "fanart.jpg"))
	return
}

func spriteVtt(job *Job) (err error) {
	vttFile := filepath.Join(job.OutputPath, ThumbnailVtt)
	videoFile := job.Input
	thumbnailHeight := TheConfig.ThumbnailHeight
	thumbnailInterval := TheConfig.ThumbnailInterval
	chunkInterval := TheConfig.ThumbnailChunkInterval

	// Get video duration and aspect ratio using FFprobe
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoFile).Output()
	if err != nil {
		log.Errorf("Error getting video duration: %v\n", err)
		return
	}
	job.Duration, _ = strconv.ParseFloat(strings.TrimSpace(string(out)), 64)

	out, err = exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", videoFile).Output()
	if err != nil {
		log.Errorf("Error getting video aspect ratio: %v\n", err)
		return
	}
	aspectRatioStr := strings.TrimSpace(string(out))
	aspectRatioParts := strings.Split(aspectRatioStr, "x")
	job.Width, _ = strconv.Atoi(aspectRatioParts[0])
	job.Height, _ = strconv.Atoi(aspectRatioParts[1])
	aspectRatio := float64(job.Width) / float64(job.Height)
	log.Infof("Width: %d, Height: %d, Duration: %f, Aspect Ratio: %f", job.Width, job.Height, job.Duration, aspectRatio)
	numThumbnailsPerChunk := chunkInterval / thumbnailInterval
	numChunks := int(math.Ceil(job.Duration / float64(chunkInterval)))
	thumbnailWidth := int(math.Round(float64(thumbnailHeight) * aspectRatio))
	gridSize := int(math.Ceil(math.Sqrt(float64(numThumbnailsPerChunk))))

	vttContent := "WEBVTT\n\n"
	for i := 0; i < numChunks; i++ {
		chunkStartTime := i * chunkInterval
		spriteFile := filepath.Join(job.OutputPath, fmt.Sprintf("%s_%d%s", SpritePrefix, i+1, SpriteExtension))
		cmd := exec.Command("ffmpeg", "-i", videoFile, "-ss", fmt.Sprintf("%d", chunkStartTime), "-t", fmt.Sprintf("%d", chunkInterval),
			"-vf", fmt.Sprintf("fps=1/%d,scale=%d:%d,tile=%dx%d", thumbnailInterval, thumbnailWidth, thumbnailHeight, gridSize, gridSize), spriteFile)
		log.Infof("Command: %s", cmd.String())
		err = runCommand(cmd)
		if err != nil {
			log.Errorf("Error generating sprite sheet for chunk %d: %v\n", i+1, err)
			return
		}

		for j := 0; j < numThumbnailsPerChunk; j++ {
			thumbnailTime := i*chunkInterval + j*thumbnailInterval
			startHour := thumbnailTime / 3600
			startMinute := (thumbnailTime % 3600) / 60
			startSecond := thumbnailTime % 60
			startTime := fmt.Sprintf("%02d:%02d:%02d.000", startHour, startMinute, startSecond)

			endThumbnailTime := thumbnailTime + thumbnailInterval
			endHour := endThumbnailTime / 3600
			endMinute := (endThumbnailTime % 3600) / 60
			endSecond := endThumbnailTime % 60
			endTime := fmt.Sprintf("%02d:%02d:%02d.000", endHour, endMinute, endSecond)

			row := j / gridSize
			col := j % gridSize
			thumbnailCoords := fmt.Sprintf("%d,%d,%d,%d", col*thumbnailWidth, row*thumbnailHeight, thumbnailWidth, thumbnailHeight)
			vttContent += fmt.Sprintf("%s --> %s\n%s#xywh=%s\n\n", startTime, endTime, fmt.Sprintf("%s_%d%s", SpritePrefix, i+1, SpriteExtension), thumbnailCoords)
		}
	}

	err = os.WriteFile(vttFile, []byte(vttContent), 0644)
	if err != nil {
		log.Errorf("Error writing WebVTT file: %v\n", err)
		return
	}

	log.Infof("Sprite sheets and WebVTT file generated successfully!")
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

func processFile(file os.DirEntry, path string) bool {
	ext := filepath.Ext(file.Name())
	if slices.Contains(ValidExtensions, ext[1:]) {
		startTime := time.Now()
		log.Infof("Processing file: %s", file.Name())
		job, err := pipeline(path)
		if err != nil {
			log.Errorf("error processing file: %v", err)
		}
		log.Infof("Processed %s, time cost: %s", file.Name(), time.Since(startTime))
		if job.State == Complete && TheConfig.RemoveOnSuccess {
			err = os.Remove(job.Input)
			if err != nil {
				log.Errorf("error removing file: %v", err)
			}
			return true
		} else {
			err = os.Rename(job.Input, path)
			if err != nil {
				log.Errorf("error renaming file: %v", err)
			}
			return false
		}
	}
	return false
}

func encode() error {
	files, err := os.ReadDir(TheConfig.Input)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			fs, err := os.ReadDir(filepath.Join(TheConfig.Input, file.Name()))
			if err != nil {
				return err
			}
			for _, f := range fs {
				if processFile(f, filepath.Join(TheConfig.Input, file.Name(), f.Name())) {
					err = os.RemoveAll(filepath.Join(TheConfig.Input, file.Name()))
				}
			}
		} else {
			processFile(file, filepath.Join(TheConfig.Input, file.Name()))
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
