package target

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/overseerr"
	"Sparkle/utils"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

func loop(matches func(s string) bool, te ToEncode, runner func(file os.DirEntry, parent string, te ToEncode) bool) error {
	files, err := os.ReadDir(config.TheConfig.Input)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			fs, err := os.ReadDir(utils.InputJoin(file.Name()))
			if err != nil {
				return err
			}
			for _, f := range fs {
				if matches == nil || matches(f.Name()) {
					runner(f, file.Name(), te)
				}
			}
		} else {
			if matches == nil || matches(file.Name()) {
				runner(file, "", te)
			}
		}
	}
	return nil
}

func LoopShows(root string, shows []Show, runner func(file os.DirEntry, parent string, te ToEncode) bool) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, show := range shows {
		for _, file := range files {
			if file.IsDir() {
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
							err := loop(matches, show.ToEncode, runner)
							if err != nil {
								discord.Errorf("error: %v", err)
							}
						}
						if f.IsDir() && (SeasonRe.MatchString(f.Name()) || f.Name() == "Specials") {
							if len(show.Seasons) > 0 {
								if season, ok := show.Seasons[f.Name()]; ok {
									if season.StartEpisode != nil {
										p(func(s string) bool {
											match := SeasonEpisodeRe.FindStringSubmatch(s)
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
									} else if season.ExactEpisode != nil {
										p(func(s string) bool {
											match := SeasonEpisodeRe.FindStringSubmatch(s)
											if match != nil && len(match) > 1 {
												currentEpisode, err := strconv.Atoi(match[1])
												if err != nil {
													return false
												}
												if currentEpisode == *season.ExactEpisode {
													return true
												}
											} else {
												discord.Infof("No episode number found")
											}
											return false
										})
									} else {
										p(nil)
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

func LoopMovies(root string, movies []Movie, runner func(file os.DirEntry, parent string, te ToEncode) bool) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, movie := range movies {
		for _, file := range files {
			if file.IsDir() {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(movie.Name)) {
					root := filepath.Join(root, file.Name())
					discord.Infof("Processing %s", root)
					config.TheConfig.Input = root
					err = loop(nil, movie.ToEncode, runner)
					if err != nil {
						discord.Errorf("error: %v", err)
					}
				}
			}
		}
	}
}

// "DAN DA DAN,1|3" means starting from season 1, episode 3, it stops at season 1 and doesn't encode seasons > 1
// "DAN DA DAN,1:3" means only process season 1, episode 3
// "f:DAN DA DAN,1|6,2:t" means season 1, episode 6 to end, then season 2, with fast encoding and subtitle translation

func StringToShow(keyword string) Show {
	s := strings.Split(keyword, ",")
	showName := s[0]
	seasons := make(map[string]Season)
	if len(s) > 1 {
		for i := 1; i < len(s); i++ {
			var startEpisode *int
			var exactEpisode *int
			seasonName := s[i]
			ss := strings.Split(seasonName, "|")
			if len(ss) > 1 {
				se, _ := strconv.Atoi(ss[1])
				startEpisode = &se
				seasonName = ss[0]
			}

			exact := strings.Split(seasonName, ":")
			if len(exact) > 1 {
				ee, _ := strconv.Atoi(exact[1])
				exactEpisode = &ee
				seasonName = exact[0]
			}
			if strings.ToLower(s[i]) == "specials" {
				seasons["Specials"] = Season{Name: "Specials", StartEpisode: startEpisode, ExactEpisode: exactEpisode}
			} else {
				name := fmt.Sprintf("Season %s", seasonName)
				seasons[name] = Season{Name: name, StartEpisode: startEpisode, ExactEpisode: exactEpisode}
			}
		}
	}
	return Show{Name: showName, Seasons: seasons}
}

type EncodeList struct {
	Shows  []string `json:"shows"`
	Movies []string `json:"movies"`
}

var Shows []string
var Movies []string
var SMMutex sync.Mutex

var SeasonRe = regexp.MustCompile(`Season\s+\d+`)
var SeasonEpisodeRe = regexp.MustCompile(`S\d+E(\d+)`)

type ToEncode struct {
	Fast      bool
	Translate bool
}

type Movie struct {
	Name string
	ToEncode
}

type Show struct {
	Name    string
	Seasons map[string]Season
	ToEncode
}

type Season struct {
	Name         string
	StartEpisode *int
	ExactEpisode *int
}

var SessionIds = mapset.NewSet[string]()

func NewRandomString(n int) string {
	for {
		s := utils.RandomString(n)
		if !SessionIds.Contains(s) {
			SessionIds.Add(s)
			return s
		}
	}
}

func UpdateEncoderList() bool {
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
	if config.TheConfig.OverSeerrAPI != "" && len(config.TheConfig.OverSeerrUserIds) > 0 {
		discord.Infof("Appending overseerr requests")
		for _, userId := range config.TheConfig.OverSeerrUserIds {
			responses, err := overseerr.GetUserRequests(userId)
			if err != nil {
				discord.Errorf("Error getting user requests: %v, user id: %d", err, userId)
			}
			discord.Infof("Found %d requests from user id: %d", len(responses.Results), userId)
			for _, req := range responses.Results {
				title, err := overseerr.GetTitleById(req.Type, req.Media.TMDBID)
				if err != nil {
					discord.Errorf("Error getting title: %v, id: %d", err, req.Media.TMDBID)
				}
				switch req.Type {
				case "movie":
					encodeList.Movies = append(encodeList.Movies, title)
				case "tv":
					encodeList.Shows = append(encodeList.Shows, title)
				}
			}
		}
	}
	encodeList.Movies = utils.RemoveEmptyStrings(utils.UniqueStrings(encodeList.Movies))
	encodeList.Shows = utils.RemoveEmptyStrings(utils.UniqueStrings(encodeList.Shows))
	SMMutex.Lock()
	changed := false
	if !utils.SlicesSetEqual(Shows, encodeList.Shows) {
		Shows = encodeList.Shows
		changed = true
	}
	if !utils.SlicesSetEqual(Movies, encodeList.Movies) {
		Movies = encodeList.Movies
		changed = true
	}
	SMMutex.Unlock()
	if changed {
		discord.Infof("List updated")
		fmt.Println(utils.AsJson(encodeList))
	}
	return changed
}
