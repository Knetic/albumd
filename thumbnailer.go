package albumd

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"os"

	"github.com/disintegration/imageorient"
	"github.com/nfnt/resize"
)

type thumbnailer struct {
	ThumbPath string
	AlbumPath string

	thumbRequests chan *thumbRequest
}

type thumbRequest struct {
	albumName    string
	imageName    string
	outThumbPath chan string
	outError     chan error
}

func newThumbnailer(albumPath string, thumbPath string) *thumbnailer {
	t := &thumbnailer{
		AlbumPath:     albumPath,
		ThumbPath:     thumbPath,
		thumbRequests: make(chan *thumbRequest),
	}
	return t
}

func (this *thumbnailer) Run() {
	for req := range this.thumbRequests {

		thumbPath := this.deriveThumbPath(req.albumName, req.imageName)

		// thumb path exists?
		_, err := os.Stat(thumbPath)
		if os.IsNotExist(err) {
			_, err := this.createThumbnail(req.albumName, req.imageName)
			if err != nil {
				req.outError <- err
				return
			}
		}

		req.outThumbPath <- thumbPath
	}
}

func (this *thumbnailer) RequestThumbnail(albumName string, imageName string) (string, error) {

	outThumbPath := make(chan string)
	outError := make(chan error)
	defer close(outThumbPath)
	defer close(outError)

	this.thumbRequests <- &thumbRequest{
		albumName:    albumName,
		imageName:    imageName,
		outThumbPath: outThumbPath,
		outError:     outError,
	}

	// await response.
	select {
	case thumbPath := <-outThumbPath:
		return thumbPath, nil
	case err := <-outError:
		return "", err
	}
}

func (this thumbnailer) createThumbnail(albumName string, imageName string) (string, error) {

	// create dir for album, if needed
	err := os.MkdirAll(fmt.Sprintf("%s/%s", this.ThumbPath, albumName), 0755)
	if err != nil {
		msg := fmt.Sprintf("Error creating thumbnail directory: %v", err)
		return "", errors.New(msg)
	}

	// resize thumbnail
	thumbPath := this.deriveThumbPath(albumName, imageName)
	originalImagePath := fmt.Sprintf("%s/%s/%s", this.AlbumPath, albumName, imageName)

	originalImageF, err := os.Open(originalImagePath)
	if err != nil {
		msg := fmt.Sprintf("Error reading image file for resizing: %v", err)
		return "", errors.New(msg)
	}
	defer originalImageF.Close()

	originalImage, _, err := imageorient.Decode(originalImageF)
	if err != nil {
		msg := fmt.Sprintf("Error decoding image for resizing: %v (%s)", err, originalImagePath)
		return "", errors.New(msg)
	}

	newImage := resize.Resize(160, 0, originalImage, resize.Lanczos3)

	// write out
	var thumbBuffer bytes.Buffer
	err = png.Encode(&thumbBuffer, newImage)
	if err != nil {
		msg := fmt.Sprintf("Error encoding thumbnail image: %v", err)
		return "", errors.New(msg)
	}

	err = os.WriteFile(thumbPath, thumbBuffer.Bytes(), 0644)
	if err != nil {
		msg := fmt.Sprintf("Error writing thumbnail image: %v", err)
		return "", errors.New(msg)
	}

	return thumbPath, nil
}

func (this thumbnailer) deriveThumbPath(albumName string, imageName string) string {
	return fmt.Sprintf("%s/%s/%s.png", this.ThumbPath, albumName, imageName)
}
