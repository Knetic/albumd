package albumd

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"

	"golang.org/x/crypto/scrypt"
)

type Server struct {
	AlbumPath     string
	Salt          []byte
	AdminUsername string
	AdminPassword string

	// cache of hashed album names to actual album names
	albumHashes  map[string]string
	templateChan chan *templateRequest
}

type albumRenderRequest struct {
	AlbumName string
}

func (this *Server) Run() {

	this.albumHashes = make(map[string]string)
	this.templateChan = make(chan *templateRequest)

	go runTemplater("./templates", this.templateChan)

	http.HandleFunc("/", this.serveIndex)
	http.HandleFunc("/a/", this.serveAlbum)
	http.HandleFunc("/find/", this.serveFind)

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

	// get the album name from the URL
	incoming := r.URL.Path[len("/a/"):]
	if incoming == "" {
		http.Error(w, "No album name given", http.StatusBadRequest)
		return
	}

	// find the actual album name
	albumName, err, ok := this.findHashedAlbumName(incoming)
	if err != nil {
		http.Error(w, "Error finding album", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Album not found", http.StatusNotFound)
	}

	var templateReq albumRenderRequest
	templateReq.AlbumName = albumName

	// render album
	req := &templateRequest{
		templateName: TMPL_ALBUM,
		content:      templateReq,
		out:          w,
		done:         make(chan error),
	}
	renderHTTPTemplate(req, this.templateChan)
}

func (this *Server) serveFind(w http.ResponseWriter, r *http.Request) {
	incoming := r.URL.Path[len("/a/"):]
	if incoming == "" {
		http.Error(w, "No album name given", http.StatusBadRequest)
		return
	}

	albumName, err := this.hashAlbumName(incoming)
	if err != nil {
		http.Error(w, "Error hashing album name", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(albumName))
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
	entries, err := os.ReadDir("./")
	if err != nil {
		return "", err, false
	}

	incomingB64, err := base64.RawURLEncoding.DecodeString(incoming)
	if err != nil {
		return "", err, false
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		hashed, err := this.hashAlbumName(e.Name())
		if err != nil {
			continue
		}

		if string(incomingB64) == hashed {
			// cache it
			this.albumHashes[incoming] = e.Name()
			return e.Name(), nil, true
		}
	}

	return "", nil, false
}

// returns a hashed album name for the given actual name.
func (this Server) hashAlbumName(name string) (string, error) {

	hashed, err := scrypt.Key([]byte(name), this.Salt, 16384, 8, 1, 32)
	b64 := base64.RawURLEncoding.EncodeToString(hashed)
	return b64, err
}
