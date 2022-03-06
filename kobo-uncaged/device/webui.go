package device

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/shermp/UNCaGED/uc"
	"github.com/unrolled/render"
)

//go:embed web
var web_files embed.FS

// IgnoreProgress tells HandleMessage not to send progress value to web UI
const IgnoreProgress int = -127

func (k *Kobo) initWeb() {
	k.initRouter()
	k.initRender()
}

func (k *Kobo) initRouter() {
	k.mux = httprouter.New()
	k.mux.HandlerFunc("GET", "/", k.HandleIndex)
	k.webInfo.ConfigPath = "/config"
	k.mux.HandlerFunc("GET", k.webInfo.ConfigPath, k.HandleConfig)
	k.mux.HandlerFunc("POST", k.webInfo.ConfigPath, k.HandleConfig)
	k.webInfo.ExitPath = "/exit"
	k.mux.HandlerFunc("GET", k.webInfo.ExitPath, k.HandleExit)
	k.webInfo.SSEPath = "/messages"
	k.mux.HandlerFunc("GET", k.webInfo.SSEPath, k.HandleMessages)
	k.webInfo.AuthPath = "/calibreauth"
	k.mux.HandlerFunc("GET", k.webInfo.AuthPath, k.HandleCalAuth)
	k.mux.HandlerFunc("POST", k.webInfo.AuthPath, k.HandleCalAuth)
	k.webInfo.InstancePath = "/calibreinstance"
	k.mux.HandlerFunc("GET", k.webInfo.InstancePath, k.HandleCalInstances)
	k.mux.HandlerFunc("POST", k.webInfo.InstancePath, k.HandleCalInstances)
	k.webInfo.LibInfoPath = "/libinfo"
	k.mux.HandlerFunc("GET", k.webInfo.LibInfoPath, k.HandleLibraryInfo)
	k.mux.HandlerFunc("POST", k.webInfo.LibInfoPath, k.HandleLibraryInfo)
	k.webInfo.DisconnectPath = "/ucexit"
	k.mux.HandlerFunc("GET", k.webInfo.DisconnectPath, k.HandleUCExit)
	fsys, _ := fs.Sub(web_files, "web/static")
	k.mux.ServeFiles("/static/*filepath", http.FS(fsys))
}

func (k *Kobo) initRender() {
	k.rend = render.New(render.Options{
		Directory: "web/templates",
		FileSystem: &render.EmbedFileSystem{
			FS: web_files,
		},
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
			if !msg.GetPassword && !msg.GetCalInstance && !msg.GetLibInfo && msg.Finished == "" {
				// Note, we replace all newlines in the message with spaces. That is because server
				// sent events are newline delimited
				if msg.ShowMessage != "" {
					fmt.Fprintf(w, "event: showMessage\ndata: %s\n\n", strings.ReplaceAll(msg.ShowMessage, "\n", " "))
					f.Flush()
				}
				if msg.Progress != IgnoreProgress {
					fmt.Fprintf(w, "event: progress\ndata: %d\n\n", msg.Progress)
					f.Flush()
				}
			} else if msg.GetPassword {
				fmt.Fprintf(w, "event: auth\ndata: %s\n\n", "")
				f.Flush()
			} else if msg.GetCalInstance {
				fmt.Fprintf(w, "event: calibreInstances\ndata: %s\n\n", "")
				f.Flush()
			} else if msg.GetLibInfo {
				fmt.Fprintf(w, "event: libInfo\ndata: %s\n\n", "")
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

// HandleCalInstances gets the user selected calibre instance to connect to
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

// HandleLibraryInfo gets and sets library specific config/info
func (k *Kobo) HandleLibraryInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		stdFields := make([]string, 0)
		userFields := make([]string, 0)
		allFields := []string{""}
		selField := ""
		if libOpt, exists := k.KuConfig.LibOptions[k.LibInfo.LibraryUUID]; exists {
			selField = libOpt.SubtitleColumn
		}
		for name, field := range k.LibInfo.FieldMetadata {
			switch name {
			case "languages", "tags", "rating", "publisher":
				stdFields = append(stdFields, name)
			default:
				if field.IsCustom {
					userFields = append(userFields, name)
				}
			}
		}
		sort.Strings(stdFields)
		sort.Strings(userFields)
		allFields = append(allFields, stdFields...)
		allFields = append(allFields, userFields...)
		wlo := webLibOpts{CurrSel: 0, SubtitleFields: allFields}
		for i, field := range allFields {
			if field == selField {
				wlo.CurrSel = i
			}
		}
		k.rend.JSON(w, http.StatusOK, wlo)
	} else {
		var wlo webLibOpts
		if err := json.NewDecoder(r.Body).Decode(&wlo); err != nil {
			http.Error(w, "error getting subtitle field from client", http.StatusInternalServerError)
		}
		if k.KuConfig.LibOptions == nil {
			k.KuConfig.LibOptions = make(map[string]KuLibOptions)
		}
		k.KuConfig.LibOptions[k.LibInfo.LibraryUUID] = KuLibOptions{SubtitleColumn: wlo.SubtitleFields[wlo.CurrSel]}
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
