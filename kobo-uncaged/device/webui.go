package device

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/shermp/UNCaGED/uc"
	"github.com/unrolled/render"
)

func (k *Kobo) initWeb() {
	k.initRouter()
	k.initRender()
}

func (k *Kobo) initRouter() {
	k.mux = httprouter.New()
	k.mux.HandlerFunc("GET", "/", k.HandleIndex)
	k.mux.HandlerFunc("GET", "/config", k.HandleConfig)
	k.mux.HandlerFunc("POST", "/config", k.HandleConfig)
	k.webInfo.ConfigPath = "/config"
	k.mux.HandlerFunc("GET", "/exit", k.HandleExit)
	k.webInfo.ExitPath = "/exit"
	k.mux.HandlerFunc("GET", "/messages", k.HandleMessages)
	k.webInfo.SSEPath = "/messages"
	k.mux.HandlerFunc("GET", "/calibreauth", k.HandleCalAuth)
	k.mux.HandlerFunc("POST", "/calibreauth", k.HandleCalAuth)
	k.webInfo.AuthPath = "/calibreauth"
	k.mux.HandlerFunc("GET", "/calibreinstance", k.HandleCalInstances)
	k.mux.HandlerFunc("POST", "/calibreinstance", k.HandleCalInstances)
	k.webInfo.InstancePath = "/calibreinstance"
	k.mux.HandlerFunc("GET", "/ucexit", k.HandleUCExit)
	k.webInfo.DisconnectPath = "/ucexit"
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
	k.rend.HTML(w, http.StatusOK, "kuPage", k.webInfo)
}

// HandleExit allows the client to exit at the config page.
// Witout this, the only way to exit on the config page was to kill the process(es)
func (k *Kobo) HandleExit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	k.exitChan <- true
}

// HandleConfig sends and gets config from user
func (k *Kobo) HandleConfig(w http.ResponseWriter, r *http.Request) {
	res := webConfig{}
	if r.Method == http.MethodGet {
		res.Opts = *k.KuConfig
		k.rend.JSON(w, http.StatusOK, res)
	} else {
		defer close(k.startChan)
		if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
			http.Error(w, "error getting config from client", http.StatusInternalServerError)
		}
		k.startChan <- res
		w.WriteHeader(http.StatusNoContent)
	}
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
			if !msg.GetPassword && !msg.GetCalInstance && msg.Finished == "" {
				// Note, we replace all newlines in the message with spaces. That is because server
				// sent events are newline delimited
				if msg.ShowMessage != "" {
					fmt.Fprintf(w, "event: showMessage\ndata: %s\n\n", strings.ReplaceAll(msg.ShowMessage, "\n", " "))
					f.Flush()
				}
				fmt.Fprintf(w, "event: progress\ndata: %d\n\n", msg.Progress)
				f.Flush()
			} else if msg.GetPassword {
				fmt.Fprintf(w, "event: auth\ndata: %s\n\n", "")
				f.Flush()
			} else if msg.GetCalInstance {
				fmt.Fprintf(w, "event: calibreInstances\ndata: %s\n\n", "")
				f.Flush()
			} else if msg.Finished != "" {
				fmt.Fprintf(w, "event: kuFinished\ndata: %s\n\n", strings.ReplaceAll(msg.Finished, "\n", " "))
				f.Flush()
			}
			k.doneChan <- true
		case <-r.Context().Done():
			return
		}
	}
}

// HandleCalAuth gets user supplied password
func (k *Kobo) HandleCalAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		pwInfo := <-k.AuthChan
		k.rend.JSON(w, http.StatusOK, pwInfo)
	} else {
		var pw calPassword
		if err := json.NewDecoder(r.Body).Decode(&pw); err != nil {
			http.Error(w, "error getting password from client", http.StatusInternalServerError)
		}
		k.AuthChan <- &pw
		w.WriteHeader(http.StatusNoContent)
	}
}

func (k *Kobo) HandleCalInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		k.rend.JSON(w, http.StatusOK, k.calInstances)
	} else {
		var instance uc.CalInstance
		if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
			http.Error(w, "error getting calibre instance from client", http.StatusInternalServerError)
		}
		k.calInstChan <- instance
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleUCExit lets the user stop UNCaGED client side, without having to disconnect via Calibre
func (k *Kobo) HandleUCExit(w http.ResponseWriter, r *http.Request) {
	if k.UCExitChan != nil {
		k.UCExitChan <- true
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// WebSend is a small function to print a message to webclient, and wait for to be sent before returning
func (k *Kobo) WebSend(msg WebMsg) {
	k.MsgChan <- msg
	<-k.doneChan
}
