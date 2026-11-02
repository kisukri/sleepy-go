package sleepy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) events(w http.ResponseWriter, r *http.Request, legacy bool) error {
	// HEAD: go eats body writes -> loop never sees client leave, reused conn hangs. bail
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		return nil
	}
	var id int64
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 || v == 1<<63-1 {
			return apiError{400, "invalid Last-Event-ID"}
		}
		id = v
	}
	select {
	case s.streams <- struct{}{}:
		defer func() { <-s.streams }()
	default:
		return apiError{503, "too many event streams"}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	controller := http.NewResponseController(w)
	send := func(kind string, data []byte) error {
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		id++
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, kind, data); err != nil {
			return err
		}
		return controller.Flush()
	}
	snap, changed := s.store.snapshot()
	for {
		raw, err := json.Marshal(s.query(snap, legacy))
		if err != nil {
			return err
		}
		if err = send("update", raw); err != nil {
			return nil
		}
	wait:
		for {
			select {
			case <-r.Context().Done():
				return nil
			case <-changed:
				snap, changed = s.store.snapshot()
				break wait
			case <-ticker.C:
				if err = send("heartbeat", nil); err != nil {
					return nil
				}
			}
		}
	}
}
func (s *Server) autoDead(ctx context.Context) {
	ticker := time.NewTicker(s.autoInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.store.expireStatus(nowSeconds()-s.autoTimeout.Seconds(), s.autoStatus); err != nil {
				log.Printf("auto_dead: %v", err)
			}
		}
	}
}
