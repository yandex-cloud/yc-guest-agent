package goreleaser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type Artifact struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Goos         string `json:"goos,omitempty"`
	Goarch       string `json:"goarch,omitempty"`
	Goamd64      string `json:"goamd64,omitempty"`
	InternalType int    `json:"internal_type"`
	Type         string `json:"type"`
	Extra        struct {
		Binary   string   `json:"Binary,omitempty"`
		Ext      string   `json:"Ext,omitempty"`
		ID       string   `json:"ID,omitempty"`
		Binaries []string `json:"Binaries,omitempty"`
		Builds   []struct {
			Name         string `json:"name"`
			Path         string `json:"path"`
			Goos         string `json:"goos"`
			Goarch       string `json:"goarch"`
			Goamd64      string `json:"goamd64"`
			InternalType int    `json:"internal_type"`
			Type         string `json:"type"`
			Extra        struct {
				Binary string `json:"Binary"`
				Ext    string `json:"Ext"`
				ID     string `json:"ID"`
			} `json:"extra"`
		} `json:"Builds,omitempty"`
		Checksum  string      `json:"Checksum,omitempty"`
		Format    string      `json:"Format,omitempty"`
		Replaces  interface{} `json:"Replaces"`
		WrappedIn string      `json:"WrappedIn,omitempty"`
		Files     []struct {
			Src      string `json:"src"`
			Dst      string `json:"dst"`
			Type     string `json:"type,omitempty"`
			FileInfo struct {
				Mode  int       `json:"mode"`
				Mtime time.Time `json:"mtime"`
			} `json:"file_info,omitempty"`
		} `json:"Files,omitempty"`
	} `json:"extra"`
}

const Checksum = "Checksum"
const Archive = "Archive"
const Binary = "Binary"
const LinuxPackage = "Linux Package"

func ParseArtefacts(filename string) []Artifact {
	jsonFile, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	var result []Artifact
	err = json.Unmarshal([]byte(byteValue), &result)
	if err != nil {
		return nil
	}

	return result
}

func FindLinuxPackage(artifacts []Artifact) *Artifact {
	for _, a := range artifacts {
		if a.Type == LinuxPackage {
			return &a
		}
	}
	return nil
}
