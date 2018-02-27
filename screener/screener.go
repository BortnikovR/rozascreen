package screener

import (
	"github.com/bortnikovr/go/rozascreen/config"
	"time"
	"crypto/tls"
	"net/http"
	"github.com/grafov/m3u8"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"log"
	"errors"
)

const (
	plName     = "index.m3u8"
	tempPrefix = "temp"
)

type Screener struct {
	config *config.Config
	client *http.Client
}

func NewScreener() (*Screener, error) {
	c, err := config.NewConfig()
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cl := &http.Client{Transport: tr}

	s := &Screener{config: c, client: cl}
	return s, nil
}

func (s *Screener) Run() {
	t := time.NewTicker(time.Second * s.config.Timeout)
	defer t.Stop()

	for {
		<-t.C
		go s.screen()
	}
}

func (s *Screener) screen() {
	for _, camID := range s.config.Cameras {
		go s.takeScreenshot(camID)
	}
}

func (s *Screener) takeScreenshot(camID string) {
	fn, err := s.getFilename(camID)
	if err != nil {
		log.Println("Can't get name of the video file: ", err)
		return
	}

	fn, err = s.getFile(camID, fn)
	if err != nil {
		log.Println("Can't get video file: ", err)
		return
	}

	go s.extractFrame(camID, fn)
}

func (s *Screener) getFilename(camID string) (string, error) {
	r, err := s.client.Get(fmt.Sprintf(s.config.UrlTemplate, camID) + plName)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf("Invalid status code: %d", r.StatusCode))
		return "", err
	}

	p, _, err := m3u8.DecodeFrom(r.Body, true)
	if err != nil {
		return "", err
	}
	m, ok := p.(*m3u8.MediaPlaylist)
	if !ok {
		return "", err
	}

	return m.Segments[0].URI, nil
}

func (s *Screener) getFile(camID, fn string) (string, error) {
	r, err := s.client.Get(fmt.Sprintf(s.config.UrlTemplate, camID) + fn)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("Invalid status code: %d", r.StatusCode))
	}

	err = os.MkdirAll(fmt.Sprintf("%s/%s", s.config.DirName, camID), os.ModePerm)
	if err != nil {
		return "", errors.New("Can't create path: " + err.Error())
	}

	filename := fmt.Sprintf("%s/%s/%s.ts", s.config.DirName, camID, tempPrefix)
	f, err := os.Create(filename)
	defer f.Close()
	if err != nil {
		return "", errors.New("Can't create file: " + err.Error())
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", errors.New("Can't read body: " + err.Error())
	}
	f.Write(b)

	return filename, nil
}

func (s *Screener) extractFrame(camID, fn string) {
	cmd := exec.Command("ffmpeg", "-i", fn, "-vframes", "1", "-f", "singlejpeg", "-")
	buffer, err := cmd.Output()
	if err != nil {
		log.Println("Could not generate frame: ", err)
		return
	}
	fname := fmt.Sprintf(
		"%s/%s/%s.jpeg", s.config.DirName, camID, time.Now().Format(time.RFC3339))

	f, err := os.Create(fname)
	if err != nil {
		log.Println("Can't create file: ", err)
		return
	}
	defer f.Close()
	f.Write(buffer)

	if s.config.CleanUp {
		go s.cleanUp(camID, fname)
	}
}

func (s *Screener) cleanUp(camID string, fn string) {
	files, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", s.config.DirName, camID))
	if err != nil {
		log.Println(err)
		return
	}

	for _, file := range files {
		path := fmt.Sprintf("%s/%s/%s", s.config.DirName, camID, file.Name())
		if path == fn {
			continue
		}
		os.Remove(path)
	}
}
