package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

func runCommand(cmd *exec.Cmd) ([]byte, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Info(cmd.String())
		fmt.Println(string(out))
		return out, err
	} else {
		log.Debugf("output: %s", out)
	}
	return out, err
}

func (job *Job) extractStreams(path, t string) error {
	cmd := exec.Command(TheConfig.Ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", path)
	out, err := runCommand(cmd)
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
			var err error
			convert := func(codec, cs, filename string) error {
				log.Infof("Handling %s stream #%d (%s)", stream.CodecType, stream.Index, stream.CodecName)
				s := Stream{
					CodecName: codec,
					CodecType: stream.CodecType,
					Index:     stream.Index,
					Language:  stream.Tags.Language,
					Title:     stream.Tags.Title,
					Filename:  stream.Tags.Filename,
					MimeType:  stream.Tags.MimeType,
					Location:  filename,
					Channels:  stream.Channels,
				}
				if stream.CodecType == AttachmentType {
					cmd = exec.Command(TheConfig.Ffmpeg, "-y", fmt.Sprintf("-dump_attachment:%d", stream.Index), job.OutputJoin(filename), "-i", path, "-t", "0", "-f", "null", "null")
				} else {
					cmd = exec.Command(TheConfig.Ffmpeg, "-y", "-i", path, "-c:s", cs, "-map", fmt.Sprintf("0:%d", stream.Index), job.OutputJoin(filename))
				}
				_, err = runCommand(cmd)
				if err == nil {
					job.Streams = append(job.Streams, s)
				} else {
					log.Errorf("error converting %s: %v", t, err)
				}
				return err
			}
			switch stream.CodecType {
			case SubtitlesType:
				errAss := convert("ass", "ass", fmt.Sprintf("%s.ass", id))
				errVtt := convert("webvtt", "webvtt", fmt.Sprintf("%s.vtt", id))
				if errAss != nil && errVtt != nil {
					toCodec, ok := codecMap[stream.CodecName]
					if !ok {
						toCodec = stream.CodecName
					}
					err = convert(toCodec, "copy", fmt.Sprintf("%s.%s", id, toCodec))
				}
			case AudioType:
				if TheConfig.EnableAudioExtraction {
					err = convert(stream.CodecName, "copy", fmt.Sprintf("%s.%s", id, stream.CodecName))
				}
			case AttachmentType:
				if TheConfig.EnableAttachmentExtraction {
					err = convert(stream.Tags.MimeType, "copy", stream.Tags.Filename)
				}
			}
		}
	}
	return nil
}

func (job *Job) handbrakeTranscode() error {
	encoders := strings.Split(TheConfig.Encoder, ",")
	wg := sync.WaitGroup{}
	job.EncodedExt = TheConfig.VideoExt
	runEncoder := func(encoder, encoderCmd, encoderPreset, encoderProfile, encoderTune string) {
		outputFile := job.OutputJoin(fmt.Sprintf("%s.%s", encoder, TheConfig.VideoExt))
		log.Infof("Converting video: %s -> %s", job.Input, outputFile)
		args := []string{
			"-i", job.InputJoin(job.InputAfterRename()),
			"-o", outputFile,
			"--encoder", encoderCmd,
			"--vfr",
			"--quality", TheConfig.ConstantQuality,
			"--encoder-preset", encoderPreset,
			"--subtitle", "none",
			"--aencoder", "opus",
			"--audio-lang-list", "any",
			"--all-audio",
			"--mixdown", "stereo"}
		if encoderProfile != "" {
			args = append(args, "--encoder-profile", encoderProfile)
		}
		if encoderTune != "" {
			args = append(args, "--encoder-tune", encoderTune)
		}
		cmd := exec.Command(
			TheConfig.HandbrakeCli, args...)
		wg.Add(1)
		go func() {
			out, err := runCommand(cmd)
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
			runEncoder(encoder, TheConfig.Av1Encoder, TheConfig.Av1Preset, "", "")
		case "hevc":
			runEncoder(encoder, TheConfig.HevcEncoder, TheConfig.HevcPreset, "", "")
		case "h264-10bit":
			runEncoder(encoder, TheConfig.H26410BitEncoder, TheConfig.H26410BitPreset, "", "")
		case "h264-8bit":
			runEncoder(encoder, TheConfig.H2648BitEncoder, TheConfig.H2648BitPreset, TheConfig.H2648BitProfile, TheConfig.H2648BitTune)
		default:
			return fmt.Errorf("unsupported encoder: %s", encoder)
		}
	}
	wg.Wait()
	return nil
}

func (job *Job) pipeline() error {
	var err error
	if TheConfig.EnableRename {
		err = os.Rename(job.InputJoin(job.Input), job.InputJoin(job.InputAfterRename()))
		if err != nil {
			return err
		}
	}
	job.SHA256, err = calculateFileSHA256(job.InputJoin(job.InputAfterRename()))
	if err != nil {
		return err
	}
	log.Infof("Processing Job: %+v", job)
	err = os.MkdirAll(job.OutputJoin(), 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(job.OutputJoin(job.InputName()), []byte{}, 0644)
	err = job.updateState(Incomplete)
	if err != nil {
		return err
	}
	err = job.thumbnailsNfo()
	if err != nil {
		return err
	}
	err = job.extractStreams(job.InputJoin(job.InputAfterRename()), SubtitlesType)
	if err != nil {
		return err
	}
	err = job.extractStreams(job.InputJoin(job.InputAfterRename()), AttachmentType)
	if err != nil {
		return err
	}
	err = job.updateState(StreamsExtracted)
	if err != nil {
		return err
	}
	if TheConfig.EnableSprite {
		err = job.spriteVtt()
		if err != nil {
			return err
		}
	}
	if TheConfig.EnableEncode {
		err = job.handbrakeTranscode()
		if err != nil {
			return err
		}
		if len(job.EncodedCodecs) > 0 {
			err = job.extractStreams(job.GetCodecVideo(job.EncodedCodecs[0]), AudioType)
			if err != nil {
				return err
			}
			job.mapAudioTracks()
		}
	}
	err = job.updateState(Complete)
	if err != nil {
		return err
	}
	for _, codec := range job.EncodedCodecs {
		err = os.Remove(job.OutputJoin(fmt.Sprintf("%s.%s", codec, TheConfig.VideoExt)))
	}
	return nil
}

func (job *Job) mapAudioTracks() {
	job.MappedAudio = make(map[string][]Stream)
	for _, audio := range job.Streams {
		if audio.CodecType != AudioType {
			continue
		}
		for _, codec := range job.EncodedCodecs {
			id := fmt.Sprintf("%s-%d-%s", codec, audio.Index, audio.Language)
			cmd := exec.Command(TheConfig.Ffmpeg, "-i", job.GetCodecVideo(codec), "-i", job.OutputJoin(audio.Location),
				"-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "copy", "-shortest", job.OutputJoin(fmt.Sprintf("%s.%s", id, TheConfig.VideoExt)))
			log.Infof("Command: %s", cmd.String())
			_, err := runCommand(cmd)
			if err != nil {
				log.Errorf("error mapping audio tracks: %v", err)
			} else {
				if _, ok := job.MappedAudio[codec]; !ok {
					job.MappedAudio[codec] = make([]Stream, 0)
				}
				job.MappedAudio[codec] = append(job.MappedAudio[codec], audio)
			}
		}
	}
	return
}

func (job *Job) renameAndMove(source string, dest string) {
	source = job.InputJoin(source)
	dest = job.OutputJoin(dest)
	_, err := os.Stat(source)
	if err == nil {
		if TheConfig.RemoveOnSuccess {
			err = os.Rename(source, dest)
			if err != nil {
				log.Errorf("error moving file: %s->%s %v", source, dest, err)
			}
		} else {
			_, err = copyFile(source, dest)
			if err != nil {
				log.Errorf("error copying file: %s->%s %v", source, dest, err)
			}
		}
	}
}

func (job *Job) thumbnailsNfo() (err error) {
	job.renameAndMove("movie.nfo", "info.nfo")
	job.renameAndMove(job.InputName()+".nfo", "info.nfo")
	job.renameAndMove(job.InputName()+"-thumb.jpg", "poster.jpg")
	job.renameAndMove("poster.jpg", "poster.jpg")
	job.renameAndMove("fanart.jpg", "fanart.jpg")
	return
}

func (job *Job) spriteVtt() (err error) {
	vttFile := job.OutputJoin(ThumbnailVtt)
	videoFile := job.InputJoin(job.InputAfterRename())
	thumbnailHeight := TheConfig.ThumbnailHeight
	thumbnailInterval := TheConfig.ThumbnailInterval
	chunkInterval := TheConfig.ThumbnailChunkInterval

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
		spriteFile := job.OutputJoin(fmt.Sprintf("%s_%d%s", SpritePrefix, i+1, SpriteExtension))
		cmd := exec.Command("ffmpeg", "-i", videoFile, "-ss", fmt.Sprintf("%d", chunkStartTime), "-t", fmt.Sprintf("%d", chunkInterval),
			"-vf", fmt.Sprintf("fps=1/%d,scale=%d:%d,tile=%dx%d", thumbnailInterval, thumbnailWidth, thumbnailHeight, gridSize, gridSize), spriteFile)
		log.Infof("Command: %s", cmd.String())
		_, err = runCommand(cmd)
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

func (job *Job) updateState(newState string) error {
	job.State = newState
	jobStr, err := json.Marshal(job)
	if err != nil {
		log.Errorf("error persisting job: %v", err)
		return err
	}
	err = os.WriteFile(job.OutputJoin(JobFile), jobStr, 0644)
	if err != nil {
		log.Errorf("error persisting job: %v", err)
		return err
	}
	return nil
}

func processFile(file os.DirEntry, parent string) bool {
	ext := filepath.Ext(file.Name())
	jobs, err := jobsCache.Get()
	if err != nil {
		log.Errorf("error getting all jobs: %v", err)
		return false
	}
	for _, job := range jobs {
		if job["Input"] == file.Name() && job["State"] == Complete {
			log.Infof("File exists: %s", file.Name())
			return false
		}
	}
	if slices.Contains(ValidExtensions, ext[1:]) {
		job := Job{
			Id:          RandomString(8),
			InputParent: parent,
			Input:       file.Name(),
		}
		startTime := time.Now()
		log.Infof("Processing file: %s", file.Name())
		err := job.pipeline()
		if err != nil {
			log.Errorf("error processing file: %v", err)
		}
		log.Infof("Processed %s, time cost: %s", file.Name(), time.Since(startTime))
		if job.State == Complete && TheConfig.RemoveOnSuccess {
			err = os.Remove(job.InputJoin(job.InputAfterRename()))
			if err != nil {
				log.Errorf("error removing file: %v", err)
			}
			return true
		} else if TheConfig.EnableRename {
			err = os.Rename(job.InputJoin(job.InputAfterRename()), job.InputJoin(job.Input))
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
			fs, err := os.ReadDir(InputJoin(file.Name()))
			if err != nil {
				return err
			}
			for _, f := range fs {
				if processFile(f, file.Name()) && TheConfig.RemoveOnSuccess {
					err = os.RemoveAll(InputJoin(file.Name()))
				}
			}
		} else {
			processFile(file, "")
		}
	}
	return nil
}

var showsKeywords = []string{
	"blessing on this wonderful world,specials,3",
}
var showsRoots = []string{"O:\\Managed-Videos\\Anime"}
var moviesRoot = []string{"O:\\Managed-Videos\\Movies"}
var moviesKeywords = []string{
	"Soda Pop",
}

var re = regexp.MustCompile(`Season\s+\d+`)

func encodeShows(root string) {
	files, err := os.ReadDir(root)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			for _, keyword := range showsKeywords {
				s := strings.Split(keyword, ",")
				showName := s[0]
				seasons := mapset.NewSet[string]()
				if len(s) > 1 {
					for i := 1; i < len(s); i++ {
						if strings.ToLower(s[i]) == "specials" {
							seasons.Add("Specials")
						} else {
							seasons.Add(fmt.Sprintf("Season %s", s[i]))
						}
					}
				}
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(showName)) {
					fs, err := os.ReadDir(filepath.Join(root, file.Name()))
					if err != nil {
						log.Fatalf("error reading directory: %v", err)
					}
					for _, f := range fs {
						p := func() {
							root := filepath.Join(root, file.Name(), f.Name())
							log.Infof("Processing %s", root)
							TheConfig.Input = root
							err := encode()
							if err != nil {
								log.Errorf("error: %v", err)
							}
						}
						if f.IsDir() && (re.MatchString(f.Name()) || f.Name() == "Specials") {
							if seasons.Cardinality() > 0 {
								if seasons.Contains(f.Name()) {
									p()
								}
							} else {
								p()
							}
						}
					}
				}
			}
		}
	}
}

func encodeMovies(root string) {
	files, err := os.ReadDir(root)
	if err != nil {
		log.Fatalf("error reading directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			for _, keyword := range moviesKeywords {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(keyword)) {
					root := filepath.Join(root, file.Name())
					log.Infof("Processing %s", root)
					TheConfig.Input = root
					err = encode()
					if err != nil {
						log.Errorf("error: %v", err)
					}
				}
			}
		}
	}
}

func main() {
	log.SetLevel(log.InfoLevel)
	configure()
	cleanup.InitSignalCallback()
	log.Infof("Starting in %s mode", TheConfig.Mode)
	switch TheConfig.Mode {
	case EncodingMode:
		for _, root := range showsRoots {
			encodeShows(root)
		}
		for _, root := range moviesRoot {
			encodeMovies(root)
		}
	case RESTMode:
		REST()
	}
}
