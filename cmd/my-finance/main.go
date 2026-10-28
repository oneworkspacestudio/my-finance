package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/oneworkspacestudio/my-finance/internal/finance"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"os"
	"os/signal"
	"path/filepath"
)

var version = "0.1.0-preview.1"
var runtimeSHA = "development"

func main() {
	if e := run(); e != nil {
		apphost.Report(e)
		os.Exit(1)
	}
}
func run() error {
	data := flag.String("data", "", "Private NTFS data folder")
	listen := flag.String("listen", "127.0.0.1:5016", "Loopback listener")
	origin := flag.String("origin", "http://localhost:5016/finance", "Public app URL")
	noBrowser := flag.Bool("no-browser", false, "Run in background")
	email := flag.String("bootstrap-email", "", "Standalone first owner")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *data == "" {
		d, e := os.UserConfigDir()
		if e != nil {
			return e
		}
		*data = filepath.Join(d, "OneWorkspace", "MyFinance")
	}
	abs, e := filepath.Abs(*data)
	if e != nil {
		return e
	}
	c := apphost.FromEnv(apphost.Config{ID: "finance", Name: "My Finance", Version: version, Data: abs, Listen: *listen, Origin: *origin})
	h, e := apphost.New(c, finance.UI())
	if e != nil {
		return e
	}
	defer h.Close()
	if *email != "" {
		if e = h.Bootstrap(*email, os.Getenv("WORKSPACE_BOOTSTRAP_PASSWORD")); e != nil {
			return e
		}
		os.Unsetenv("WORKSPACE_BOOTSTRAP_PASSWORD")
	}
	a, e := finance.New(h, runtimeSHA)
	if e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	done := make(chan error, 1)
	go func() { e := a.Run(ctx); done <- e; cancel() }()
	link:=c.Origin+"/";if h.SetupURL()!=""{link=h.SetupURL()}
	fmt.Println("My Finance: " + link)
	if !*noBrowser {
		a.OpenBrowser(link)
	}
	e = h.Run(ctx)
	cancel()
	worker := <-done
	if e != nil {
		return e
	}
	return worker
}
