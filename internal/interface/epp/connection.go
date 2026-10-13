package epp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
)

// connContext implements epp-lib's ConnContext hook: it runs once per
// connection after the TLS handshake, applies the connection limit and
// stamps the connection's identity on the context every handler receives.
func (h *handlers) connContext(ctx context.Context, conn *tls.Conn) (context.Context, error) {
	clientIP := conn.RemoteAddr().String()
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		clientIP = tcpAddr.IP.String()
	}

	if h.limiter != nil {
		if err := h.limiter.CheckConnectionLimit(ctx, clientIP, ""); err != nil {
			h.logger.Warn("EPP connection rejected", "client_ip", clientIP, "error", err)
			return nil, err
		}
	}

	connectionID := fmt.Sprintf("conn-%d", time.Now().UnixNano())
	ctx = appcontext.WithConnectionID(ctx, connectionID)
	ctx = appcontext.WithClientIP(ctx, clientIP)

	if h.limiter != nil {
		if err := h.limiter.IncrementConnection(ctx, clientIP, ""); err != nil {
			h.logger.Error("EPP connection not tracked", "connection_id", connectionID, "client_ip", clientIP, "error", err)
		}
	}

	h.logger.Info("EPP connection established", "connection_id", connectionID, "client_ip", clientIP)

	// #nosec G118 -- deliberate: this cleanup runs after ctx.Done(), so the request context is already cancelled and cannot carry it
	go func() {
		<-ctx.Done()
		registrarID, _ := appcontext.RegistrarID(ctx)

		if h.limiter != nil {
			cleanupCtx := context.Background() // #nosec G118 -- deliberate: this runs after ctx.Done(), so the request context is already cancelled and cannot carry the cleanup
			if err := h.limiter.DecrementConnection(cleanupCtx, clientIP, registrarID); err != nil {
				h.logger.Error("EPP connection not released", "connection_id", connectionID, "client_ip", clientIP, "error", err)
			}
		}
		h.logger.Info("EPP connection closed", "connection_id", connectionID, "client_ip", clientIP)
	}()

	return ctx, nil
}
