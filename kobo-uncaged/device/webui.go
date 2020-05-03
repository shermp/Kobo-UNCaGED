package device

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/unrolled/render"
)

func (k *Kobo) initWeb() {
	k.initRouter()
	k.initRender()
}

func (k *Kobo) initRouter() {
	k.mux = httprouter.New()
	k.mux.HandlerFunc("GET", "/", k.HandleIndex)
	k.mux.HandlerFunc("POST", "/start", k.HandleStart)
	k.mux.HandlerFunc("GET", "/main", k.HandleMain)
	k.mux.HandlerFunc("GET", "/messages", k.HandleMessages)
	k.mux.ServeFiles("/static/*filepath", http.Dir("./static"))
}

func (k *Kobo) initRender() {
	k.rend = render.New(render.Options{
		Directory:     "templates",
		Extensions:    []string{".tmpl"},
		IsDevelopment: true,
	})
}

// HandleIndex displays a form allowing the user to customize
// KU. It uses the existing ku.toml file as a seed
func (k *Kobo) HandleIndex(w http.ResponseWriter, r *http.Request) {
	k.rend.HTML(w, http.StatusOK, "indexPage", k)
}

// HandleStart parses the configuration form data
func (k *Kobo) HandleStart(w http.ResponseWriter, r *http.Request) {
	defer close(k.startChan)
	var err error
	res := webStartRes{}
	if err = r.ParseForm(); err != nil {
		res.err = err
		k.startChan <- res
		http.Error(w, "unable to parse config form", http.StatusInternalServerError)
	}
	if r.PostFormValue("PreferSDCard") != "" {
		res.opts.PreferSDCard = true
	}
	if r.PostFormValue("PreferKepub") != "" {
		res.opts.PreferKepub = true
	}
	if r.PostFormValue("EnableDebug") != "" {
		res.opts.EnableDebug = true
	}
	if r.PostFormValue("AddMetadataByTrigger") != "" {
		res.opts.AddMetadataByTrigger = true
	}
	res.opts.Thumbnail.GenerateLevel = r.PostFormValue("GenerateLevel")
	res.opts.Thumbnail.ResizeAlgorithm = r.PostFormValue("ResizeAlgorithm")
	res.opts.Thumbnail.JpegQuality, _ = strconv.Atoi(r.PostFormValue("JpegQuality"))
	if r.PostFormValue("updateConfig") != "" {
		res.saveOpts = true
	}
	k.startChan <- res
	http.Redirect(w, r, "/main", http.StatusSeeOther)
}

// HandleMain renders the main KU interface page
func (k *Kobo) HandleMain(w http.ResponseWriter, r *http.Request) {
	k.rend.HTML(w, http.StatusOK, "mainPage", k)
}

// HandleMessages sends messages to the client using server sent events.
func (k *Kobo) HandleMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "ResponseWriter not a flusher", http.StatusInternalServerError)
	}
	for {
		select {
		case msg := <-k.MsgChan:
			// Note, we replace all newlines in the message with spaces. That is because server
			// sent events are newline delimited
			if msg.Body != "" {
				fmt.Fprintf(w, "event: body\ndata: %s\n\n", strings.ReplaceAll(msg.Body, "\n", " "))
				f.Flush()
			}
			if msg.Footer != "" {
				fmt.Fprintf(w, "event: footer\ndata: %s\n\n", strings.ReplaceAll(msg.Footer, "\n", " "))
				f.Flush()
			}
			fmt.Fprintf(w, "event: progress\ndata: %d\n\n", msg.Progress)
			f.Flush()
			k.doneChan <- true
		case <-r.Context().Done():
			return
		}
	}
}