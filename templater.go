package albumd

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"text/template"
)

type templateRequest struct {
	templateName string
	content      interface{}
	out          io.Writer
	done         chan error
}

type siteContent struct {
	Stylesheet string
	Content    interface{}
}

const TMPL_INDEX = "index.tmpl"
const TMPL_ALBUM = "album.tmpl"
const TMPL_DIRECT = "direct.tmpl"

func runTemplater(templatePath string, in chan *templateRequest) {

	defer close(in)

	templates := map[string]*template.Template{
		TMPL_INDEX:  template.Must(template.ParseFiles(filepath.Join(templatePath, "index.tmpl"))),
		TMPL_ALBUM:  template.Must(template.ParseFiles(filepath.Join(templatePath, "album.tmpl"))),
		TMPL_DIRECT: template.Must(template.ParseFiles(filepath.Join(templatePath, "direct.tmpl"))),
	}

	stylesheetRaw, err := ioutil.ReadFile(filepath.Join(templatePath, "style.css"))
	if err != nil {
		return
	}

	stylesheet := string(stylesheetRaw)

	for req := range in {

		template, ok := templates[req.templateName]
		if !ok {
			req.done <- errors.New("No such template")
			continue
		}

		content := siteContent{
			Content:    req.content,
			Stylesheet: stylesheet,
		}

		err := template.Execute(req.out, content)
		req.done <- err
	}
}

// convenience function to render the given template request as an http response.
// functionally this is the same as any other submission to the channel, except it writes http 5xx errors if encountered.
func renderHTTPTemplate(req *templateRequest, in chan *templateRequest) {

	in <- req

	err := <-req.done
	if err != nil {
		httpOut := req.out.(http.ResponseWriter)
		http.Error(httpOut, "Error rendering template", http.StatusInternalServerError)
		return
	}
}
