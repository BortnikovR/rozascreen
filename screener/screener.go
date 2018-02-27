package screener

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/bortnikovr/rozascreen/config"
	"github.com/grafov/m3u8"
)

const plName = "index.m3u8"

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

	data, err := s.getData(camID, fn)
	if err != nil {
		log.Println("Can't get video file: ", err)
		return
	}

	go s.extractFrame(camID, data)
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
		return "", errors.New("wrong playlist type")
	}

	return m.Segments[0].URI, nil
}

func (s *Screener) getData(camID, fn string) ([]byte, error) {
	r, err := s.client.Get(fmt.Sprintf(s.config.UrlTemplate, camID) + fn)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Invalid status code: %d", r.StatusCode))
	}

	err = os.MkdirAll(fmt.Sprintf("%s/%s", s.config.DirName, camID), os.ModePerm)
	if err != nil {
		return nil, errors.New("Can't create path: " + err.Error())
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.New("Can't read body: " + err.Error())
	}

	return b, nil
}

func (s *Screener) extractFrame(camID string, data []byte) {
	cmd := exec.Command("ffmpeg", "-i", "-", "-vframes", "1", "-f", "singlejpeg", "-")
	pipe, err := cmd.StdinPipe()
	if err != nil {
		log.Println("Can't create stdin pipe: ", err)
		return
	}

	go func() {
		defer pipe.Close()
		pipe.Write(data)
	}()

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
		s.cleanUp(camID)
	}
}

func (s *Screener) cleanUp(camID string) {
	files, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", s.config.DirName, camID))
	if err != nil {
		log.Println(err)
		return
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	for i := 0; i < len(files)-1; i++ {
		path := fmt.Sprintf("%s/%s/%s", s.config.DirName, camID, files[i].Name())
		os.Remove(path)
	}
}
