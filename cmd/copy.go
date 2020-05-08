/*
Copyright © 2020 reeve0930 <reeve0930@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/reeve0930/foltia/db"
	"github.com/spf13/cobra"
)

type fileCopyInfo struct {
	tid      int
	title    string
	epNum    int
	epTitle  string
	pid      int
	srcname  string
	dstname  string
	station  string
	scramble bool
}

var dbt = []string{"NHK総合", "NHK Eテレ", "フジテレビ", "日本テレビ", "TBS", "テレビ朝日", "テレビ東京", "tvk", "チバテレビ", "TOKYO MX", "TOKYO MX2"}
var dbs = []string{"NHK-BS1", "NHK-BS2", "BSテレ東", "BS-TBS", "BSフジ", "BS朝日", "BS日テレ", "BS11イレブン", "BS12トゥエルビ", "NHK BSプレミアム", "Dlife"}

// copyCmd represents the copy command
var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy the video files from foltia ANIME LOCKER",
	Run: func(cmd *cobra.Command, args []string) {
		copyFunc(cmd)
	},
}

func init() {
	rootCmd.AddCommand(copyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// copyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	copyCmd.Flags().BoolP("list", "l", false, "Show the video files that will be copied")
}

func copyFunc(cmd *cobra.Command) {
	list, err := cmd.Flags().GetBool("list")
	if err != nil {
		log.Fatalln(err)
	}
	if list {
		err = showCopyList()
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		err = copyFiles()
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func showCopyList() error {
	fcil, err := getCopyList()
	if err != nil {
		return err
	}
	showList(fcil)
	log.Printf("Found %d video files", len(fcil))
	return nil
}

func copyFiles() error {
	log.Println("Start file copy")
	fcil, err := getCopyList()
	if err != nil {
		return err
	}
	ep, err := db.GetAllEpisode()
	if err != nil {
		return err
	}
	for i, f := range fcil {
		log.Printf("[%d/%d] %s (%d:%s)", i+1, len(fcil), f.title, f.epNum, f.epTitle)
		if f.scramble {
            log.Println("This file is scrambled")
			f.dstname = "[S]" + f.dstname
		}
		src := filepath.Join(conf.fPath, f.srcname)
		dst := filepath.Join(conf.cDest, f.dstname)
		err = copyVideoFile(src, dst)
		if err != nil {
			return err
		}
		for _, e := range ep {
			if e.TID == f.tid && e.EpNum == f.epNum {
				db.UpdateEpisode(e.ID, e.TID, e.EpNum, e.EpTitle, true)
				break
			}
		}
	}
	log.Println("Fished file copy")
	return nil
}

func copyVideoFile(src string, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	sourceStat, err := s.Stat()
	if err != nil {
		return err
	}
	srcSize := sourceStat.Size()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	bar := pb.New64(srcSize).SetTemplateString(barTemp).Start()
	reader := bar.NewProxyReader(s)

	io.Copy(d, reader)

	bar.Finish()

	return nil
}

func getCopyList() ([]fileCopyInfo, error) {
	title, err := db.GetAllTitle()
	if err != nil {
		return []fileCopyInfo{}, err
	}
	episode, err := db.GetAllEpisode()
	if err != nil {
		return []fileCopyInfo{}, err
	}
	videofile, err := db.GetAllVideoFile()
	if err != nil {
		return []fileCopyInfo{}, err
	}
	var fcil []fileCopyInfo
	for _, t := range title {
		for _, e := range episode {
			if e.CopyStatus {
				continue
			}
			if t.TID == e.TID {
				var f fileCopyInfo
				f.tid = t.TID
				f.title = t.Title
				f.epNum = e.EpNum
				f.epTitle = e.EpTitle
				f.dstname = conf.cFilename
				f.dstname = strings.Replace(f.dstname, "%title%", t.Title, -1)
				f.dstname = strings.Replace(f.dstname, "%epnum%", fmt.Sprintf("%02d", e.EpNum), -1)
				f.dstname = strings.Replace(f.dstname, "%eptitle%", e.EpTitle, -1)
				if conf.cFiletype == "TS" {
					f.dstname = f.dstname + ".m2t"
				} else if conf.cFiletype == "MP4" {
					f.dstname = f.dstname + ".mp4"
				} else {
					return []fileCopyInfo{}, fmt.Errorf("Please check config : copy_filetype")
				}

				nonDropExists := false
				fileExists := false
				for _, v := range videofile {
					if e.TID == v.TID && e.EpNum == v.EpNum {
						fileExists = true
						if v.Drop < conf.cDropThresh {
							nonDropExists = true
							if f.srcname == "" {
								name, check := getSrcname(v)
								if check {
									f.srcname = name
									f.pid = v.PID
									f.station = v.Station
									if v.Scramble != 0 {
										f.scramble = true
									} else {
										f.scramble = false
									}
								} else {
									continue
								}
							} else {
								p1, err := getStationPriority(f.station)
								if err != nil {
									return []fileCopyInfo{}, err
								}
								p2, err := getStationPriority(v.Station)
								if err != nil {
									return []fileCopyInfo{}, err
								}
								if p2 > p1 {
									name, check := getSrcname(v)
									if check {
										f.srcname = name
										f.pid = v.PID
										f.station = v.Station
										if v.Scramble != 0 {
											f.scramble = true
										} else {
											f.scramble = false
										}
									}
								}
							}
						}
					}
				}
				if f.srcname != "" {
					fcil = append(fcil, f)
				} else {
					if !nonDropExists && fileExists {
						log.Printf("Found a lot of dropped TS packets : %s (%d:%s)", t.Title, e.EpNum, e.EpTitle)
					}
				}
			}
		}
	}
	return fcil, nil
}

func getStationPriority(station string) (int, error) {
	for _, s := range dbt {
		if station == s {
			return 0, nil
		}
	}
	for _, s := range dbs {
		if station == s {
			return 1, nil
		}
	}
	return -1, fmt.Errorf("Can not get staion : %s", station)
}

func getSrcname(v db.VideoFile) (string, bool) {
	if conf.cFiletype == "TS" {
		if v.FileTS != "" {
			return v.FileTS, true
		}
		return "", false
	}
	if v.FileMP4SD != "" {
		if v.FileMP4HD != "" {
			return v.FileMP4HD, true
		}
		return v.FileMP4SD, true
	}
	return "", false
}

func showList(fcil []fileCopyInfo) {
	for _, f := range fcil {
		fmt.Printf("%d : %s\n", f.pid, f.dstname)
	}
}