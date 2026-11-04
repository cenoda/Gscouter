package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// handleEngineError logs what the engine failed on in a browser: the page
// runs that simulation on gcsim (layout.html, reportEngine) and tells the
// server the teams and the error, which names what the engine lacks, so
// that the log shows what to port, as the server's own runs do
// (score.RunIterations). No build is sent or logged: nothing of an
// imported file is (about.d_good). The answer is always 204; at most
// engineReportsPerMinute are logged a minute.
func (s *Server) handleEngineError(w http.ResponseWriter, r *http.Request) {
	defer w.WriteHeader(http.StatusNoContent)
	if !s.engineReports.allow(time.Now()) {
		return
	}
	var rep struct {
		Teams  []string `json:"teams"`
		Error  string   `json:"error"`
		Engine string   `json:"engine"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&rep); err != nil {
		return
	}
	clip := func(s string, n int) string {
		if len(s) > n {
			return s[:n]
		}
		return s
	}
	for i := range rep.Teams {
		rep.Teams[i] = clip(rep.Teams[i], 120)
	}
	if len(rep.Teams) > 2 {
		rep.Teams = rep.Teams[:2]
	}
	slog.Warn("engine failed a config in a browser, running gcsim", "engine", clip(rep.Engine, 16), "team", strings.Join(rep.Teams, ","), "err", clip(rep.Error, 2000))
}

const engineReportsPerMinute = 60

// reportLimit lets through at most engineReportsPerMinute reports in
// each minute.
type reportLimit struct {
	mu     sync.Mutex
	minute int64
	n      int
}

func (l *reportLimit) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m := now.Unix() / 60; m != l.minute {
		l.minute, l.n = m, 0
	}
	l.n++
	return l.n <= engineReportsPerMinute
}
