package sleepy

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"time"
	_ "time/tzdata"
)

func Run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("sleepy", flag.ContinueOnError)
	dir := flags.String("data-dir", "data", "persistent data/configuration directory")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	c, err := loadConfig(*dir)
	if err != nil {
		return err
	}
	store, err := openStore(c)
	if err != nil {
		return err
	}
	defer store.db.Close()
	app, err := newServer(c, store)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := &http.Server{
		Addr: c.address(), Handler: app, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 << 10, BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
			_ = srv.Close()
		}
	}()
	if c.enabled("auto_dead") {
		go app.autoDead(ctx)
	}
	log.Printf("Sleepy Go listening on %s (data: %s)", srv.Addr, c.DataDir)
	if c.Main.HTTPS {
		err = srv.ListenAndServeTLS(c.Main.SSLCert, c.Main.SSLKey)
	} else {
		err = srv.ListenAndServe()
	}
	cancel()
	<-stopped
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
