package albumd

import (
	"encoding/base64"
	"errors"
	"fmt"
	_ "image/jpeg" // for image.Decode
	_ "image/png"  // for image.Decode
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/CAFxX/httpcompression"
	"golang.org/x/crypto/scrypt"
)

type Server struct {
	AlbumPath     string
	ThumbPath     string
	Salt          []byte
	AdminUsername string
	AdminPassword string

	// cache of hashed album names to actual album names
	albumHashes  map[string]string
	templateChan chan *templateRequest

	thumbnailer *thumbnailer
}

type albumRenderRequest struct {
	AlbumName      string
	ServableImages []servableImage
}

type servableImage struct {
	ThumbPath    string
	OriginalPath string
	Description  string
}

type directRenderRequest struct {
	AlbumName       string
	HashedAlbumName string
	ImageName       string
	ImagePath       string
	Description     string
	PrevImage       string
	NextImage       string
}

func (this *Server) Run() {

	this.albumHashes = make(map[string]string)
	this.templateChan = make(chan *templateRequest)

	this.thumbnailer = newThumbnailer(this.AlbumPath, this.ThumbPath)

	// start four thumbnailers to speed things up
	for i := 0; i < 4; i++ {
		go this.thumbnailer.Run()
	}

	go runTemplater("./templates", this.templateChan)

	// make thumbnail cache path
	if this.ThumbPath != "" {
		err := os.MkdirAll(this.ThumbPath, 0755)
		if err != nil {
			panic(fmt.Sprintf("Error creating thumbnail directory: %v", err))
		}
	}

	// set up HTTP handlers
	compress, _ := httpcompression.DefaultAdapter()

	http.HandleFunc("/", this.serveIndex)
	http.Handle("/a/", compress(http.HandlerFunc(this.serveAlbum)))
	http.HandleFunc("/find/", this.serveFind)
	http.HandleFunc("/direct/", this.serveDirect)

	http.HandleFunc("/thumbs/", this.serveThumb)
	http.HandleFunc("/original/", this.serveOriginal)

	// block.
	http.ListenAndServe(":8080", nil)
}

func (this *Server) serveIndex(w http.ResponseWriter, r *http.Request) {

	req := &templateRequest{
		templateName: TMPL_INDEX,
		out:          w,
		done:         make(chan error),
	}
	renderHTTPTemplate(req, this.templateChan)
}

func (this *Server) serveAlbum(w http.ResponseWriter, r *http.Request) {

	var templateReq albumRenderRequest

	// get the album name from the URL
	incoming := r.URL.Path[len("/a/"):]
	if incoming == "" {
		http.Error(w, "No album name given", http.StatusBadRequest)
		return
	}

	// find the actual album name
	albumName, err, ok := this.findHashedAlbumName(incoming)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error finding album: %v", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Album not found", http.StatusNotFound)
		return
	}

	templateReq.ServableImages, err = this.findServableImages(albumName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error finding/creating thumbnails: %v", err), http.StatusInternalServerError)
		return
	}

	// find the desired plaintext name of the album
	nameBytes, err := os.ReadFile(fmt.Sprintf("%s/%s/.name", this.AlbumPath, albumName))
	if err == nil {
		templateReq.AlbumName = string(nameBytes)
	}

	// render
	req := &templateRequest{
		templateName: TMPL_ALBUM,
		content:      templateReq,
		out:          w,
		done:         make(chan error),
	}
	renderHTTPTemplate(req, this.templateChan)
}

func (this *Server) serveFind(w http.ResponseWriter, r *http.Request) {

	if !this.requireAuth(w, r) {
		return
	}

	incoming := r.URL.Path[len("/find/"):]
	if incoming == "" {
		http.Error(w, "No album name given", http.StatusBadRequest)
		return
	}

	albumName, err := this.hashAlbumName(incoming)
	if err != nil {
		http.Error(w, "Error hashing album name", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/a/%s", albumName), http.StatusSeeOther)
}

func (this *Server) serveDirect(w http.ResponseWriter, r *http.Request) {

	incoming := r.URL.Path[len("/direct/"):]
	if incoming == "" {
		http.Error(w, "No album name given", http.StatusBadRequest)
		return
	}

	// Parse the URL path: /direct/<hashedAlbumName>/<imageName>
	parts := strings.SplitN(incoming, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	hashedAlbumName := parts[0]
	imageName := parts[1]

	// Find the actual album name
	albumName, err, ok := this.findHashedAlbumName(hashedAlbumName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error finding album: %v", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Album not found", http.StatusNotFound)
		return
	}

	// Verify the image exists
	imagePath := fmt.Sprintf("%s/%s/%s", this.AlbumPath, albumName, imageName)
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Get all images in the album (sorted lexicographically)
	albumImages, err := this.getAlbumImages(albumName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading album: %v", err), http.StatusInternalServerError)
		return
	}

	// Find current image index and determine prev/next
	var currentIndex = -1
	for i, img := range albumImages {
		if img == imageName {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		http.Error(w, "Image not found in album", http.StatusNotFound)
		return
	}

	var templateReq directRenderRequest
	templateReq.ImageName = imageName
	templateReq.ImagePath = fmt.Sprintf("%s/%s", hashedAlbumName, imageName)
	templateReq.HashedAlbumName = hashedAlbumName

	// Set prev/next images
	if currentIndex > 0 {
		templateReq.PrevImage = albumImages[currentIndex-1]
	}
	if currentIndex < len(albumImages)-1 {
		templateReq.NextImage = albumImages[currentIndex+1]
	}

	// Read description if available
	descPath := fmt.Sprintf("%s/%s/%s.txt", this.AlbumPath, albumName, imageName)
	descBytes, err := os.ReadFile(descPath)
	if err == nil {
		templateReq.Description = string(descBytes)
	}

	// Find the desired plaintext name of the album
	nameBytes, err := os.ReadFile(fmt.Sprintf("%s/%s/.name", this.AlbumPath, albumName))
	if err == nil {
		templateReq.AlbumName = string(nameBytes)
	}

	// Render
	req := &templateRequest{
		templateName: TMPL_DIRECT,
		content:      templateReq,
		out:          w,
		done:         make(chan error),
	}
	renderHTTPTemplate(req, this.templateChan)
}

func (this *Server) serveThumb(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Path[len("/thumbs/"):]
	hashedAlbumName := requestedPath[:strings.Index(requestedPath, "/")]
	imageName := requestedPath[strings.Index(requestedPath, "/")+1:]

	albumName, err, found := this.findHashedAlbumName(hashedAlbumName)
	if err != nil {
		http.Error(w, "Thumb not findable", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Album not found", http.StatusNotFound)
		return
	}

	thumbPath := fmt.Sprintf("%s/%s/%s.png", this.ThumbPath, albumName, imageName)
	http.ServeFile(w, r, thumbPath)
}

func (this *Server) serveOriginal(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Path[len("/original/"):]
	hashedAlbumName := requestedPath[:strings.Index(requestedPath, "/")]
	imageName := requestedPath[strings.Index(requestedPath, "/")+1:]

	albumName, err, found := this.findHashedAlbumName(hashedAlbumName)
	if err != nil {
		http.Error(w, "Original not findable", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "Album not found", http.StatusNotFound)
		return
	}

	imagePath := fmt.Sprintf("%s/%s/%s", this.AlbumPath, albumName, imageName)
	http.ServeFile(w, r, imagePath)
}

// Finds/creates thumbs for every image in the given albumName.
// returns a list of paths to the thumbnails.
func (this *Server) findServableImages(albumName string) ([]servableImage, error) {

	var servableImages []servableImage

	hashedAlbumName, err := this.hashAlbumName(albumName)
	if err != nil {
		msg := fmt.Sprintf("Error hashing album name: %v", err)
		return servableImages, errors.New(msg)
	}

	albumItems, err := os.ReadDir(fmt.Sprintf("%s/%s", this.AlbumPath, albumName))
	if err != nil {
		msg := fmt.Sprintf("Error reading album directory: %v", err)
		return servableImages, errors.New(msg)
	}

	for _, item := range albumItems {
		if item.IsDir() {
			continue
		}
		if !this.isImageFile(item.Name()) {
			continue
		}

		_, err := this.thumbnailer.RequestThumbnail(albumName, item.Name())
		if err != nil {
			msg := fmt.Sprintf("Error creating thumbnail: %v", err)
			return servableImages, errors.New(msg)
		}

		// if there's a .txt file with the same name, read it for description
		description := ""
		descPath := fmt.Sprintf("%s/%s/%s.txt", this.AlbumPath, albumName, item.Name())
		descBytes, err := os.ReadFile(descPath)
		if err == nil {
			description = string(descBytes)
		}

		servableImages = append(servableImages, servableImage{
			ThumbPath:    fmt.Sprintf("%s/%s", hashedAlbumName, item.Name()),
			OriginalPath: fmt.Sprintf("%s/%s", hashedAlbumName, item.Name()),
			Description:  description,
		})
	}

	return servableImages, nil
}

func (this Server) isImageFile(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, ".jpg") ||
		strings.HasSuffix(name, ".jpeg") ||
		strings.HasSuffix(name, ".png")
}

// getAlbumImages returns a sorted list of image filenames in the album
func (this *Server) getAlbumImages(albumName string) ([]string, error) {
	var images []string

	albumItems, err := os.ReadDir(fmt.Sprintf("%s/%s", this.AlbumPath, albumName))
	if err != nil {
		return images, err
	}

	for _, item := range albumItems {
		if item.IsDir() {
			continue
		}
		if !this.isImageFile(item.Name()) {
			continue
		}
		images = append(images, item.Name())
	}

	// Sort images lexicographically for consistent navigation
	sort.Strings(images)

	return images, nil
}

// returns true if processing should continue, false otherwise.
func (this Server) requireAuth(response http.ResponseWriter, request *http.Request) bool {

	if this.AdminUsername == "" || this.AdminPassword == "" {
		return true
	}

	username, password, err := parseAuth(request)
	if err != nil {
		response.Header().Set("WWW-Authenticate", `Basic realm="kuratoro"`)
		http.Error(response, err.Error(), 401)
		return false
	}

	if username != this.AdminUsername || password != this.AdminPassword {
		response.Header().Set("WWW-Authenticate", `Basic realm="kuratoro"`)
		http.Error(response, "Not authorized", 401)
		return false
	}

	return true
}

func parseAuth(r *http.Request) (user string, password string, _ error) {

	username, password, ok := r.BasicAuth()
	if !ok {
		return "", "", errors.New("Basic auth must be provided")
	}

	return username, password, nil
}

// finds (and then caches) the actual album name given the incoming hashed/base64'd name.
func (this *Server) findHashedAlbumName(incoming string) (string, error, bool) {

	unhashed, ok := this.albumHashes[incoming]
	if ok {
		return unhashed, nil, true
	}

	// not in cache, so brute search every album.
	// find every directory under AlbumPath
	entries, err := os.ReadDir(this.AlbumPath)
	if err != nil {
		return "", err, false
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		hashed, err := this.hashAlbumName(e.Name())
		if err != nil {
			return "", err, false
		}

		if incoming == hashed {
			// cache it
			this.albumHashes[incoming] = e.Name()
			return e.Name(), nil, true
		}
	}

	return "", nil, false
}

// returns a hashed+base64 album name for the given actual name.
func (this Server) hashAlbumName(name string) (string, error) {

	hashed, err := scrypt.Key([]byte(name), this.Salt, 16384, 8, 1, 32)
	b64 := base64.RawURLEncoding.EncodeToString(hashed)
	return b64, err
}
