package main

import (
	"Sparkle/cleanup"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/redis/rueidis"

	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	HANDBRAKE = "./HandBrakeCLI"
	FFPROBE   = "./ffprobe"
	FFMPEG    = "/usr/local/bin/ffmpeg"
	OUTPUT    = "./output"
	Av1Preset = "8"
)

var ValidExtensions = []string{"mkv", "mp4", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "ts", "vob", "3gp", "3g2"}

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
}

func handbrakeScan(input string) error {
	cmd := exec.Command(HANDBRAKE, "--input", input, "--scan", "--json")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	strOut := string(out)

	strOut = strings.ReplaceAll(strOut, "Progress:", ",\"Progress\":")
	strOut = strings.ReplaceAll(strOut, "Version:", "\"Version\":")
	strOut = strings.ReplaceAll(strOut, "JSON Title Set:", ",\"JSON Title Set\":")
	strOut = strings.Replace(strOut, ",\"Progress\":", ",\"Progress\":[", 1)
	strOut = strings.Replace(strOut, ",\"JSON Title Set\":", "],\"JSON Title Set\":", 1)
	strOut = strings.ReplaceAll(strOut, ",\"Progress\": ", ",")

	strOut = "{" + strOut + "}"
	print(strOut)
	result := Result{}
	err = json.Unmarshal([]byte(strOut), &result)
	if err != nil {
		log.Fatalf("error decoding JSON: %v", err)
	}

	fmt.Printf("Decoded JSON: %+v\n", result)

	if len(result.JSONTitleSet.TitleList) == 0 {
		return fmt.Errorf("no titles found")
	}
	return nil
}

func extractStream(job Job, stream StreamInfo, streamType string) error {
	outputFile := fmt.Sprintf("%s/%d-%s", job.OutputPath, stream.Index, streamType)
	var cmd *exec.Cmd
	if streamType == "audio" {
		// "-profile:a", "aac_he_v2",
		cmd = exec.Command(FFMPEG, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), "-c:a", "libfdk_aac", "-vbr", "4", outputFile+".m4a")
	} else if streamType == "subtitle" {
		cmd = exec.Command(FFMPEG, "-i", job.Input, "-map", fmt.Sprintf("0:%d", stream.Index), outputFile+"_"+stream.Tags.Language+".vtt")
	} else {
		return fmt.Errorf("unsupported stream type: %s", streamType)
	}
	out, err := cmd.CombinedOutput()
	log.Debugf("output: %s", out)
	return err
}
func extractStreams(job Job) error {
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
		case "audio":
			log.Infof("Extracting audio stream #%d (%s)", stream.Index, stream.CodecName)
			err = extractStream(job, stream, "audio")
			if err != nil {
				return err
			}
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
	outputFile := fmt.Sprintf("%s/out.mkv", job.OutputPath)
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.Input, outputFile)
	cmd := exec.Command(
		HANDBRAKE,
		"-i", job.Input, // Input file
		"-o", outputFile, // Output file
		"--encoder", "svt_av1_10bit", // Use AV1 encoder
		"--vfr",           // Variable frame rate
		"--quality", "21", // Constant quality RF 22
		"--encoder-preset", Av1Preset, // Encoder preset
		"--audio", "none", // No audio tracks
		"--subtitle", "none", // No subtitles
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go printOutput(stdout)
	go printOutput(stderr)

	if err := cmd.Wait(); err != nil {
		fmt.Println("Error waiting for command execution:", err)
	}
	return nil
}

func printOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from pipe:", err)
	}
}

func calculateFileSHA256(filePath string) (string, error) {
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Errorf("error closing file: %v", err)
		}
	}(file)

	// Create a new SHA256 hash instance
	hash := sha256.New()

	// Copy the file content into the hash instance, computing the checksum as it reads
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Compute the final checksum and return it as a hexadecimal string
	checksum := hex.EncodeToString(hash.Sum(nil))
	return checksum, nil
}

func pipeline(inputFile string) error {
	job := Job{
		Id:          randomString(32),
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
	job.State = "incomplete"
	err = persistJob(job)
	if err != nil {
		return err
	}
	err = extractStreams(job)
	if err != nil {
		return err
	}
	job.State = "streams_extracted"
	err = persistJob(job)
	if err != nil {
		return err
	}
	err = convertVideoToSVTAV1(job)
	if err != nil {
		return err
	}
	job.State = "complete"
	err = persistJob(job)
	if err != nil {
		return err
	}
	return nil
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

var rdb rueidis.Client

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

func main() {
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
	log.SetLevel(log.WarnLevel)

	err = pipeline("./test2.mkv")
	if err != nil {
		log.Fatalf("error scanning input file: %v", err)
	}
}
