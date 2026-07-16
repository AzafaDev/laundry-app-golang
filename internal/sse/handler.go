package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"laundry-app-with-golang/internal/auth"
	"laundry-app-with-golang/internal/config"
	db "laundry-app-with-golang/internal/db/generated"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

const heartbeatInterval = 25 * time.Second

type Handler struct {
	Queries *db.Queries
	Cfg     config.Config
}

func NewHandler(queries *db.Queries, cfg config.Config) *Handler {
	return &Handler{Queries: queries, Cfg: cfg}
}

// identify authenticates the connecting client against either the customer
// or the employee cookie — the two Gin auth middlewares can't both be
// attached to one route (each aborts the request if its own cookie is
// missing), so this route does its own auth instead of reusing them as
// middleware.
func (h *Handler) identify(c *gin.Context) (channels []string, ok bool) {
	if token, err := c.Cookie("access_token"); err == nil {
		claims, err := auth.VerifyAccessToken(token, h.Cfg.JWTAccessSecret)
		if err != nil {
			return nil, false
		}
		return []string{"user:" + claims.CustomerID}, true
	}

	if token, err := c.Cookie("staff_access_token"); err == nil {
		claims, err := auth.VerifyEmployeeAccessToken(token, h.Cfg.JWTEmployeeAccessSecret)
		if err != nil {
			return nil, false
		}

		var employeeUUID pgtype.UUID
		if err := employeeUUID.Scan(claims.EmployeeID); err != nil {
			return nil, false
		}

		employee, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeUUID)
		if err != nil || !employee.IsActive {
			return nil, false
		}

		channels = []string{"user:" + claims.EmployeeID, "role:" + employee.Role}
		if employee.OutletID.Valid {
			channels = append(channels, "outlet:"+employee.OutletID.String())
		}
		return channels, true
	}

	return nil, false
}

func (h *Handler) Stream(c *gin.Context) {
	channels, ok := h.identify(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// At most 3 channels per connection (user + role + outlet for an
	// employee), so plain multi-case select covers every case without
	// spinning up fan-in goroutines.
	subs := make([]chan Event, len(channels))
	for i, ch := range channels {
		subs[i] = Default.Subscribe(ch)
	}
	defer func() {
		for i, ch := range channels {
			Default.Unsubscribe(ch, subs[i])
		}
	}()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	write := func(evt Event) {
		payload, err := json.Marshal(evt.Data)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", evt.Event, payload)
		flusher.Flush()
	}

	ctx := c.Request.Context()
	ch0 := subs[0]
	var ch1, ch2 chan Event
	if len(subs) > 1 {
		ch1 = subs[1]
	}
	if len(subs) > 2 {
		ch2 = subs[2]
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
		case evt, open := <-ch0:
			if !open {
				return
			}
			write(evt)
		case evt, open := <-ch1:
			if !open {
				return
			}
			write(evt)
		case evt, open := <-ch2:
			if !open {
				return
			}
			write(evt)
		}
	}
}
