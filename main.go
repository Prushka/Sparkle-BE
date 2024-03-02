package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strings"
)

const (
	HANDBRAKE = "./HandBrakeCLI"
	FFPROBE   = "./ffprobe"
	FFMPEG    = "/usr/local/bin/ffmpeg"
	INPUT     = "./test2.mkv"
	OUTPUT    = "./output"
)

type Job struct {
	id            string
	fileRawPath   string
	fileRawFolder string
	fileRawName   string
	fileRawExt    string
	input         string
	outputPath    string
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
	outputFile := fmt.Sprintf("%s/%d-%s", job.outputPath, stream.Index, streamType)
	var cmd *exec.Cmd
	if streamType == "audio" {
		// "-profile:a", "aac_he_v2",
		cmd = exec.Command(FFMPEG, "-i", job.input, "-map", fmt.Sprintf("0:%d", stream.Index), "-c:a", "libfdk_aac", "-vbr", "4", outputFile+".m4a")
	} else if streamType == "subtitle" {
		cmd = exec.Command(FFMPEG, "-i", job.input, "-map", fmt.Sprintf("0:%d", stream.Index), outputFile+"_"+stream.Tags.Language+".vtt")
	} else {
		return fmt.Errorf("unsupported stream type: %s", streamType)
	}
	out, err := cmd.CombinedOutput()
	log.Debugf("output: %s", out)
	return err
}
func extractStreams(job Job) error {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", job.input)
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
	outputFile := fmt.Sprintf("%s/out.mkv", job.outputPath)
	log.Infof("Converting video to SVT-AV1-10Bit: %s -> %s", job.input, outputFile)
	cmd := exec.Command(
		HANDBRAKE,
		"-i", job.input, // Input file
		"-o", outputFile, // Output file
		"--encoder", "svt_av1_10bit", // Use AV1 encoder
		"--vfr",           // Variable frame rate
		"--quality", "21", // Constant quality RF 22
		"--encoder-preset", "4", // Encoder preset
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

// printOutput reads from the given reader (representing either stdout or stderr) and prints the output line by line
func printOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from pipe:", err)
	}
}

func pipeline(inputFile string) error {
	job := Job{
		id:          randomString(8),
		fileRawPath: "./test2.mkv",
	}
	s := strings.Split(job.fileRawPath, "/")
	file := s[len(s)-1]
	job.fileRawFolder = strings.Join(s[:len(s)-1], "/")
	s = strings.Split(file, ".")
	job.fileRawName = s[0]
	job.fileRawExt = s[1]
	job.input = fmt.Sprintf("%s/%s.%s", job.fileRawFolder, job.id, job.fileRawExt)
	err := os.Rename(job.fileRawPath, job.input)
	if err != nil {
		return err
	}
	job.outputPath = fmt.Sprintf("%s/%s", OUTPUT, job.id)
	log.Infof("Processing Job: %+v", job)
	err = os.MkdirAll(job.outputPath, 0755)
	if err != nil {
		return err
	}
	err = extractStreams(job)
	if err != nil {
		return err
	}
	err = convertVideoToSVTAV1(job)
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

func main() {
	log.SetLevel(log.WarnLevel)

	err := pipeline(INPUT)
	if err != nil {
		log.Fatalf("error scanning input file: %v", err)
	}
}
