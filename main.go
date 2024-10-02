package main

import (
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cenkalti/dominantcolor"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"image"
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

func Run(c *exec.Cmd) error {
	if err := c.Start(); err != nil {
		return err
	}
	if config.TheConfig.EnableLowPriority {
		err := lowPriority(c.Process.Pid)
		if err != nil {
			discord.Errorf("error setting priority: %v", err)
		}
	}
	return c.Wait()
}

func CombinedOutput(c *exec.Cmd) ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := Run(c)
	return b.Bytes(), err
}

func runCommand(cmd *exec.Cmd) ([]byte, error) {
	out, err := CombinedOutput(cmd)
	if err != nil {
		discord.Errorf(cmd.String())
		fmt.Println(string(out))
		return out, err
	} else {
		log.Debugf("output: %s", out)
	}
	return out, err
}

func (job *Job) extractChapters() error {
	cmd := exec.Command(config.TheConfig.Ffprobe, "-v", "quiet", "-print_format", "json", "-show_chapters", job.InputJoin(job.InputAfterRename()))
	out, err := runCommand(cmd)
	if err != nil {
		return err
	}
	var probeOutput FFProbeOutput
	err = json.Unmarshal(out, &probeOutput)
	if err != nil {
		return err
	}
	job.Chapters = probeOutput.Chapters
	discord.Infof("Chapters: %+v", job.Chapters)
	return nil
}

func (job *Job) extractStreams(path, t string) error {
	cmd := exec.Command(config.TheConfig.Ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", path)
	out, err := runCommand(cmd)
	if err != nil {
		return err
	}
	var probeOutput FFProbeOutput
	err = json.Unmarshal(out, &probeOutput)
	if err != nil {
		return err
	}
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == t {
			log.Debugf("Stream: %+v", stream)
			id := fmt.Sprintf("%d-%s", stream.Index, stream.Tags.Language)
			var cmd *exec.Cmd
			var err error
			convert := func(codec, cs, filename string) error {
				discord.Infof("Handling %s stream #%d (%s)", stream.CodecType, stream.Index, stream.CodecName)
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
					cmd = exec.Command(config.TheConfig.Ffmpeg, "-y", fmt.Sprintf("-dump_attachment:%d", stream.Index), job.OutputJoin(filename), "-i", path, "-t", "0", "-f", "null", "null")
				} else {
					cmd = exec.Command(config.TheConfig.Ffmpeg, "-y", "-i", path, "-c:s", cs, "-map", fmt.Sprintf("0:%d", stream.Index), job.OutputJoin(filename))
				}
				_, err = runCommand(cmd)
				if err == nil {
					job.Streams = append(job.Streams, s)
				} else {
					discord.Errorf("error converting %s: %v", t, err)
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
				if config.TheConfig.EnableAudioExtraction {
					err = convert(stream.CodecName, "copy", fmt.Sprintf("%s.%s", id, stream.CodecName))
				}
			case AttachmentType:
				if config.TheConfig.EnableAttachmentExtraction {
					err = convert(stream.Tags.MimeType, "copy", stream.Tags.Filename)
				}
			}
		}
	}
	return nil
}

func (job *Job) handbrakeTranscode() error {
	encoders := strings.Split(config.TheConfig.Encoder, ",")
	wg := sync.WaitGroup{}
	job.EncodedExt = config.TheConfig.VideoExt
	runEncoder := func(encoder, encoderCmd, encoderPreset, encoderProfile, encoderTune string) {
		outputFile := job.OutputJoin(fmt.Sprintf("%s.%s", encoder, config.TheConfig.VideoExt))
		discord.Infof("Converting video: %s -> %s", job.Input, outputFile)
		args := []string{
			"-i", job.InputJoin(job.InputAfterRename()),
			"-o", outputFile,
			"--encoder", encoderCmd,
			"--vfr",
			"--quality", config.TheConfig.ConstantQuality,
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
			config.TheConfig.HandbrakeCli, args...)
		wg.Add(1)
		go func() {
			_, err := runCommand(cmd)
			if err == nil {
				job.EncodedCodecs = append(job.EncodedCodecs, encoder)
			}
			wg.Done()
		}()
	}
	for _, encoder := range encoders {
		switch encoder {
		case "av1":
			runEncoder(encoder, config.TheConfig.Av1Encoder, config.TheConfig.Av1Preset, "", "")
		case "hevc":
			runEncoder(encoder, config.TheConfig.HevcEncoder, config.TheConfig.HevcPreset, "", "")
		case "h264-10bit":
			runEncoder(encoder, config.TheConfig.H26410BitEncoder, config.TheConfig.H26410BitPreset, "", "")
		case "h264-8bit":
			runEncoder(encoder, config.TheConfig.H2648BitEncoder, config.TheConfig.H2648BitPreset, config.TheConfig.H2648BitProfile, config.TheConfig.H2648BitTune)
		default:
			return fmt.Errorf("unsupported encoder: %s", encoder)
		}
	}
	wg.Wait()
	return nil
}

func (job *Job) pipeline() error {
	var err error
	if config.TheConfig.EnableRename {
		err = os.Rename(job.InputJoin(job.Input), job.InputJoin(job.InputAfterRename()))
		if err != nil {
			return err
		}
	}
	job.SHA256, err = calculateFileSHA256(job.InputJoin(job.InputAfterRename()))
	if err != nil {
		return err
	}
	discord.Infof("Processing Job: %+v", job)
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
	_ = job.extractDominantColor()
	err = job.extractChapters()
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
	if config.TheConfig.EnableSprite {
		err = job.spriteVtt()
		if err != nil {
			return err
		}
	}
	if config.TheConfig.EnableEncode {
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
		err = os.Remove(job.OutputJoin(fmt.Sprintf("%s.%s", codec, config.TheConfig.VideoExt)))
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
			cmd := exec.Command(config.TheConfig.Ffmpeg, "-i", job.GetCodecVideo(codec), "-i", job.OutputJoin(audio.Location),
				"-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "copy", "-shortest", job.OutputJoin(fmt.Sprintf("%s.%s", id, config.TheConfig.VideoExt)))
			discord.Infof("Command: %s", cmd.String())
			_, err := runCommand(cmd)
			if err != nil {
				discord.Errorf("error mapping audio tracks: %v", err)
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
		if config.TheConfig.RemoveOnSuccess {
			err = os.Rename(source, dest)
			if err != nil {
				discord.Errorf("error moving file: %s->%s %v", source, dest, err)
			}
		} else {
			_, err = copyFile(source, dest)
			if err != nil {
				discord.Errorf("error copying file: %s->%s %v", source, dest, err)
			}
		}
		discord.Infof("Moved %s to %s", source, dest)
	}
}

func (job *Job) thumbnailsNfo() (err error) {
	discord.Infof("Generating thumbnails and nfo files")
	job.renameAndMove("movie.nfo", "info.nfo")
	job.renameAndMove("tvshow.nfo", "info.nfo")
	job.renameAndMove(job.InputName()+".nfo", "info.nfo")
	job.renameAndMove(job.InputName()+"-thumb.jpg", "poster.jpg")
	job.renameAndMove("poster.jpg", "poster.jpg")
	job.renameAndMove("fanart.jpg", "fanart.jpg")
	return
}

func (job *Job) extractDominantColor() (err error) {
	f, err := os.Open(job.OutputJoin("poster.jpg"))
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			discord.Errorf("error closing file: %v", err)
		}
	}(f)
	if err != nil {
		discord.Errorf("Poster not found: " + job.OutputJoin("poster.jpg"))
		return err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		discord.Errorf("Error decoding image: %v", err)
		return err
	}
	color := dominantcolor.Hex(dominantcolor.Find(img))
	job.DominantColors = append(job.DominantColors, color)
	discord.Infof("Dominant color: %s", color)
	return nil
}

func (job *Job) spriteVtt() (err error) {
	vttFile := job.OutputJoin(ThumbnailVtt)
	videoFile := job.InputJoin(job.InputAfterRename())
	thumbnailHeight := config.TheConfig.ThumbnailHeight
	thumbnailInterval := config.TheConfig.ThumbnailInterval
	chunkInterval := config.TheConfig.ThumbnailChunkInterval

	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoFile).Output()
	if err != nil {
		discord.Errorf("Error getting video duration: %v\n", err)
		return
	}
	job.Duration, _ = strconv.ParseFloat(strings.TrimSpace(string(out)), 64)

	out, err = exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", videoFile).Output()
	if err != nil {
		discord.Errorf("Error getting video aspect ratio: %v\n", err)
		return
	}
	aspectRatioStr := strings.TrimSpace(string(out))
	aspectRatioParts := strings.Split(aspectRatioStr, "x")
	job.Width, _ = strconv.Atoi(aspectRatioParts[0])
	job.Height, _ = strconv.Atoi(aspectRatioParts[1])
	aspectRatio := float64(job.Width) / float64(job.Height)
	discord.Infof("Width: %d, Height: %d, Duration: %f, Aspect Ratio: %f", job.Width, job.Height, job.Duration, aspectRatio)
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
		discord.Infof("Command: %s", cmd.String())
		_, err = runCommand(cmd)
		if err != nil {
			discord.Errorf("Error generating sprite sheet for chunk %d: %v\n", i+1, err)
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
		discord.Errorf("Error writing WebVTT file: %v\n", err)
		return
	}

	discord.Infof("Sprite sheets and WebVTT file generated successfully!")
	return
}

func (job *Job) updateState(newState string) error {
	job.State = newState
	jobStr, err := json.Marshal(job)
	if err != nil {
		discord.Errorf("error persisting job: %v", err)
		return err
	}
	err = os.WriteFile(job.OutputJoin(JobFile), jobStr, 0644)
	if err != nil {
		discord.Errorf("error persisting job: %v", err)
		return err
	}
	return nil
}

func processFile(file os.DirEntry, parent string) bool {
	ext := filepath.Ext(file.Name())
	if slices.Contains(ValidExtensions, ext[1:]) {
		jobs, err := jobsCache.Get(false)
		if err != nil {
			discord.Errorf("error getting all jobs: %v", err)
			return false
		}
		stats, err := file.Info()
		if err != nil {
			discord.Errorf("error getting file info: %v", err)
			return false
		}
		currId := getTitleId(file.Name())
		discord.Infof("Current ID: %s", currId)
		for _, job := range jobs {
			prevId := getTitleId(job.Input)
			if currId == prevId {
				discord.Infof("File exists: %s", file.Name())
				if job.State == Complete && len(job.EncodedCodecs) > 0 && (job.OriSize == 0 || job.OriSize == stats.Size()) {
					return false
				} else {
					discord.Infof("File modified or prev encoding incomplete: %s, remove old", file.Name())
					err := os.RemoveAll(OutputJoin(job.Id))
					if err != nil {
						discord.Errorf("error removing file: %v", err)
					}
				}
			}
		}
		job := Job{
			Id:          newRandomString(5),
			InputParent: parent,
			Input:       file.Name(),
			OriSize:     stats.Size(),
			OriModTime:  stats.ModTime().Unix(),
		}
		startTime := time.Now()
		discord.Infof("Processing file: %s", file.Name())
		err = job.pipeline()
		if err != nil {
			discord.Errorf("error processing file: %v", err)
		}
		discord.Infof("Processed %s, time cost: %s", file.Name(), time.Since(startTime))
		if job.State == Complete && config.TheConfig.RemoveOnSuccess {
			err = os.Remove(job.InputJoin(job.InputAfterRename()))
			if err != nil {
				discord.Errorf("error removing file: %v", err)
			}
			return true
		} else if config.TheConfig.EnableRename {
			err = os.Rename(job.InputJoin(job.InputAfterRename()), job.InputJoin(job.Input))
			if err != nil {
				discord.Errorf("error renaming file: %v", err)
			}
			return false
		}
	}
	return false
}

func encode(matches func(s string) bool) error {
	files, err := os.ReadDir(config.TheConfig.Input)
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
				if matches == nil || matches(f.Name()) {
					if processFile(f, file.Name()) && config.TheConfig.RemoveOnSuccess {
						err = os.RemoveAll(InputJoin(file.Name()))
					}
				}
			}
		} else {
			if matches == nil || matches(file.Name()) {
				processFile(file, "")
			}
		}
	}
	return nil
}

func newRandomString(n int) string {
	for {
		s := RandomString(n)
		if !sessionIds.Contains(s) {
			sessionIds.Add(s)
			return s
		}
	}
}

func stringToShow(keyword string) Show {
	s := strings.Split(keyword, ",")
	showName := s[0]
	seasons := make(map[string]Season)
	if len(s) > 1 {
		for i := 1; i < len(s); i++ {
			ss := strings.Split(s[i], "|")
			var startEpisode *int
			seasonName := s[i]
			if len(ss) > 1 {
				se, _ := strconv.Atoi(ss[1])
				startEpisode = &se
				seasonName = ss[0]
			}
			if strings.ToLower(s[i]) == "specials" {
				seasons["Specials"] = Season{Name: "Specials", StartEpisode: startEpisode}
			} else {
				name := fmt.Sprintf("Season %s", seasonName)
				seasons[name] = Season{Name: name, StartEpisode: startEpisode}
			}
		}
	}
	return Show{Name: showName, Seasons: seasons}
}

func encodeShows(root string, shows []Show) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			for _, show := range shows {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(show.Name)) {
					fs, err := os.ReadDir(filepath.Join(root, file.Name()))
					if err != nil {
						discord.Errorf("error reading directory: %v", err)
						return
					}
					for _, f := range fs {
						p := func(matches func(s string) bool) {
							root := filepath.Join(root, file.Name(), f.Name())
							discord.Infof("Scanning %s", root)
							config.TheConfig.Input = root
							err := encode(matches)
							if err != nil {
								discord.Errorf("error: %v", err)
							}
						}
						if f.IsDir() && (re.MatchString(f.Name()) || f.Name() == "Specials") {
							if len(show.Seasons) > 0 {
								if season, ok := show.Seasons[f.Name()]; ok {
									if season.StartEpisode == nil {
										p(nil)
									} else {
										p(func(s string) bool {
											match := episodeRe.FindStringSubmatch(s)
											if match != nil && len(match) > 1 {
												currentEpisode, err := strconv.Atoi(match[1])
												if err != nil {
													return false
												}
												if currentEpisode >= *season.StartEpisode {
													return true
												}
											} else {
												discord.Infof("No episode number found")
											}
											return false
										})
									}
								}
							} else {
								p(nil)
							}
						}
					}
				}
			}
		}
	}
}

func encodeMovies(root string, movies []string) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			for _, keyword := range movies {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(keyword)) {
					root := filepath.Join(root, file.Name())
					discord.Infof("Processing %s", root)
					config.TheConfig.Input = root
					err = encode(nil)
					if err != nil {
						discord.Errorf("error: %v", err)
					}
				}
			}
		}
	}
}

type EncodeList struct {
	Shows  []string `json:"shows"`
	Movies []string `json:"movies"`
}

var showSet = mapset.NewSet[string]()
var movieSet = mapset.NewSet[string]()
var smMutex sync.Mutex

var re = regexp.MustCompile(`Season\s+\d+`)
var episodeRe = regexp.MustCompile(`S\d+E(\d+)`)

type Show struct {
	Name    string
	Seasons map[string]Season
}

type Season struct {
	Name         string
	StartEpisode *int
}

var sessionIds = mapset.NewSet[string]()

func process() {
	smMutex.Lock()
	defer smMutex.Unlock()
	shows := make([]Show, 0)
	sessionIds.Clear()
	jobs, err := jobsCache.Get(true)
	if err != nil {
		discord.Errorf("error getting all jobs: %v", err)
		return
	}
	for _, job := range jobs {
		sessionIds.Add(job.Id)
	}
	for _, keyword := range showSet.ToSlice() {
		show := stringToShow(keyword)
		discord.Infof(AsJson(show))
		shows = append(shows, show)
	}
	for _, root := range config.TheConfig.ShowDirs {
		encodeShows(root, shows)
	}
	for _, root := range config.TheConfig.MovieDirs {
		encodeMovies(root, movieSet.ToSlice())
	}
}

func main() {
	log.SetLevel(log.InfoLevel)
	config.Configure()
	discord.Init()
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	discord.Infof("Starting in %s mode", config.TheConfig.Mode)
	switch config.TheConfig.Mode {
	case config.EncodingMode:
		scheduler := gocron.NewScheduler(time.Now().Location())
		cleanup.AddOnStopFunc(func(_ os.Signal) {
			scheduler.Stop()
		})
		panicOnSec(scheduler.SingletonMode().Every(5).Minute().Do(func() {
			encodeList := EncodeList{}
			encodeListFile := config.TheConfig.EncodeListFile
			if _, err := os.Stat(encodeListFile); err == nil {
				content, err := os.ReadFile(encodeListFile)
				if err != nil {
					discord.Errorf("error reading file: %v", err)
				}
				err = json.Unmarshal(content, &encodeList)
				if err != nil {
					discord.Errorf("error unmarshalling file: %v", err)
				}
			}
			smMutex.Lock()
			currShows := mapset.NewSet[string](encodeList.Shows...)
			currMovies := mapset.NewSet[string](encodeList.Movies...)
			changed := false
			if !showSet.Equal(currShows) {
				showSet = currShows
				changed = true
			}
			if !movieSet.Equal(currMovies) {
				movieSet = currMovies
				changed = true
			}
			smMutex.Unlock()
			if changed {
				discord.Infof("List updated: %v", encodeList)
				process()
			}
		}))

		panicOnSec(scheduler.SingletonMode().Every(2).Hours().Do(func() {
			process()
		}))
		scheduler.StartAsync()
		<-blocking
	case config.RESTMode:
		REST()
	case config.CLEARMode:
		titleId := "harem"
		jobs, err := jobsCache.Get(true)
		if err != nil {
			discord.Errorf("error getting all jobs: %v", err)
			return
		}
		for _, job := range jobs {
			id := getTitleId(job.Input)
			if strings.Contains(id, titleId) {
				discord.Infof("File: %s, remove old, %s", OutputJoin(job.Id), job.Input)
				err := os.RemoveAll(OutputJoin(job.Id))
				if err != nil {
					discord.Errorf("error removing file: %v", err)
				}
			}
		}
	}
}

func panicOnSec(a interface{}, err error) {
	if err != nil {
		panic(err)
	}
}
